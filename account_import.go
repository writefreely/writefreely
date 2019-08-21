package writefreely

import (
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/go-multierror"
	"github.com/writeas/impart"
	wfimport "github.com/writeas/import"
	"github.com/writeas/web-core/log"
)

func viewImport(app *App, u *User, w http.ResponseWriter, r *http.Request) error {
	// Fetch extra user data
	p := NewUserPage(app, r, u, "Import", nil)

	c, err := app.db.GetCollections(u)
	if err != nil {
		return impart.HTTPError{http.StatusInternalServerError, fmt.Sprintf("unable to fetch collections: %v", err)}
	}

	d := struct {
		*UserPage
		Collections *[]Collection
		Flashes     []template.HTML
		Message     string
		InfoMsg     bool
	}{
		UserPage:    p,
		Collections: c,
		Flashes:     []template.HTML{},
	}

	flashes, _ := getSessionFlashes(app, w, r, nil)
	for _, flash := range flashes {
		if strings.HasPrefix(flash, "SUCCESS: ") {
			d.Message = strings.TrimPrefix(flash, "SUCCESS: ")
		} else if strings.HasPrefix(flash, "INFO: ") {
			d.Message = strings.TrimPrefix(flash, "INFO: ")
			d.InfoMsg = true
		} else {
			d.Flashes = append(d.Flashes, template.HTML(flash))
		}
	}

	showUserPage(w, "import", d)
	return nil
}

func handleImport(app *App, u *User, w http.ResponseWriter, r *http.Request) error {
	// limit 10MB per submission
	// TODO: increase?
	r.ParseMultipartForm(10 << 20)
	files := r.MultipartForm.File["files"]
	filesSubmitted := len(files)
	var filesImported, collsImported int
	var errs []error
	// TODO: support multiple zip uploads at once
	if filesSubmitted == 1 && files[0].Header.Get("Content-Type") == "application/zip" {
		filesSubmitted, filesImported, collsImported, errs = importZipPosts(app, w, r, files[0], u)
	} else {
		filesImported, errs = importFilePosts(app, w, r, files, u)
	}

	if len(errs) != 0 {
		_ = addSessionFlash(app, w, r, multierror.ListFormatFunc(errs), nil)
	}
	if filesImported == filesSubmitted {
		postAdj := "posts"
		if filesSubmitted == 1 {
			postAdj = "post"
		}
		if collsImported != 0 {
			collAdj := "collections"
			if collsImported == 1 {
				collAdj = "collection"
			}
			_ = addSessionFlash(app, w, r, fmt.Sprintf(
				"SUCCESS: Import complete, %d %s imported across %d %s.",
				filesImported,
				postAdj,
				collsImported,
				collAdj,
			), nil)
		} else {
			_ = addSessionFlash(app, w, r, fmt.Sprintf("SUCCESS: Import complete, %d %s imported.", filesImported, postAdj), nil)
		}
	} else if filesImported > 0 {
		_ = addSessionFlash(app, w, r, fmt.Sprintf("INFO: %d of %d posts imported, see details below.", filesImported, filesSubmitted), nil)
	}
	return impart.HTTPError{http.StatusFound, "/me/import"}
}

