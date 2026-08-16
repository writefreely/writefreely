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
	"errors"
	"html/template"
	"io"
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/dustin/go-humanize"
	"github.com/writeas/web-core/l10n"
	"github.com/writeas/web-core/log"
	"github.com/writefreely/writefreely/author"
	"github.com/writefreely/writefreely/config"
)

var (
	templates = map[string]*template.Template{}
	pages     = map[string]*template.Template{}
	userPages = map[string]*template.Template{}
	funcMap   = template.FuncMap{
		"largeNumFmt": largeNumFmt,
		"pluralize":   pluralize,
		"isRTL":       isRTL,
		"isLTR":       isLTR,
		"localstr":    localStr,
		"localhtml":   localHTML,
		"tolower":     strings.ToLower,
		"title":       strings.Title,
		"hasPrefix":   strings.HasPrefix,
		"hasSuffix":   strings.HasSuffix,
		"dict":        dict,
	}
)

const (
	templatesDir = "templates"
	pagesDir     = "pages"
)

// tmplFile identifies a template file within a specific filesystem, since
// the files that make up a single parsed template can be split across the
// independently-configurable templates/ and pages/ trees.
type tmplFile struct {
	fsys fs.FS
	name string
}

// parseTemplateSet parses files into t in order, associating each with t the
// same way (html/template.Template).ParseFiles does -- it just reads from
// each file's own fs.FS instead of assuming a single filesystem.
func parseTemplateSet(t *template.Template, files ...tmplFile) (*template.Template, error) {
	if len(files) == 0 {
		return nil, errors.New("template: no files named in call")
	}
	for _, tf := range files {
		b, err := fs.ReadFile(tf.fsys, tf.name)
		if err != nil {
			return nil, err
		}
		name := path.Base(tf.name)
		var tmpl *template.Template
		if name == t.Name() {
			tmpl = t
		} else {
			tmpl = t.New(name)
		}
		_, err = tmpl.Parse(string(b))
		if err != nil {
			return nil, err
		}
	}
	return t, nil
}

func showUserPage(w http.ResponseWriter, name string, obj interface{}) {
	if obj == nil {
		log.Error("showUserPage: data is nil!")
		return
	}
	if err := userPages[path.Join("user", name+".tmpl")].ExecuteTemplate(w, name, obj); err != nil {
		log.Error("Error parsing %s: %v", name, err)
	}
}

func initTemplate(tfs fs.FS, name string) {
	if debugging {
		log.Info("  " + name + ".tmpl")
	}

	files := []tmplFile{
		{tfs, name + ".tmpl"},
		{tfs, path.Join("include", "footer.tmpl")},
		{tfs, "base.tmpl"},
		{tfs, path.Join("user", "include", "silenced.tmpl")},
	}
	if name == "collection" || name == "collection-tags" || name == "collection-archive" || name == "chorus-collection" || name == "read" {
		// These pages list out collection posts, so we also parse templatesDir + "include/posts.tmpl"
		files = append(files, tmplFile{tfs, path.Join("include", "posts.tmpl")})
	}
	if name == "chorus-collection" || name == "chorus-collection-post" {
		files = append(files, tmplFile{tfs, path.Join("user", "include", "header.tmpl")})
	}
	if name == "collection" || name == "collection-tags" || name == "collection-archive" || name == "collection-post" || name == "post" || name == "chorus-collection" || name == "chorus-collection-post" {
		files = append(files, tmplFile{tfs, path.Join("include", "post-render.tmpl")})
	}
	templates[name] = template.Must(parseTemplateSet(template.New("").Funcs(funcMap), files...))
}

func initPage(tfs, pfs fs.FS, pagePath, key string) {
	if debugging {
		log.Info("  [%s] %s", key, pagePath)
	}

	files := []tmplFile{
		{pfs, pagePath},
		{tfs, path.Join("include", "footer.tmpl")},
		{tfs, "base.tmpl"},
		{tfs, path.Join("user", "include", "silenced.tmpl")},
	}

	if key == "login.tmpl" || key == "landing.tmpl" || key == "signup.tmpl" {
		files = append(files, tmplFile{tfs, path.Join("include", "oauth.tmpl")})
	}

	pages[key] = template.Must(parseTemplateSet(template.New("").Funcs(funcMap), files...))
}

