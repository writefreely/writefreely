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
	"encoding/gob"
	"github.com/gorilla/sessions"
	"github.com/writeas/web-core/log"
	"net/http"
	"strings"
)

const (
	day           = 86400
	sessionLength = 180 * day
	cookieName    = "wfu"
	cookieUserVal = "u"

	blogPassCookieName = "ub"
)

// InitSession creates the cookie store. It depends on the keychain already
// being loaded.
func (app *App) InitSession() {
	// Register complex data types we'll be storing in cookies
	gob.Register(&User{})

	// Create the cookie store
	store := sessions.NewCookieStore(app.keys.CookieAuthKey, app.keys.CookieKey)
	store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   sessionLength,
		HttpOnly: true,
		Secure:   strings.HasPrefix(app.cfg.App.Host, "https://"),
		SameSite: http.SameSiteNoneMode,
	}
	app.sessionStore = store
}

func getSessionFlashes(app *App, w http.ResponseWriter, r *http.Request, session *sessions.Session) ([]string, error) {
	var err error
	if session == nil {
		session, err = app.sessionStore.Get(r, cookieName)
		if err != nil {
			return nil, err
		}
	}

	f := []string{}
	if flashes := session.Flashes(); len(flashes) > 0 {
		for _, flash := range flashes {
			if str, ok := flash.(string); ok {
				f = append(f, str)
			}
		}
	}
	saveUserSession(app, r, w)

	return f, nil
}

func addSessionFlash(app *App, w http.ResponseWriter, r *http.Request, m string, session *sessions.Session) error {
	var err error
	if session == nil {
		session, err = app.sessionStore.Get(r, cookieName)
	}

	if err != nil {
		log.Error("Unable to add flash '%s': %v", m, err)
		return err
	}

	session.AddFlash(m)
	saveUserSession(app, r, w)
	return nil
}

func getUserAndSession(app *App, r *http.Request) (*User, *sessions.Session) {
	session, err := app.sessionStore.Get(r, cookieName)
	if err == nil {
		// Got the currently logged-in user
		val := session.Values[cookieUserVal]
		var u = &User{}
		var ok bool
		if u, ok = val.(*User); ok {
			return u, session
		}
	}

	return nil, nil
}

func getUserSession(app *App, r *http.Request) *User {
	u, _ := getUserAndSession(app, r)
	return u
}

func saveUserSession(app *App, r *http.Request, w http.ResponseWriter) error {
	session, err := app.sessionStore.Get(r, cookieName)
	if err != nil {
		return ErrInternalCookieSession
	}

	// Extend the session
	session.Options.MaxAge = int(sessionLength)

	// Remove any information that accidentally got added
	// FIXME: find where Plan information is getting saved to cookie.
	val := session.Values[cookieUserVal]
	var u = &User{}
	var ok bool
	if u, ok = val.(*User); ok {
		session.Values[cookieUserVal] = u.Cookie()
	}

	err = session.Save(r, w)
	if err != nil {
		log.Error("Couldn't saveUserSession: %v", err)
	}
	return err
}

func getFullUserSession(app *App, r *http.Request) *User {
	u := getUserSession(app, r)
	if u == nil {
		return nil
	}

	u, _ = app.db.GetUserByID(u.ID)
	return u
}
