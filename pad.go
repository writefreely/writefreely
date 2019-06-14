/*
 * Copyright © 2018-2019 A Bunch Tell LLC.
 *
 * This file is part of WriteFreely.
 *
 * WriteFreely is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License, included
 * in the LICENSE file in this source code package.
 */

package writefreely

import (
	"github.com/gorilla/mux"
	"github.com/writeas/impart"
	"github.com/writeas/web-core/log"
	"github.com/writeas/writefreely/page"
	"net/http"
	"strings"
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
		Post  *RawPost
		User  *User
		Blogs *[]Collection

		Editing        bool        // True if we're modifying an existing post
		EditCollection *Collection // Collection of the post we're editing, if any
	}{
		StaticPage: pageForReq(app, r),
		Post:       &RawPost{Font: "norm"},
		User:       getUserSession(app, r),
	}
	var err error
	if appData.User != nil {
		appData.Blogs, err = app.db.GetPublishableCollections(appData.User)
		if err != nil {
			log.Error("Unable to get user's blogs for Pad: %v", err)
		}
	}

	padTmpl := "pad"

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
			return err
		}
	} else {
		// Editing a floating article
		appData.Post = getRawPost(app, action)
		appData.Post.Id = action
	}

	if appData.Post.Gone {
		return ErrPostUnpublished
	} else if appData.Post.Found && appData.Post.Content != "" {
		// Got the post
	} else if appData.Post.Found {
		return ErrPostFetchError
	} else {
		return ErrPostNotFound
	}

	if err = templates[padTmpl].ExecuteTemplate(w, "pad", appData); err != nil {
		log.Error("Unable to execute template: %v", err)
	}
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
	}{
		StaticPage: pageForReq(app, r),
		Post:       &RawPost{Font: "norm"},
		User:       getUserSession(app, r),
	}
	var err error

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

	if err = templates["edit-meta"].ExecuteTemplate(w, "edit-meta", appData); err != nil {
		log.Error("Unable to execute template: %v", err)
	}
	return nil
}
