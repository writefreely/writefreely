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
	"github.com/dustin/go-humanize"
	"github.com/writeas/web-core/l10n"
	"github.com/writeas/web-core/log"
	"github.com/writeas/writefreely/config"
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

func initTemplate(parentDir, name string) {
	if debugging {
		log.Info("  " + filepath.Join(parentDir, templatesDir, name+".tmpl"))
	}

	files := []string{
		filepath.Join(parentDir, templatesDir, name+".tmpl"),
		filepath.Join(parentDir, templatesDir, "include", "footer.tmpl"),
		filepath.Join(parentDir, templatesDir, "base.tmpl"),
	}
	if name == "collection" || name == "collection-tags" || name == "chorus-collection" {
		// These pages list out collection posts, so we also parse templatesDir + "include/posts.tmpl"
		files = append(files, filepath.Join(parentDir, templatesDir, "include", "posts.tmpl"))
	}
	if name == "chorus-collection" || name == "chorus-collection-post" {
		files = append(files, filepath.Join(parentDir, templatesDir, "user", "include", "header.tmpl"))
	}
	if name == "collection" || name == "collection-tags" || name == "collection-post" || name == "post" || name == "chorus-collection" || name == "chorus-collection-post" {
		files = append(files, filepath.Join(parentDir, templatesDir, "include", "post-render.tmpl"))
	}
	templates[name] = template.Must(template.New("").Funcs(funcMap).ParseFiles(files...))
}

func initPage(parentDir, path, key string) {
	if debugging {
		log.Info("  [%s] %s", key, path)
	}

	pages[key] = template.Must(template.New("").Funcs(funcMap).ParseFiles(
		path,
		filepath.Join(parentDir, templatesDir, "include", "footer.tmpl"),
		filepath.Join(parentDir, templatesDir, "base.tmpl"),
	))
}

func initUserPage(parentDir, path, key string) {
	if debugging {
		log.Info("  [%s] %s", key, path)
	}

	userPages[key] = template.Must(template.New(key).Funcs(funcMap).ParseFiles(
		path,
		filepath.Join(parentDir, templatesDir, "user", "include", "header.tmpl"),
		filepath.Join(parentDir, templatesDir, "user", "include", "footer.tmpl"),
	))
}

// InitTemplates loads all template files from the configured parent dir.
func InitTemplates(cfg *config.Config) error {
	log.Info("Loading templates...")
	tmplFiles, err := ioutil.ReadDir(filepath.Join(cfg.Server.TemplatesParentDir, templatesDir))
	if err != nil {
		return err
	}

	for _, f := range tmplFiles {
		if !f.IsDir() && !strings.HasPrefix(f.Name(), ".") {
			parts := strings.Split(f.Name(), ".")
			key := parts[0]
			initTemplate(cfg.Server.TemplatesParentDir, key)
		}
	}

	log.Info("Loading pages...")
	// Initialize all static pages that use the base template
	filepath.Walk(filepath.Join(cfg.Server.PagesParentDir, pagesDir), func(path string, i os.FileInfo, err error) error {
		if !i.IsDir() && !strings.HasPrefix(i.Name(), ".") {
			key := i.Name()
			initPage(cfg.Server.PagesParentDir, path, key)
		}

		return nil
	})

	log.Info("Loading user pages...")
	// Initialize all user pages that use base templates
	filepath.Walk(filepath.Join(cfg.Server.TemplatesParentDir, templatesDir, "user"), func(path string, f os.FileInfo, err error) error {
		if !f.IsDir() && !strings.HasPrefix(f.Name(), ".") {
			corePath := path
			if cfg.Server.TemplatesParentDir != "" {
				corePath = corePath[len(cfg.Server.TemplatesParentDir)+1:]
			}
			parts := strings.Split(corePath, string(filepath.Separator))
			key := f.Name()
			if len(parts) > 2 {
				key = filepath.Join(parts[1], f.Name())
			}
			initUserPage(cfg.Server.TemplatesParentDir, path, key)
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
	s = strings.Replace(s, "write.as", "<a href=\"https://writefreely.org\">writefreely</a>", 1)
	return template.HTML(s)
}
