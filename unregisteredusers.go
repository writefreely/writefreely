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
	"database/sql"
	"encoding/json"
	"github.com/writeas/impart"
	"github.com/writeas/web-core/log"
	"net/http"
)

func handleWebSignup(app *App, w http.ResponseWriter, r *http.Request) error {
	reqJSON := IsJSON(r.Header.Get("Content-Type"))

	// Get params
	var ur userRegistration
	if reqJSON {
		decoder := json.NewDecoder(r.Body)
		err := decoder.Decode(&ur)
		if err != nil {
			log.Error("Couldn't parse signup JSON request: %v\n", err)
			return ErrBadJSON
		}
	} else {
		err := r.ParseForm()
		if err != nil {
			log.Error("Couldn't parse signup form request: %v\n", err)
			return ErrBadFormData
		}

		err = app.formDecoder.Decode(&ur, r.PostForm)
		if err != nil {
			log.Error("Couldn't decode signup form request: %v\n", err)
			return ErrBadFormData
		}
	}
	ur.Web = true
	ur.Normalize = true

	to := "/"
	if app.cfg.App.SimpleNav {
		to = "/new"
	}
	if ur.InviteCode != "" {
		to = "/invite/" + ur.InviteCode
	}
	_, err := signupWithRegistration(app, ur, w, r)
	if err != nil {
		if err, ok := err.(impart.HTTPError); ok {
			session, _ := app.sessionStore.Get(r, cookieName)
			if session != nil {
				session.AddFlash(err.Message)
				session.Save(r, w)
				return impart.HTTPError{http.StatusFound, to}
			}
		}
		return err
	}
	return impart.HTTPError{http.StatusFound, to}
}

// { "username": "asdf" }
// result: { code: 204 }
func handleUsernameCheck(app *App, w http.ResponseWriter, r *http.Request) error {
	reqJSON := IsJSON(r.Header.Get("Content-Type"))

	// Get params
	var d struct {
		Username string `json:"username"`
	}
	if reqJSON {
		decoder := json.NewDecoder(r.Body)
		err := decoder.Decode(&d)
		if err != nil {
			log.Error("Couldn't decode username check: %v\n", err)
			return ErrBadFormData
		}
	} else {
		return impart.HTTPError{http.StatusNotAcceptable, "Must be JSON request"}
	}

	// Check if username is okay
	finalUsername := getSlug(d.Username, "")
	if finalUsername == "" {
		errMsg := "Invalid username"
		if d.Username != "" {
			// Username was provided, but didn't convert into valid latin characters
			errMsg += " - must have at least 2 letters or numbers"
		}
		return impart.HTTPError{http.StatusBadRequest, errMsg + "."}
	}
	if app.db.PostIDExists(finalUsername) {
		return impart.HTTPError{http.StatusConflict, "Username is already taken."}
	}
	var un string
	err := app.db.QueryRow("SELECT username FROM users WHERE username = ?", finalUsername).Scan(&un)
	switch {
	case err == sql.ErrNoRows:
		return impart.WriteSuccess(w, finalUsername, http.StatusOK)
	case err != nil:
		log.Error("Couldn't SELECT username: %v", err)
		return impart.HTTPError{http.StatusInternalServerError, "We messed up."}
	}

	// Username was found, so it's taken
	return impart.HTTPError{http.StatusConflict, "Username is already taken."}
}

func getValidUsername(app *App, reqName, prevName string) (string, *impart.HTTPError) {
	// Check if username is okay
	finalUsername := getSlug(reqName, "")
	if finalUsername == "" {
		errMsg := "Invalid username"
		if reqName != "" {
			// Username was provided, but didn't convert into valid latin characters
			errMsg += " - must have at least 2 letters or numbers"
		}
		return "", &impart.HTTPError{http.StatusBadRequest, errMsg + "."}
	}
	if finalUsername == prevName {
		return "", &impart.HTTPError{http.StatusNotModified, "Username unchanged."}
	}
	if app.db.PostIDExists(finalUsername) {
		return "", &impart.HTTPError{http.StatusConflict, "Username is already taken."}
	}
	var un string
	err := app.db.QueryRow("SELECT username FROM users WHERE username = ?", finalUsername).Scan(&un)
	switch {
	case err == sql.ErrNoRows:
		return finalUsername, nil
	case err != nil:
		log.Error("Couldn't SELECT username: %v", err)
		return "", &impart.HTTPError{http.StatusInternalServerError, "We messed up."}
	}

	// Username was found, so it's taken
	return "", &impart.HTTPError{http.StatusConflict, "Username is already taken."}
}
