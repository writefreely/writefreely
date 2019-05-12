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
	"fmt"
	. "github.com/gorilla/feeds"
	"github.com/gorilla/mux"
	stripmd "github.com/writeas/go-strip-markdown"
	"github.com/writeas/web-core/log"
	"net/http"
	"time"
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
		coll.Posts, _ = app.db.GetPostsTagged(c, tag, 1, false)
	} else {
		coll.Posts, _ = app.db.GetPosts(c, 1, false, true)
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

	feed := &Feed{
		Title:       collectionTitle,
		Link:        &Link{Href: siteURL},
		Description: coll.Description,
		Author:      &Author{author, ""},
		Created:     time.Now(),
	}

	var title, permalink string
	for _, p := range *coll.Posts {
		title = p.PlainDisplayTitle()
		permalink = fmt.Sprintf("%s%s", baseUrl, p.Slug.String)
		feed.Items = append(feed.Items, &Item{
			Id:          fmt.Sprintf("%s%s", basePermalinkUrl, p.Slug.String),
			Title:       title,
			Link:        &Link{Href: permalink},
			Description: "<![CDATA[" + stripmd.Strip(p.Content) + "]]>",
			Content:     applyMarkdown([]byte(p.Content), ""),
			Author:      &Author{author, ""},
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