func initUserPage(tfs fs.FS, userPagePath, key string) {
	if debugging {
		log.Info("  [%s] %s", key, userPagePath)
	}

	userPages[key] = template.Must(parseTemplateSet(template.New(key).Funcs(funcMap),
		tmplFile{tfs, userPagePath},
		tmplFile{tfs, path.Join("user", "include", "header.tmpl")},
		tmplFile{tfs, path.Join("user", "include", "footer.tmpl")},
		tmplFile{tfs, path.Join("user", "include", "silenced.tmpl")},
		tmplFile{tfs, path.Join("user", "include", "nav.tmpl")},
	))
}

// InitTemplates loads all template files from the configured parent dir.
func InitTemplates(cfg *config.Config) error {
	tfs := templatesFileSystem(cfg.Server.TemplatesParentDir)
	pfs := pagesFileSystem(cfg.Server.PagesParentDir)
	// Let the author package resolve reserved usernames against the same
	// (possibly embedded) pages filesystem instead of reading disk directly.
	author.PagesFS = pfs

	log.Info("Loading templates...")
	tmplFiles, err := fs.ReadDir(tfs, ".")
	if err != nil {
		return err
	}

	for _, f := range tmplFiles {
		if !f.IsDir() && !strings.HasPrefix(f.Name(), ".") {
			parts := strings.Split(f.Name(), ".")
			key := parts[0]
			initTemplate(tfs, key)
		}
	}

	log.Info("Loading pages...")
	// Initialize all static pages that use the base template
	err = fs.WalkDir(pfs, ".", func(p string, i fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !i.IsDir() && !strings.HasPrefix(i.Name(), ".") {
			key := i.Name()
			initPage(tfs, pfs, p, key)
		}

		return nil
	})
	if err != nil {
		return err
	}

	log.Info("Loading user pages...")
	// Initialize all user pages that use base templates
	err = fs.WalkDir(tfs, "user", func(p string, f fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !f.IsDir() && !strings.HasPrefix(f.Name(), ".") {
			// However deep the file is under templates/user/, it's keyed
			// the same way showUserPage looks it up: "user/<basename>".
			key := path.Join("user", f.Name())
			initUserPage(tfs, p, key)
		}

		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

// renderPage retrieves the given template and renders it to the given io.Writer.
// If something goes wrong, the error is logged and returned.
func renderPage(w io.Writer, tmpl string, data interface{}) error {
	err := pages[tmpl].ExecuteTemplate(w, "base", data)
	if err != nil {
		log.Error("%v", err)
	}
	return err
}

func largeNumFmt(n int64) string {
	return humanize.Comma(n)
}

func pluralize(singular, plural string, n int64) string {
	if n == 1 {
		return singular
	}
	return plural
}

func isRTL(d string) bool {
	return d == "rtl"
}

func isLTR(d string) bool {
	return d == "ltr" || d == "auto"
}

func localStr(term, lang string) string {
	s := l10n.Strings(lang)[term]
	if s == "" {
		s = l10n.Strings("")[term]
	}
	return s
}

func localHTML(term, lang string) template.HTML {
	s := l10n.Strings(lang)[term]
	if s == "" {
		s = l10n.Strings("")[term]
	}
	s = strings.Replace(s, "write.as", "<a href=\"https://writefreely.org\">writefreely</a>", 1)
	return template.HTML(s)
}

// from: https://stackoverflow.com/a/18276968/1549194
func dict(values ...interface{}) (map[string]interface{}, error) {
	if len(values)%2 != 0 {
		return nil, errors.New("dict: invalid number of parameters")
	}
	dict := make(map[string]interface{}, len(values)/2)
	for i := 0; i < len(values); i += 2 {
		key, ok := values[i].(string)
		if !ok {
			return nil, errors.New("dict: keys must be strings")
		}
		dict[key] = values[i+1]
	}
	return dict, nil
}