func importFilePosts(app *App, w http.ResponseWriter, r *http.Request, files []*multipart.FileHeader, u *User) (int, []error) {
	var fileErrs []error
	var count int
	for _, formFile := range files {
		if filepath.Ext(formFile.Filename) == ".zip" {
			fileErrs = append(fileErrs, fmt.Errorf("zips are supported as a single upload only: %s", formFile.Filename))
			log.Info("zip included in bulk files, skipping")
			continue
		}
		info, err := formFileToTemp(formFile)
		if err != nil {
			fileErrs = append(fileErrs, fmt.Errorf("failed to get file info of: %s", formFile.Filename))
			log.Error("import textfile: stat temp file: %v", err)
			continue
		}
		post, err := wfimport.FromFile(filepath.Join(os.TempDir(), info.Name()))
		if err == wfimport.ErrEmptyFile {
			// not a real error so don't log
			_ = addSessionFlash(app, w, r, fmt.Sprintf("%s was empty, import skipped", formFile.Filename), nil)
			continue
		} else if err == wfimport.ErrInvalidContentType {
			// same as above
			_ = addSessionFlash(app, w, r, fmt.Sprintf("%s is not a supported post file", formFile.Filename), nil)
			continue
		} else if err != nil {
			fileErrs = append(fileErrs, fmt.Errorf("failed to read copy of %s", formFile.Filename))
			log.Error("import textfile: file to post: %v", err)
			continue
		}

		post.Collection = r.PostFormValue("collection")
		coll, _ := app.db.GetCollection(post.Collection)
		if coll == nil {
			coll = &Collection{
				ID: 0,
			}
		}
		coll.hostName = app.cfg.App.Host
		created := post.Created.Format("2006-01-02T15:04:05Z")
		submittedPost := SubmittedPost{
			Title:   &post.Title,
			Content: &post.Content,
			Font:    "norm",
			Created: &created,
		}
		rp, err := app.db.CreatePost(u.ID, coll.ID, &submittedPost)
		if err != nil {
			fileErrs = append(fileErrs, fmt.Errorf("failed to create post from %s", formFile.Filename))
			log.Error("import textfile: create db post: %v", err)
			continue
		}

		// create public post

		if coll.ID != 0 && app.cfg.App.Federation {
			go federatePost(
				app,
				&PublicPost{
					Post: rp,
					Collection: &CollectionObj{
						Collection: *coll,
					},
				},
				coll.ID,
				false,
			)
		}
		count++
	}
	return count, fileErrs
}

func importZipPosts(app *App, w http.ResponseWriter, r *http.Request, file *multipart.FileHeader, u *User) (filesSubmitted, importedPosts, importedColls int, errs []error) {
	info, err := formFileToTemp(file)
	if err != nil {
		errs = append(errs, fmt.Errorf("upload temp file: %v", err))
		return
	}

	postMap, err := wfimport.FromZipDirs(filepath.Join(os.TempDir(), info.Name()))
	if err != nil {
		errs = append(errs, fmt.Errorf("parse posts and collections from zip: %v", err))
		return
	}

	for collKey, posts := range postMap {
		// TODO: will posts ever be 0? should skip if so
		collObj := CollectionObj{}
		importedColls++
		if collKey != wfimport.DraftsKey {
			coll, err := app.db.GetCollection(collKey)
			if err == ErrCollectionNotFound {
				coll, err = app.db.CreateCollection(app.cfg, collKey, collKey, u.ID)
				if err != nil {
					errs = append(errs, fmt.Errorf("create non existent collection: %v", err))
					continue
				}
				coll.hostName = app.cfg.App.Host
				collObj.Collection = *coll
			} else if err != nil {
				errs = append(errs, fmt.Errorf("get collection: %v", err))
				continue
			}
			collObj.Collection = *coll
		}

		for _, post := range posts {
			if post != nil {
				filesSubmitted++
				created := post.Created.Format("2006-01-02T15:04:05Z")
				submittedPost := SubmittedPost{
					Title:   &post.Title,
					Content: &post.Content,
					Font:    "norm",
					Created: &created,
				}
				rp, err := app.db.CreatePost(u.ID, collObj.Collection.ID, &submittedPost)
				if err != nil {
					errs = append(errs, fmt.Errorf("create post: %v", err))
				}

				if collObj.Collection.ID != 0 && app.cfg.App.Federation {
					go federatePost(
						app,
						&PublicPost{
							Post:       rp,
							Collection: &collObj,
						},
						collObj.Collection.ID,
						false,
					)
				}
				importedPosts++
			}
		}
	}
	return
}

func formFileToTemp(formFile *multipart.FileHeader) (os.FileInfo, error) {
	file, err := formFile.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open form file: %s", formFile.Filename)
	}
	defer file.Close()

	tempFile, err := ioutil.TempFile("", fmt.Sprintf("upload-*%s", filepath.Ext(formFile.Filename)))
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary file for: %s", formFile.Filename)
	}
	defer tempFile.Close()

	_, err = io.Copy(tempFile, file)
	if err != nil {
		return nil, fmt.Errorf("failed to copy file into temporary location: %s", formFile.Filename)
	}

	return tempFile.Stat()
}
