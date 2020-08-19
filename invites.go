/*
 * Copyright © 2019-2020 A Bunch Tell LLC.
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

func (i Invite) Active(db *datastore) bool {
	if i.Expired() {
		return false
	}
	if i.MaxUses.Valid && i.MaxUses.Int64 > 0 {
		if c := db.GetUsersInvitedCount(i.ID); c >= i.MaxUses.Int64 {
			return false
		}
	}
	return true
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
		Invites  *[]Invite
		Silenced bool
	}{
		UserPage: NewUserPage(app, r, u, "Invite People", f),
	}

	var err error

	p.Silenced, err = app.db.IsUserSilenced(u.ID)
	if err != nil {
		log.Error("view invites: %v", err)
	}

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

	if u.IsSilenced() {
		return ErrUserSilenced
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

	expired := i.Expired()
	if !expired && i.MaxUses.Valid && i.MaxUses.Int64 > 0 {
		// Invite has a max-use number, so check if we're past that limit
		i.uses = app.db.GetUsersInvitedCount(inviteCode)
		expired = i.uses >= i.MaxUses.Int64
	}

	if u := getUserSession(app, r); u != nil {
		// check if invite belongs to another user
		// error can be ignored as not important in this case
		if ownInvite, _ := app.db.IsUsersInvite(inviteCode, u.ID); !ownInvite {
			addSessionFlash(app, w, r, "You're already registered and logged in.", nil)
			// show homepage
			return impart.HTTPError{http.StatusFound, "/me/settings"}
		}

		// show invite instructions
		p := struct {
			*UserPage
			Invite  *Invite
			Expired bool
		}{
			UserPage: NewUserPage(app, r, u, "Invite to "+app.cfg.App.SiteName, nil),
			Invite:   i,
			Expired:  expired,
		}
		showUserPage(w, "invite-help", p)
		return nil
	}

	p := struct {
		page.StaticPage
		*OAuthButtons
		Error   string
		Flashes []template.HTML
		Invite  string
	}{
		StaticPage:   pageForReq(app, r),
		OAuthButtons: NewOAuthButtons(app.cfg),
		Invite:       inviteCode,
	}

	if expired {
		p.Error = "This invite link has expired."
	}

	// Tell search engines not to index invite links
	w.Header().Set("X-Robots-Tag", "noindex")

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
