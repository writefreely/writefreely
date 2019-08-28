/*
 * Copyright © 2019 A Bunch Tell LLC.
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
	"html/template"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/writeas/impart"
	"github.com/writeas/nerds/store"
	"github.com/writeas/web-core/log"
	"github.com/writeas/writefreely/page"
)

type Invite struct {
	ID       string
	MaxUses  sql.NullInt64
	Created  time.Time
	Expires  *time.Time
	Inactive bool

	uses int64
}

func (i Invite) Uses() int64 {
	return i.uses
}

func (i Invite) Expired() bool {
	return i.Expires != nil && i.Expires.Before(time.Now())
}

func (i Invite) ExpiresFriendly() string {
	return i.Expires.Format("January 2, 2006, 3:04 PM")
}

func handleViewUserInvites(app *App, u *User, w http.ResponseWriter, r *http.Request) error {
	// Don't show page if instance doesn't allow it
	if !(app.cfg.App.UserInvites != "" && (u.IsAdmin() || app.cfg.App.UserInvites != "admin")) {
		return impart.HTTPError{http.StatusNotFound, ""}
	}

	f, _ := getSessionFlashes(app, w, r, nil)

	p := struct {
		*UserPage
		Invites *[]Invite
	}{
		UserPage: NewUserPage(app, r, u, "Invite People", f),
	}

	var err error
	p.Invites, err = app.db.GetUserInvites(u.ID)
	if err != nil {
		return err
	}
	for i := range *p.Invites {
		(*p.Invites)[i].uses = app.db.GetUsersInvitedCount((*p.Invites)[i].ID)
	}

	showUserPage(w, "invite", p)
	return nil
}

func handleCreateUserInvite(app *App, u *User, w http.ResponseWriter, r *http.Request) error {
	muVal := r.FormValue("uses")
	expVal := r.FormValue("expires")

	if u.Suspended {
		return ErrUserSuspended
	}

	var err error
	var maxUses int
	if muVal != "0" {
		maxUses, err = strconv.Atoi(muVal)
		if err != nil {
			return impart.HTTPError{http.StatusBadRequest, "Invalid value for 'max_uses'"}
		}
	}

	var expDate *time.Time
	var expires int
	if expVal != "0" {
		expires, err = strconv.Atoi(expVal)
		if err != nil {
			return impart.HTTPError{http.StatusBadRequest, "Invalid value for 'expires'"}
		}
		ed := time.Now().Add(time.Duration(expires) * time.Minute)
		expDate = &ed
	}

	inviteID := store.GenerateRandomString("0123456789BCDFGHJKLMNPQRSTVWXYZbcdfghjklmnpqrstvwxyz", 6)
	err = app.db.CreateUserInvite(inviteID, u.ID, maxUses, expDate)
	if err != nil {
		return err
	}

	return impart.HTTPError{http.StatusFound, "/me/invites"}
}

func handleViewInvite(app *App, w http.ResponseWriter, r *http.Request) error {
	inviteCode := mux.Vars(r)["code"]

	i, err := app.db.GetUserInvite(inviteCode)
	if err != nil {
		return err
	}

	p := struct {
		page.StaticPage
		Error   string
		Flashes []template.HTML
		Invite  string
	}{
		StaticPage: pageForReq(app, r),
		Invite:     inviteCode,
	}

	if i.Expired() {
		p.Error = "This invite link has expired."
	}

	if i.MaxUses.Valid && i.MaxUses.Int64 > 0 {
		if c := app.db.GetUsersInvitedCount(inviteCode); c >= i.MaxUses.Int64 {
			p.Error = "This invite link has expired."
		}
	}

	// Get error messages
	session, err := app.sessionStore.Get(r, cookieName)
	if err != nil {
		// Ignore this
		log.Error("Unable to get session in handleViewInvite; ignoring: %v", err)
	}
	flashes, _ := getSessionFlashes(app, w, r, session)
	for _, flash := range flashes {
		p.Flashes = append(p.Flashes, template.HTML(flash))
	}

	// Show landing page
	return renderPage(w, "signup.tmpl", p)
}
