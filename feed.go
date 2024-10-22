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
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/feeds"
	"github.com/gorilla/mux"
	stripmd "github.com/writeas/go-strip-markdown/v2"
	"github.com/writeas/web-core/log"
)

func ViewFeed(app *App, w http.ResponseWriter, req *http.Request) error {
	alias := collectionAliasFromReq(req)

	// Display collection if this is a collection
	var c *Collection
	var err error
	if app.cfg.App.SingleUser {
		c, err = app.db.GetCollectionByID(1)
	} else {
		c, err = app.db.GetCollection(alias)
	}
	if err != nil {
		return nil
	}

	silenced, err := app.db.IsUserSilenced(c.OwnerID)
	if err != nil {
		log.Error("view feed: get user: %v", err)
		return ErrInternalGeneral
	}
	if silenced {
		return ErrCollectionNotFound
	}
	c.hostName = app.cfg.App.Host

	if c.IsPrivate() || c.IsProtected() {
		return ErrCollectionNotFound
	}

	// Fetch extra data about the Collection
	// TODO: refactor out this logic, shared in collection.go:fetchCollection()
	coll := &DisplayCollection{CollectionObj: &CollectionObj{Collection: *c}}
	if c.PublicOwner {
		u, err := app.db.GetUserByID(coll.OwnerID)
		if err != nil {
			// Log the error and just continue
			log.Error("Error getting user for collection: %v", err)
		} else {
			coll.Owner = u
		}
	}

	tag := mux.Vars(req)["tag"]
	if tag != "" {
		coll.Posts, _ = app.db.GetPostsTagged(app.cfg, c, tag, 1, false)
	} else {
		coll.Posts, _ = app.db.GetPosts(app.cfg, c, 1, false, true, false, "")
	}

	author := ""
	if coll.Owner != nil {
		author = coll.Owner.Username
	}

	collectionTitle := coll.DisplayTitle()
	if tag != "" {
		collectionTitle = tag + " &mdash; " + collectionTitle
	}

	baseUrl := coll.CanonicalURL()
	basePermalinkUrl := baseUrl
	siteURL := baseUrl
	if tag != "" {
		siteURL += "tag:" + tag
	}

	feed := &feeds.Feed{
		Title:       collectionTitle,
		Link:        &feeds.Link{Href: siteURL},
		Description: coll.Description,
		Author:      &feeds.Author{author, ""},
		Created:     time.Now(),
	}

	var title, permalink string
	for _, p := range *coll.Posts {
		// Add necessary path back to the web browser for Web Monetization if needed
		p.Collection = coll.CollectionObj // augmentReadingDestination requires a populated Collection field
		p.augmentReadingDestination()
		// Create the item for the feed
		title = p.PlainDisplayTitle()
		permalink = fmt.Sprintf("%s%s", baseUrl, p.Slug.String)
		feed.Items = append(feed.Items, &feeds.Item{
			Id:          fmt.Sprintf("%s%s", basePermalinkUrl, p.Slug.String),
			Title:       title,
			Link:        &feeds.Link{Href: permalink},
			Description: "<![CDATA[" + stripmd.Strip(p.Content) + "]]>",
			Content:     string(p.HTMLContent),
			Author:      &feeds.Author{author, ""},
			Created:     p.Created,
			Updated:     p.Updated,
		})
	}

	rss, err := feed.ToRss()
	if err != nil {
		return err
	}

	fmt.Fprint(w, rss)
	return nil
}
