/*
 * Copyright © 2018 A Bunch Tell LLC.
 *
 * This file is part of WriteFreely.
 *
 * WriteFreely is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License, included
 * in the LICENSE file in this source code package.
 */
package author

import (
	"github.com/writeas/writefreely/config"
	"os"
	"path/filepath"
	"regexp"
)

// Regex pattern for valid usernames
var validUsernameReg = regexp.MustCompile("^[a-zA-Z0-9][a-zA-Z0-9-]*$")

// List of reserved usernames
var reservedUsernames = map[string]bool{
	"a":                true,
	"about":            true,
	"add":              true,
	"admin":            true,
	"administrator":    true,
	"adminzone":        true,
	"api":              true,
	"article":          true,
	"articles":         true,
	"auth":             true,
	"authenticate":     true,
	"browse":           true,
	"c":                true,
	"categories":       true,
	"category":         true,
	"changes":          true,
	"community":        true,
	"create":           true,
	"css":              true,
	"data":             true,
	"dev":              true,
	"developers":       true,
	"draft":            true,
	"drafts":           true,
	"edit":             true,
	"edits":            true,
	"faq":              true,
	"feed":             true,
	"feedback":         true,
	"guide":            true,
	"guides":           true,
	"help":             true,
	"index":            true,
	"js":               true,
	"login":            true,
	"logout":           true,
	"me":               true,
	"media":            true,
	"meta":             true,
	"metadata":         true,
	"new":              true,
	"news":             true,
	"post":             true,
	"posts":            true,
	"privacy":          true,
	"publication":      true,
	"publications":     true,
	"publish":          true,
	"random":           true,
	"read":             true,
	"reader":           true,
	"register":         true,
	"remove":           true,
	"signin":           true,
	"signout":          true,
	"signup":           true,
	"start":            true,
	"status":           true,
	"summary":          true,
	"support":          true,
	"tag":              true,
	"tags":             true,
	"team":             true,
	"template":         true,
	"templates":        true,
	"terms":            true,
	"terms-of-service": true,
	"termsofservice":   true,
	"theme":            true,
	"themes":           true,
	"tips":             true,
	"tos":              true,
	"update":           true,
	"updates":          true,
	"user":             true,
	"users":            true,
	"yourname":         true,
}

// IsValidUsername returns true if a given username is neither reserved nor
// of the correct format.
func IsValidUsername(cfg *config.Config, username string) bool {
	// Username has to be above a character limit
	if len(username) < cfg.App.MinUsernameLen {
		return false
	}
	// Username is invalid if page with the same name exists. So traverse
	// available pages, adding them to reservedUsernames map that'll be checked
	// later.
	// TODO: use pagesDir const
	filepath.Walk("pages/", func(path string, i os.FileInfo, err error) error {
		reservedUsernames[i.Name()] = true
		return nil
	})

	// Username is invalid if it is reserved!
	if _, reserved := reservedUsernames[username]; reserved {
		return false
	}

	// TODO: use correct regexp function here
	return len(validUsernameReg.FindStringSubmatch(username)) > 0
}
