/*
 * Copyright © 2018 A Bunch Tell LLC.
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
	"github.com/writeas/web-core/log"
	"strings"
	"time"
)

func exportPostsCSV(u *User, posts *[]PublicPost) []byte {
	var b bytes.Buffer

	r := [][]string{
		{"id", "slug", "blog", "url", "created", "title", "body"},
	}
	for _, p := range *posts {
		var blog string
		if p.Collection != nil {
			blog = p.Collection.Alias
		}
		f := []string{p.ID, p.Slug.String, blog, p.CanonicalURL(), p.Created8601(), p.Title.String, strings.Replace(p.Content, "\n", "\\n", -1)}
		r = append(r, f)
	}

	w := csv.NewWriter(&b)
	w.WriteAll(r) // calls Flush internally
	if err := w.Error(); err != nil {
		log.Info("error writing csv:", err)
	}

	return b.Bytes()
}

type exportedTxt struct {
	Name, Body string
	Mod        time.Time
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
		files = append(files, exportedTxt{filename, p.Content, p.Created})
	}

	for _, file := range files {
		head := &zip.FileHeader{Name: file.Name}
		head.SetModTime(file.Mod)
		f, err := w.CreateHeader(head)
		if err != nil {
			log.Error("export zip header: %v", err)
		}
		_, err = f.Write([]byte(file.Body))
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

func compileFullExport(app *app, u *User) *ExportUser {
	exportUser := &ExportUser{
		User: u,
	}

	colls, err := app.db.GetCollections(u)
	if err != nil {
		log.Error("unable to fetch collections: %v", err)
	}

	posts, err := app.db.GetAnonymousPosts(u)
	if err != nil {
		log.Error("unable to fetch anon posts: %v", err)
	}
	exportUser.AnonymousPosts = *posts

	var collObjs []CollectionObj
	for _, c := range *colls {
		co := &CollectionObj{Collection: c}
		co.Posts, err = app.db.GetPosts(&c, 0, true, false)
		if err != nil {
			log.Error("unable to get collection posts: %v", err)
		}
		app.db.GetPostsCount(co, true)
		collObjs = append(collObjs, *co)
	}
	exportUser.Collections = &collObjs

	return exportUser
}
