/*
 * Copyright © 2018-2021 Musing Studio LLC.
 *
 * This file is part of WriteFreely.
 *
 * WriteFreely is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License, included
 * in the LICENSE file in this source code package.
 */

package writefreely

import (
	"os"
	"io"
	"fmt"
	"strconv"
	"io/ioutil"
	"path/filepath"
	"encoding/json"
	"net/http"
	"strings"

	uuid "github.com/nu7hatch/gouuid"

	"github.com/gorilla/mux"
	"github.com/writeas/impart"
	"github.com/writeas/web-core/log"
	"github.com/writefreely/writefreely/page"
)

func handleViewPad(app *App, w http.ResponseWriter, r *http.Request) error {
	vars := mux.Vars(r)
	action := vars["action"]
	slug := vars["slug"]
	collAlias := vars["collection"]
	if app.cfg.App.SingleUser {
		// TODO: refactor all of this, especially for single-user blogs
		c, err := app.db.GetCollectionByID(1)
		if err != nil {
			return err
		}
		collAlias = c.Alias
	}
	appData := &struct {
		page.StaticPage
		Post     *RawPost
		User     *User
		Blogs    *[]Collection
		Silenced bool

		Editing        bool        // True if we're modifying an existing post
		EditCollection *Collection // Collection of the post we're editing, if any
	}{
		StaticPage: pageForReq(app, r),
		Post:       &RawPost{Font: "norm"},
		User:       getUserSession(app, r),
	}
	var err error
	if appData.User != nil {
		appData.Blogs, err = app.db.GetPublishableCollections(appData.User, app.cfg.App.Host)
		if err != nil {
			log.Error("Unable to get user's blogs for Pad: %v", err)
		}
		appData.Silenced, err = app.db.IsUserSilenced(appData.User.ID)
		if err != nil {
			if err == ErrUserNotFound {
				return err
			}
			log.Error("Unable to get user status for Pad: %v", err)
		}
	}

	padTmpl := app.cfg.App.Editor
	if templates[padTmpl] == nil {
		if padTmpl != "" {
			log.Info("No template '%s' found. Falling back to default 'pad' template.", padTmpl)
		}
		padTmpl = "pad"
	}

	if action == "" && slug == "" {
		// Not editing any post; simply render the Pad
		if err = templates[padTmpl].ExecuteTemplate(w, "pad", appData); err != nil {
			log.Error("Unable to execute template: %v", err)
		}

		return nil
	}

	// Retrieve post information for editing
	appData.Editing = true
	// Make sure this isn't cached, so user doesn't accidentally lose data
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Expires", "Thu, 04 Oct 1990 20:00:00 GMT")
	if slug != "" {
		// TODO: refactor all of this, especially for single-user blogs
		appData.Post = getRawCollectionPost(app, slug, collAlias)
		if appData.Post.OwnerID != appData.User.ID {
			// TODO: add ErrForbiddenEditPost message to flashes
			return impart.HTTPError{http.StatusFound, r.URL.Path[:strings.LastIndex(r.URL.Path, "/edit")]}
		}
		appData.EditCollection, err = app.db.GetCollectionForPad(collAlias)
		if err != nil {
			log.Error("Unable to GetCollectionForPad: %s", err)
			return err
		}
		appData.EditCollection.hostName = app.cfg.App.Host
	} else {
		// Editing a floating article
		appData.Post = getRawPost(app, action)
		appData.Post.Id = action
	}

	if appData.Post.Gone {
		return ErrPostUnpublished
	} else if appData.Post.Found && (appData.Post.Title != "" || appData.Post.Content != "") {
		// Got the post
	} else if appData.Post.Found {
		log.Error("Found post, but other conditions failed.")
		return ErrPostFetchError
	} else {
		return ErrPostNotFound
	}

	if err = templates[padTmpl].ExecuteTemplate(w, "pad", appData); err != nil {
		log.Error("Unable to execute template: %v", err)
	}
	return nil
}

