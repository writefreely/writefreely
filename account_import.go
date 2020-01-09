package writefreely

import (
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
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
	p := NewUserPage(app, r, u, "Import Posts", nil)

	c, err := app.db.GetCollections(u, app.Config().App.Host)
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
	r.ParseMultipartForm(10 << 20)

	collAlias := r.PostFormValue("collection")
	coll := &Collection{
		ID: 0,
	}
	var err error
	if collAlias != "" {
		coll, err = app.db.GetCollection(collAlias)
		if err != nil {
			log.Error("Unable to get collection for import: %s", err)
			return err
		}
		// Only allow uploading to collection if current user is owner
		if coll.OwnerID != u.ID {
			err := ErrUnauthorizedGeneral
			_ = addSessionFlash(app, w, r, err.Message, nil)
			return err
		}
		coll.hostName = app.cfg.App.Host
	}

	files := r.MultipartForm.File["files"]
	var fileErrs []error
	filesSubmitted := len(files)
	var filesImported int
	for _, formFile := range files {
		fname := ""
		ok := func() bool {
			file, err := formFile.Open()
			if err != nil {
				fileErrs = append(fileErrs, fmt.Errorf("failed to open form file: %s", formFile.Filename))
				log.Error("import textfile: open from form: %v", err)
				return false
			}
			defer file.Close()

			tempFile, err := ioutil.TempFile("", "post-upload-*.txt")
			if err != nil {
				fileErrs = append(fileErrs, fmt.Errorf("failed to create temporary file for: %s", formFile.Filename))
				log.Error("import textfile: create temp file: %v", err)
				return false
			}
			defer tempFile.Close()

			_, err = io.Copy(tempFile, file)
			if err != nil {
				fileErrs = append(fileErrs, fmt.Errorf("failed to copy file into temporary location: %s", formFile.Filename))
				log.Error("import textfile: copy to temp: %v", err)
				return false
			}

			info, err := tempFile.Stat()
			if err != nil {
				fileErrs = append(fileErrs, fmt.Errorf("failed to get file info of: %s", formFile.Filename))
				log.Error("import textfile: stat temp file: %v", err)
				return false
			}
			fname = info.Name()
			return true
		}()
		if !ok {
			continue
		}

		post, err := wfimport.FromFile(filepath.Join(os.TempDir(), fname))
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

		if collAlias != "" {
			post.Collection = collAlias
		}
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
		filesImported++
	}
	if len(fileErrs) != 0 {
		_ = addSessionFlash(app, w, r, multierror.ListFormatFunc(fileErrs), nil)
	}

	if filesImported == filesSubmitted {
		verb := "posts"
		if filesSubmitted == 1 {
			verb = "post"
		}
		_ = addSessionFlash(app, w, r, fmt.Sprintf("SUCCESS: Import complete, %d %s imported.", filesImported, verb), nil)
	} else if filesImported > 0 {
		_ = addSessionFlash(app, w, r, fmt.Sprintf("INFO: %d of %d posts imported, see details below.", filesImported, filesSubmitted), nil)
	}
	return impart.HTTPError{http.StatusFound, "/me/import"}
}
