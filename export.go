/*
 * Copyright © 2018-2019 Musing Studio LLC.
 *
 * This file is part of WriteFreely.
 *
 * WriteFreely is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License, included
 * in the LICENSE file in this source code package.
 */

package writefreely

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"strings"
	"time"

	"github.com/writeas/web-core/log"
)

func exportPostsCSV(hostName string, u *User, posts *[]PublicPost) []byte {
	var b bytes.Buffer

	r := [][]string{
		{"id", "slug", "blog", "url", "created", "title", "body"},
	}
	for _, p := range *posts {
		var blog string
		if p.Collection != nil {
			blog = p.Collection.Alias
			p.Collection.hostName = hostName
		}
		f := []string{p.ID, p.Slug.String, blog, p.CanonicalURL(hostName), p.Created8601(), p.Title.String, strings.Replace(p.Content, "\n", "\\n", -1)}
		r = append(r, f)
	}

	w := csv.NewWriter(&b)
	w.WriteAll(r) // calls Flush internally
	if err := w.Error(); err != nil {
		log.Info("error writing csv: %v", err)
	}

	return b.Bytes()
}

type exportedTxt struct {
	Name, Title, Body string

	Mod time.Time
}

func exportPostsZip(u *User, posts *[]PublicPost) []byte {
	// Create a buffer to write our archive to.
	b := new(bytes.Buffer)

	// Create a new zip archive.
	w := zip.NewWriter(b)

	// Add some files to the archive.
	var filename string
	files := []exportedTxt{}
	for _, p := range *posts {
		filename = ""
		if p.Collection != nil {
			filename += p.Collection.Alias + "/"
		}
		if p.Slug.String != "" {
			filename += p.Slug.String + "_"
		}
		filename += p.ID + ".txt"
		files = append(files, exportedTxt{filename, p.Title.String, p.Content, p.Created})
	}

	for _, file := range files {
		head := &zip.FileHeader{Name: file.Name}
		head.SetModTime(file.Mod)
		f, err := w.CreateHeader(head)
		if err != nil {
			log.Error("export zip header: %v", err)
		}
		var fullPost string
		if file.Title != "" {
			fullPost = "# " + file.Title + "\n\n"
		}
		fullPost += file.Body
		_, err = f.Write([]byte(fullPost))
		if err != nil {
			log.Error("export zip write: %v", err)
		}
	}

	// Make sure to check the error on Close.
	err := w.Close()
	if err != nil {
		log.Error("export zip close: %v", err)
	}

	return b.Bytes()
}

func compileFullExport(app *App, u *User) *ExportUser {
	exportUser := &ExportUser{
		User: u,
	}

	colls, err := app.db.GetCollections(u, app.cfg.App.Host)
	if err != nil {
		log.Error("unable to fetch collections: %v", err)
	}

	posts, err := app.db.GetAnonymousPosts(u, 0)
	if err != nil {
		log.Error("unable to fetch anon posts: %v", err)
	}
	exportUser.AnonymousPosts = *posts

	var collObjs []CollectionObj
	for _, c := range *colls {
		co := &CollectionObj{Collection: c}
		co.Posts, err = app.db.GetPosts(app.cfg, &c, 0, true, false, true, "")
		if err != nil {
			log.Error("unable to get collection posts: %v", err)
		}
		app.db.GetPostsCount(co, true)
		collObjs = append(collObjs, *co)
	}
	exportUser.Collections = &collObjs

	return exportUser
}
