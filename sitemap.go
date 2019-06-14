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
	"github.com/gorilla/mux"
	"github.com/ikeikeikeike/go-sitemap-generator/stm"
	"github.com/writeas/web-core/log"
	"net/http"
	"time"
)

func buildSitemap(host, alias string) *stm.Sitemap {
	sm := stm.NewSitemap()
	sm.SetDefaultHost(host)
	if alias != "/" {
		sm.SetSitemapsPath(alias)
	}

	sm.Create()

	// Note: Do not call `sm.Finalize()` because it flushes
	// the underlying datastructure from memory to disk.

	return sm
}

func handleViewSitemap(app *App, w http.ResponseWriter, r *http.Request) error {
	vars := mux.Vars(r)

	// Determine canonical blog URL
	alias := vars["collection"]
	subdomain := vars["subdomain"]
	isSubdomain := subdomain != ""
	if isSubdomain {
		alias = subdomain
	}

	host := fmt.Sprintf("%s/%s/", app.cfg.App.Host, alias)
	var c *Collection
	var err error
	pre := "/"
	if app.cfg.App.SingleUser {
		c, err = app.db.GetCollectionByID(1)
	} else {
		c, err = app.db.GetCollection(alias)
	}
	if err != nil {
		return err
	}
	c.hostName = app.cfg.App.Host

	if !isSubdomain {
		pre += alias + "/"
	}
	host = c.CanonicalURL()

	sm := buildSitemap(host, pre)
	posts, err := app.db.GetPosts(c, 0, false, false, false)
	if err != nil {
		log.Error("Error getting posts: %v", err)
		return err
	}
	lastSiteMod := time.Now()
	for i, p := range *posts {
		if i == 0 {
			lastSiteMod = p.Updated
		}
		u := stm.URL{
			"loc":        p.Slug.String,
			"changefreq": "weekly",
			"mobile":     true,
			"lastmod":    p.Updated,
		}
		if len(p.Images) > 0 {
			imgs := []stm.URL{}
			for _, i := range p.Images {
				imgs = append(imgs, stm.URL{"loc": i, "title": ""})
			}
			u["image"] = imgs
		}
		sm.Add(u)
	}

	// Add top URL
	sm.Add(stm.URL{
		"loc":        pre,
		"changefreq": "daily",
		"priority":   "1.0",
		"lastmod":    lastSiteMod,
	})

	w.Write(sm.XMLContent())

	return nil
}
