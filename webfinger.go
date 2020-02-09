/*
 * Copyright © 2018-2020 A Bunch Tell LLC.
 *
 * This file is part of WriteFreely.
 *
 * WriteFreely is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License, included
 * in the LICENSE file in this source code package.
 */

package writefreely

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/writeas/go-webfinger"
	"github.com/writeas/impart"
	"github.com/writeas/web-core/log"
	"github.com/writeas/writefreely/config"
)

type wfResolver struct {
	db  *datastore
	cfg *config.Config
}

var wfUserNotFoundErr = impart.HTTPError{http.StatusNotFound, "User not found."}

func (wfr wfResolver) FindUser(username string, host, requestHost string, r []webfinger.Rel) (*webfinger.Resource, error) {
	var c *Collection
	var err error
	if wfr.cfg.App.SingleUser {
		c, err = wfr.db.GetCollectionByID(1)
	} else {
		c, err = wfr.db.GetCollection(username)
	}
	if err != nil {
		log.Error("Unable to get blog: %v", err)
		return nil, err
	}
	suspended, err := wfr.db.IsUserSuspended(c.OwnerID)
	if err != nil {
		log.Error("webfinger find user: check is suspended: %v", err)
		return nil, err
	}
	if suspended {
		return nil, wfUserNotFoundErr
	}
	c.hostName = wfr.cfg.App.Host
	if wfr.cfg.App.SingleUser {
		// Ensure handle matches user-chosen one on single-user blogs
		if username != c.Alias {
			log.Info("Username '%s' is not handle '%s'", username, c.Alias)
			return nil, wfUserNotFoundErr
		}
	}
	// Only return information if site has federation enabled.
	// TODO: enable two levels of federation? Unlisted or Public on timelines?
	if !wfr.cfg.App.Federation {
		return nil, wfUserNotFoundErr
	}

	res := webfinger.Resource{
		Subject: "acct:" + username + "@" + host,
		Aliases: []string{
			c.CanonicalURL(),
			c.FederatedAccount(),
		},
		Links: []webfinger.Link{
			{
				HRef: c.CanonicalURL(),
				Type: "text/html",
				Rel:  "https://webfinger.net/rel/profile-page",
			},
			{
				HRef: c.FederatedAccount(),
				Type: "application/activity+json",
				Rel:  "self",
			},
		},
	}
	return &res, nil
}

func (wfr wfResolver) DummyUser(username string, hostname string, r []webfinger.Rel) (*webfinger.Resource, error) {
	return nil, wfUserNotFoundErr
}

func (wfr wfResolver) IsNotFoundError(err error) bool {
	return err == wfUserNotFoundErr
}

// RemoteLookup looks up a user by handle at a remote server
// and returns the actor URL
func RemoteLookup(handle string) string {
	handle = strings.TrimLeft(handle, "@")
	// let's take the server part of the handle
	parts := strings.Split(handle, "@")
	resp, err := http.Get("https://" + parts[1] + "/.well-known/webfinger?resource=acct:" + handle)
	if err != nil {
		log.Error("Error performing webfinger request", err)
		return ""
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Error("Error reading webfinger response", err)
		return ""
	}

	var result webfinger.Resource
	err = json.Unmarshal(body, &result)
	if err != nil {
		log.Error("Unsupported webfinger response received: %v", err)
		return ""
	}

	var href string
	// iterate over webfinger links and find the one with
	// a self "rel"
	for _, link := range result.Links {
		if link.Rel == "self" {
			href = link.HRef
		}
	}

	// if we didn't find it with the above then
	// try using aliases
	if href == "" {
		// take the last alias because mastodon has the
		// https://instance.tld/@user first which
		// doesn't work as an href
		href = result.Aliases[len(result.Aliases)-1]
	}

	return href
}