func okToEdit(app *App, w http.ResponseWriter, r *http.Request) error {
	vars := mux.Vars(r)
	action := vars["action"]
	slug := vars["slug"]
	collAlias := vars["collection"]
	appData := &struct {
		page.StaticPage
		Post           *RawPost
		User           *User
		EditCollection *Collection // Collection of the post we're editing, if any
		Flashes        []string
		NeedsToken     bool
		Silenced       bool
	}{
		StaticPage: pageForReq(app, r),
		Post:       &RawPost{Font: "norm"},
		User:       getUserSession(app, r),
	}
	var err error
	appData.Silenced, err = app.db.IsUserSilenced(appData.User.ID)
	if err != nil {
		log.Error("view meta: get user status: %v", err)
		return ErrInternalGeneral
	}

	if action == "" && slug == "" {
		return ErrPostNotFound
	}

	// Make sure this isn't cached, so user doesn't accidentally lose data
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Expires", "Thu, 28 Jul 1989 12:00:00 GMT")
	if slug != "" {
		appData.Post = getRawCollectionPost(app, slug, collAlias)
		if appData.Post.OwnerID != appData.User.ID {
			// TODO: add ErrForbiddenEditPost message to flashes
			return impart.HTTPError{
				http.StatusFound, r.URL.Path[:strings.LastIndex(r.URL.Path, "/meta")]}
		}
		if app.cfg.App.SingleUser {
			// TODO: optimize this query just like we do in GetCollectionForPad (?)
			appData.EditCollection, err = app.db.GetCollectionByID(1)
		} else {
			appData.EditCollection, err = app.db.GetCollectionForPad(collAlias)
		}
		if err != nil {
			return err
		}
		appData.EditCollection.hostName = app.cfg.App.Host
	} else {
		// Editing a floating article
		appData.Post = getRawPost(app, action)
		appData.Post.Id = action
	}
	appData.NeedsToken = appData.User == nil || appData.User.ID != appData.Post.OwnerID

	if appData.Post.Gone {
		return ErrPostUnpublished
	} else if appData.Post.Found && appData.Post.Content != "" {
		// Got the post
	} else if appData.Post.Found {
		return ErrPostFetchError
	} else {
		return ErrPostNotFound
	}
	return nil
}

func handleDeleteFile(app *App, w http.ResponseWriter, r *http.Request) error {
	vars := mux.Vars(r)
	fileName := vars["filename"]
	slug := vars["slug"]
	user := getUserSession(app, r)
	filePath := filepath.Join(app.cfg.Server.MediaParentDir, mediaDir, user.Username, slug) + "/" + fileName
	err := os.Remove(filePath)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return ErrFileNotFound
	}
	w.WriteHeader(http.StatusOK)
	return nil
}

func handleViewMeta(app *App, w http.ResponseWriter, r *http.Request) error {
	vars := mux.Vars(r)
	action := vars["action"]
	slug := vars["slug"]
	collAlias := vars["collection"]
	appData := &struct {
		page.StaticPage
		Post           *RawPost
		User           *User
		EditCollection *Collection // Collection of the post we're editing, if any
		Flashes        []string
		NeedsToken     bool
		Silenced       bool
	}{
		StaticPage: pageForReq(app, r),
		Post:       &RawPost{Font: "norm"},
		User:       getUserSession(app, r),
	}
	var err error
	appData.Silenced, err = app.db.IsUserSilenced(appData.User.ID)
	if err != nil {
		log.Error("view meta: get user status: %v", err)
		return ErrInternalGeneral
	}

	if action == "" && slug == "" {
		return ErrPostNotFound
	}

	// Make sure this isn't cached, so user doesn't accidentally lose data
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Expires", "Thu, 28 Jul 1989 12:00:00 GMT")
	if slug != "" {
		appData.Post = getRawCollectionPost(app, slug, collAlias)
		if appData.Post.OwnerID != appData.User.ID {
			// TODO: add ErrForbiddenEditPost message to flashes
			return impart.HTTPError{http.StatusFound, r.URL.Path[:strings.LastIndex(r.URL.Path, "/meta")]}
		}
		if app.cfg.App.SingleUser {
			// TODO: optimize this query just like we do in GetCollectionForPad (?)
			appData.EditCollection, err = app.db.GetCollectionByID(1)
		} else {
			appData.EditCollection, err = app.db.GetCollectionForPad(collAlias)
		}
		if err != nil {
			return err
		}
		appData.EditCollection.hostName = app.cfg.App.Host
	} else {
		// Editing a floating article
		appData.Post = getRawPost(app, action)
		appData.Post.Id = action
	}
	appData.NeedsToken = appData.User == nil || appData.User.ID != appData.Post.OwnerID

	if appData.Post.Gone {
		return ErrPostUnpublished
	} else if appData.Post.Found && appData.Post.Content != "" {
		// Got the post
	} else if appData.Post.Found {
		return ErrPostFetchError
	} else {
		return ErrPostNotFound
	}
	appData.Flashes, _ = getSessionFlashes(app, w, r, nil)
	user := getUserSession(app, r)

	if slug == "" {
		slug, _ = getSlugFromActionId(app, action)
	}

	mediaDirectoryPath := filepath.Join(app.cfg.Server.MediaParentDir, mediaDir,
						user.Username, slug)
	appData.Post.MediaFilesList, _ = getFilesListInPath(mediaDirectoryPath)
	if err = templates["edit-meta"].ExecuteTemplate(w, "edit-meta", appData); err != nil {
		log.Error("Unable to execute template: %v", err)
	}
	return nil
}

