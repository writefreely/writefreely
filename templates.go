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
	"fmt"
	"github.com/dustin/go-humanize"
	"github.com/writeas/web-core/l10n"
	"github.com/writeas/web-core/log"
	"html/template"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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
	}
)

const (
	templatesDir = "templates"
	pagesDir     = "pages"
)

func showUserPage(w http.ResponseWriter, name string, obj interface{}) {
	if obj == nil {
		log.Error("showUserPage: data is nil!")
		return
	}
	if err := userPages[filepath.Join("user", name+".tmpl")].ExecuteTemplate(w, name, obj); err != nil {
		log.Error("Error parsing %s: %v", name, err)
	}
}

func initTemplate(name string) {
	if debugging {
		log.Info("  %s%s%s.tmpl", templatesDir, string(filepath.Separator), name)
	}

	files := []string{
		filepath.Join(templatesDir, name+".tmpl"),
		filepath.Join(templatesDir, "include", "footer.tmpl"),
		filepath.Join(templatesDir, "base.tmpl"),
	}
	if name == "collection" || name == "collection-tags" {
		// These pages list out collection posts, so we also parse templatesDir + "include/posts.tmpl"
		files = append(files, filepath.Join(templatesDir, "include", "posts.tmpl"))
	}
	if name == "collection" || name == "collection-tags" || name == "collection-post" || name == "post" {
		files = append(files, filepath.Join(templatesDir, "include", "post-render.tmpl"))
	}
	templates[name] = template.Must(template.New("").Funcs(funcMap).ParseFiles(files...))
}

func initPage(path, key string) {
	if debugging {
		log.Info("  %s", key)
	}

	pages[key] = template.Must(template.New("").Funcs(funcMap).ParseFiles(
		path,
		filepath.Join(templatesDir, "include", "footer.tmpl"),
		filepath.Join(templatesDir, "base.tmpl"),
	))
}

func initUserPage(path, key string) {
	if debugging {
		log.Info("  %s", key)
	}

	userPages[key] = template.Must(template.New(key).Funcs(funcMap).ParseFiles(
		path,
		filepath.Join(templatesDir, "user", "include", "header.tmpl"),
		filepath.Join(templatesDir, "user", "include", "footer.tmpl"),
	))
}

func initTemplates() error {
	log.Info("Loading templates...")
	tmplFiles, err := ioutil.ReadDir(templatesDir)
	if err != nil {
		return err
	}

	for _, f := range tmplFiles {
		if !f.IsDir() && !strings.HasPrefix(f.Name(), ".") {
			parts := strings.Split(f.Name(), ".")
			key := parts[0]
			initTemplate(key)
		}
	}

	log.Info("Loading pages...")
	// Initialize all static pages that use the base template
	filepath.Walk(pagesDir, func(path string, i os.FileInfo, err error) error {
		if !i.IsDir() && !strings.HasPrefix(i.Name(), ".") {
			parts := strings.Split(path, string(filepath.Separator))
			key := i.Name()
			if len(parts) > 2 {
				key = fmt.Sprintf("%s%s%s", parts[1], string(filepath.Separator), i.Name())
			}
			initPage(path, key)
		}

		return nil
	})

	log.Info("Loading user pages...")
	// Initialize all user pages that use base templates
	filepath.Walk(filepath.Join(templatesDir, "user"), func(path string, f os.FileInfo, err error) error {
		if !f.IsDir() && !strings.HasPrefix(f.Name(), ".") {
			parts := strings.Split(path, string(filepath.Separator))
			key := f.Name()
			if len(parts) > 2 {
				key = filepath.Join(parts[1], f.Name())
			}
			initUserPage(path, key)
		}

		return nil
	})

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
	s = strings.Replace(s, "write.as", "<a href=\"https://writefreely.org\">write freely</a>", 1)
	return template.HTML(s)
}
