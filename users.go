/*
 * Copyright © 2018-2019, 2021 A Bunch Tell LLC.
 *
 * This file is part of WriteFreely.
 *
 * WriteFreely is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License, included
 * in the LICENSE file in this source code package.
 */

package writefreely

import (
	"time"

	"github.com/guregu/null/zero"
	"github.com/writeas/web-core/data"
	"github.com/writeas/web-core/log"
	"github.com/writefreely/writefreely/key"
)

type UserStatus int

const (
	UserActive = iota
	UserSilenced
)

type (
	userCredentials struct {
		Alias string `json:"alias" schema:"alias"`
		Pass  string `json:"pass" schema:"pass"`
		Email string `json:"email" schema:"email"`
		Web   bool   `json:"web" schema:"-"`
		To    string `json:"-" schema:"to"`

		EmailLogin bool `json:"via_email" schema:"via_email"`
	}

	userRegistration struct {
		userCredentials
		InviteCode string `json:"invite_code" schema:"invite_code"`
		Honeypot   string `json:"fullname" schema:"fullname"`
		Normalize  bool   `json:"normalize" schema:"normalize"`
		Signup     bool   `json:"signup" schema:"signup"`
	}

	// AuthUser contains information for a newly authenticated user (either
	// from signing up or logging in).
	AuthUser struct {
		AccessToken string `json:"access_token,omitempty"`
		Password    string `json:"password,omitempty"`
		User        *User  `json:"user"`

		// Verbose user data
		Posts       *[]PublicPost `json:"posts,omitempty"`
		Collections *[]Collection `json:"collections,omitempty"`
	}

	// User is a consistent user object in the database and all contexts (auth
	// and non-auth) in the API.
	User struct {
		ID         int64       `json:"-"`
		Username   string      `json:"username"`
		HashedPass []byte      `json:"-"`
		HasPass    bool        `json:"has_pass"`
		Email      zero.String `json:"email"`
		Created    time.Time   `json:"created"`
		Status     UserStatus  `json:"status"`

		clearEmail string `json:"email"`
	}

	userMeStats struct {
		TotalCollections, TotalArticles, CollectionPosts uint64
	}

	ExportUser struct {
		*User
		Collections    *[]CollectionObj `json:"collections"`
		AnonymousPosts []PublicPost     `json:"posts"`
	}

	PublicUser struct {
		Username string `json:"username"`
	}
)

// EmailClear decrypts and returns the user's email, caching it in the user
// object.
func (u *User) EmailClear(keys *key.Keychain) string {
	if u.clearEmail != "" {
		return u.clearEmail
	}

	if u.Email.Valid && u.Email.String != "" {
		email, err := data.Decrypt(keys.EmailKey, []byte(u.Email.String))
		if err != nil {
			log.Error("Error decrypting user email: %v", err)
		} else {
			u.clearEmail = string(email)
			return u.clearEmail
		}
	}
	return ""
}

func (u User) CreatedFriendly() string {
	/*
		// TODO: accept a locale in this method and use that for the format
		var loc monday.Locale = monday.LocaleEnUS
		return monday.Format(u.Created, monday.DateTimeFormatsByLocale[loc], loc)
	*/
	return u.Created.Format("January 2, 2006, 3:04 PM")
}

// Cookie strips down an AuthUser to contain only information necessary for
// cookies.
func (u User) Cookie() *User {
	u.HashedPass = []byte{}

	return &u
}

func (u *User) IsAdmin() bool {
	// TODO: get this from database
	return u.ID == 1
}

func (u *User) IsSilenced() bool {
	return u.Status&UserSilenced != 0
}