func getNewFileName(path string, originalFieName string) (string, error) {
	u, err := uuid.NewV4()
	if err != nil {
		log.Error("Unable to generate uuid: %v", err)
		return "", err
	}
	extension := filepath.Ext(originalFieName)
	return u.String() + extension, nil
}

func getFilesListInPath(path string) ([]string, error) {
	var files []string
	entries, err := ioutil.ReadDir(path)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			files = append(files, "/" + path + "/" + entry.Name())
		}
	}
	return files, nil
}

func handleGetFile(app *App, w http.ResponseWriter, r *http.Request) error {
	vars := mux.Vars(r)
	slug := vars["slug"]
	author := vars["author"]
	filename := vars["filename"]
	mediaDirectoryPath := filepath.Join(app.cfg.Server.MediaParentDir, mediaDir,
							                                   author, slug, filename)
	filePath := mediaDirectoryPath 
	file, err := http.Dir("").Open(filePath)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return nil
	}
	defer file.Close()
	fileInfo, err := file.Stat()
	if err != nil {
		http.Error(w, "Failed to get file information", http.StatusInternalServerError)
		return nil
	}
	w.Header().Set("Content-Disposition", "attachment; filename="+fileInfo.Name())
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.FormatInt(fileInfo.Size(), 10))
	http.ServeContent(w, r, fileInfo.Name(), fileInfo.ModTime(), file)
	return nil
}

func calculateDirectoryTotalSize(dirPath string) (int64, error) {
	var totalSize int64

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			totalSize += info.Size()
		}

		return nil
	})

	return totalSize, err
}

func handleUploadMedia(app *App, w http.ResponseWriter, r *http.Request) error {
	maxUploadSize := app.cfg.App.MediaMaxSize * 1024 * 1024
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		errMsg := fmt.Sprintf("File size limit exceeded. The limit is: %d MB",
					app.cfg.App.MediaMaxSize)
		http.Error(w, errMsg, http.StatusBadRequest)
		return nil
	}

	fileSize := r.ContentLength

	if err := okToEdit(app, w, r); err != nil {
		 return err
	}
	vars := mux.Vars(r)
	slug := vars["slug"]

	if slug == "" {
		actionId := vars["action"]
		if actionId == "" {
			return ErrPostNotFound
		}
		var err error
		slug, err = getSlugFromActionId(app, actionId)
		if slug == "" || err != nil {
			return ErrPostNotFound
		}
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return nil
	}
	file, handler, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Error retrieving the file", http.StatusInternalServerError)
		return nil
	}
	defer file.Close()
	user := getUserSession(app, r)
	mediaDirectoryPath := filepath.Join(app.cfg.Server.MediaParentDir, mediaDir,
					user.Username, slug)
	err = os.MkdirAll(mediaDirectoryPath, 0755)
	if err != nil {
		return err
	}

	totalSize, err := calculateDirectoryTotalSize(mediaDirectoryPath)
	totalMediaSpace := app.cfg.App.TotalMediaSpace * 1024 * 1024
	if totalSize + fileSize > totalMediaSpace {
		errMsg := fmt.Sprintf("Your upload space limit has been exceeded. Your limit is: %d MB",
					app.cfg.App.TotalMediaSpace)
		http.Error(w, errMsg, http.StatusBadRequest)
		return nil
	}

	newFileName, _ := getNewFileName(mediaDirectoryPath, handler.Filename)
	newFilePath := filepath.Join(mediaDirectoryPath, newFileName)
	dst, err := os.Create(newFilePath)
	if err != nil {
		http.Error(w, "Error saving the file", http.StatusInternalServerError)
		return nil
	}
	defer dst.Close()
	_, err = io.Copy(dst, file)
	if err != nil {
		http.Error(w, "Error copying the file", http.StatusInternalServerError)
		return nil
	}
	response := map[string]string{
		"message": "File uploaded successfully!",
		"path": newFilePath,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
	return nil
}
