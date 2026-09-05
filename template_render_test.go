/*
 * Copyright © 2026 Musing Studio LLC.
 *
 * This file is part of WriteFreely.
 *
 * WriteFreely is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License, included
 * in the LICENSE file in this source code package.
 */

package writefreely

import (
	stdlog "log"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/writeas/web-core/auth"
	wflog "github.com/writeas/web-core/log"

	"github.com/writefreely/writefreely/config"
	"github.com/writefreely/writefreely/key"
)

// This file contains integration tests that render real pages -- especially
// authenticated "/me" pages and blog pages -- against a real (SQLite-backed)
// App and asserts that template execution didn't fail.
//
// Detecting a failed template render is trickier than checking the HTTP
// status code alone: html/template writes to the ResponseWriter as it
// executes, so a mid-render failure can leave a 200 status already sent
// with a truncated body, while the error itself only surfaces as a
// log.Error call and, if the handler propagates it, a *second* attempt to
// write the 500 error page on top of the partial output. So each check
// here does three things:
//   - capture web-core/log's error output during the request and fail if
//     it contains "template:", the prefix Go's text/template package uses
//     for every execution error
//   - fail on a >=500 status
//   - fail if the body contains the 500 page's own text, which would only
//     appear if the internal-error template got rendered (possibly
//     appended after partial output from the page that actually failed)

// newTemplateTestApp builds a fully-initialized App backed by a temporary
// SQLite database and disposable in-memory keys, for use in rendering
// tests. It skips the test if the binary wasn't built with `-tags sqlite`.
func newTemplateTestApp(t *testing.T, mutate func(cfg *config.Config)) (*App, *mux.Router) {
	t.Helper()
	if !SQLiteEnabled {
		t.Skip("SQLite support not compiled in; run with `go test -tags sqlite` to run this test")
	}

	dir := t.TempDir()

	cfg := config.New()
	cfg.Server.TemplatesParentDir = ""
	cfg.Server.PagesParentDir = ""
	cfg.Server.StaticParentDir = "testdata"
	cfg.App.Host = "http://localhost:8080"
	cfg.App.SiteName = "Test Instance"
	cfg.App.Federation = true
	cfg.App.UpdateChecks = false
	cfg.App.MinUsernameLen = 1
	cfg.App.UserInvites = "user"
	// Make the auto-created user blog fully public so we exercise the
	// normal collection/post templates rather than the password-gate flow.
	cfg.App.DefaultVisibility = "public"
	cfg.App.LocalTimeline = true
	cfg.Database.Type = driverSQLite
	cfg.Database.FileName = filepath.Join(dir, "writefreely.db")

	if mutate != nil {
		mutate(cfg)
	}

	app := &App{cfg: cfg}

	app.keys = &key.Keychain{}
	if err := app.keys.GenerateKeys(); err != nil {
		t.Fatalf("generate keys: %v", err)
	}

	if err := InitTemplates(app.cfg); err != nil {
		t.Fatalf("init templates: %v", err)
	}

	app.InitSession()
	app.InitDecoder()

	connectToDatabase(app)
	t.Cleanup(func() { app.db.Close() })

	if err := adminInitDatabase(app); err != nil {
		t.Fatalf("init database: %v", err)
	}

	initActivityPub(app)
	if app.cfg.App.LocalTimeline {
		initLocalTimeline(app)
	}

	router := mux.NewRouter()
	InitRoutes(app, router)
	app.InitStaticRoutes(router)

	return app, router
}

// loginCookie returns a session cookie that authenticates as the given user,
// bypassing the login form/handler entirely.
func loginCookie(t *testing.T, app *App, u *User) *http.Cookie {
	t.Helper()

	r := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	session, err := app.sessionStore.New(r, cookieName)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	session.Values[cookieUserVal] = u.Cookie()
	if err := session.Save(r, w); err != nil {
		t.Fatalf("save session: %v", err)
	}

	for _, c := range w.Result().Cookies() {
		if c.Name == cookieName {
			return c
		}
	}
	t.Fatal("login did not produce a session cookie")
	return nil
}

// renderedRequest performs an HTTP request against the router and returns
// the response along with any error-level log output produced while
// handling it.
func renderedRequest(t *testing.T, router *mux.Router, method, path string, cookies []*http.Cookie) (*httptest.ResponseRecorder, string) {
	t.Helper()

	var logBuf strings.Builder
	prevErrorLog := wflog.ErrorLog
	wflog.ErrorLog = stdlog.New(&logBuf, "", 0)
	defer func() { wflog.ErrorLog = prevErrorLog }()

	req := httptest.NewRequest(method, path, nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	return rec, logBuf.String()
}

// assertRendersCleanly performs the request and fails the test if there's
// any sign the page failed to render -- see the file-level comment for what
// "any sign" means.
func assertRendersCleanly(t *testing.T, router *mux.Router, method, path string, cookies []*http.Cookie, wantStatus int) *httptest.ResponseRecorder {
	t.Helper()

	rec, logOutput := renderedRequest(t, router, method, path, cookies)

	if wantStatus > 0 && rec.Code != wantStatus {
		t.Errorf("%s %s: status = %d, want %d (body: %s)", method, path, rec.Code, wantStatus, truncateForLog(rec.Body.String()))
	}
	if rec.Code >= http.StatusInternalServerError {
		t.Errorf("%s %s: got server error status %d (body: %s)", method, path, rec.Code, truncateForLog(rec.Body.String()))
	}
	if strings.Contains(logOutput, "template:") {
		t.Errorf("%s %s: template rendering error logged: %s", method, path, logOutput)
	}
	if strings.Contains(rec.Body.String(), "There seems to be an issue with this server") {
		t.Errorf("%s %s: response contains the embedded 500 page, meaning rendering failed partway through:\n%s", method, path, truncateForLog(rec.Body.String()))
	}

	return rec
}

func truncateForLog(s string) string {
	const max = 1000
	if len(s) <= max {
		return s
	}
	return s[:max] + "...(truncated)"
}

// createTemplateTestUser creates a user (and their auto-generated blog) and
// a single published post in it, returning all three.
func createTemplateTestUser(t *testing.T, app *App, username string) (*User, *Collection, *Post) {
	t.Helper()

	hashedPass, err := auth.HashPass([]byte("testpassword"))
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	u := &User{Username: username, HashedPass: hashedPass}
	if err := app.db.CreateUser(app.cfg, u, "", ""); err != nil {
		t.Fatalf("create user %q: %v", username, err)
	}

	coll, err := app.db.GetCollection(username)
	if err != nil {
		t.Fatalf("get collection for %q: %v", username, err)
	}

	title := "Hello World"
	content := "This is a **test** post used to exercise template rendering.\n\nIt has multiple paragraphs, and a [link](https://example.com)."
	post, err := app.db.CreatePost(app.cfg, u.ID, coll.ID, &SubmittedPost{Title: &title, Content: &content})
	if err != nil {
		t.Fatalf("create post: %v", err)
	}

	return u, coll, post
}

// TestTemplateRendering_MeAndBlogPages exercises the authenticated "/me"
// pages and blog rendering across single-user and multi-user instances (the
// latter with Chorus both on and off), asserting nothing fails to render.
func TestTemplateRendering_MeAndBlogPages(t *testing.T) {
	configs := []struct {
		name   string
		mutate func(cfg *config.Config)
	}{
		{
			name: "SingleUser",
			mutate: func(cfg *config.Config) {
				cfg.App.SingleUser = true
			},
		},
		{
			name: "MultiUser_ChorusOff",
			mutate: func(cfg *config.Config) {
				cfg.App.SingleUser = false
				cfg.App.Chorus = false
			},
		},
		{
			name: "MultiUser_ChorusOn",
			mutate: func(cfg *config.Config) {
				cfg.App.SingleUser = false
				cfg.App.Chorus = true
			},
		},
	}

	for _, tc := range configs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			app, router := newTemplateTestApp(t, tc.mutate)
			u, coll, post := createTemplateTestUser(t, app, "tester")
			cookie := loginCookie(t, app, u)

			slug := post.Slug.String
			if slug == "" {
				slug = post.ID
			}

			blogPrefix := ""
			if !app.cfg.App.SingleUser {
				blogPrefix = "/" + coll.Alias
			}

			type pageCase struct {
				name       string
				path       string
				authed     bool
				wantStatus int // 0 means "just don't 5xx or fail to render"
			}

			cases := []pageCase{
				// Blog rendering
				{name: "blog index (anon)", path: blogPrefix + "/", authed: false, wantStatus: http.StatusOK},
				{name: "blog index (owner)", path: blogPrefix + "/", authed: true, wantStatus: http.StatusOK},
				{name: "blog archive (anon)", path: blogPrefix + "/archive/", authed: false, wantStatus: http.StatusOK},
				{name: "post view (anon)", path: blogPrefix + "/" + slug, authed: false, wantStatus: http.StatusOK},
				{name: "post view (owner)", path: blogPrefix + "/" + slug, authed: true, wantStatus: http.StatusOK},
				{name: "post edit (owner)", path: blogPrefix + "/" + slug + "/edit", authed: true, wantStatus: http.StatusOK},

				// Reader
				{name: "reader", path: "/read", authed: false, wantStatus: http.StatusOK},

				// Authenticated "/me" pages
				{name: "me: collections list", path: "/me/c/", authed: true, wantStatus: http.StatusOK},
				{name: "me: edit collection", path: "/me/c/" + coll.Alias, authed: true, wantStatus: http.StatusOK},
				{name: "me: collection stats", path: "/me/c/" + coll.Alias + "/stats", authed: true, wantStatus: http.StatusOK},
				{name: "me: posts list", path: "/me/posts/", authed: true, wantStatus: http.StatusOK},
				{name: "me: export", path: "/me/export", authed: true, wantStatus: http.StatusOK},
				{name: "me: import", path: "/me/import", authed: true, wantStatus: http.StatusOK},
				{name: "me: invites", path: "/me/invites", authed: true, wantStatus: http.StatusOK},
				{name: "me: settings", path: "/me/settings", authed: true, wantStatus: http.StatusOK},
			}

			if app.cfg.App.SingleUser {
				cases = append(cases, pageCase{name: "pad: new post", path: "/me/new", authed: true, wantStatus: http.StatusOK})
			} else {
				cases = append(cases, pageCase{name: "pad: new post", path: "/new", authed: true, wantStatus: http.StatusOK})
				cases = append(cases,
					pageCase{name: "home (anon)", path: "/", authed: false},
					pageCase{name: "home (authed)", path: "/", authed: true},
				)
			}

			for _, c := range cases {
				c := c
				t.Run(c.name, func(t *testing.T) {
					var cookies []*http.Cookie
					if c.authed {
						cookies = []*http.Cookie{cookie}
					}
					assertRendersCleanly(t, router, "GET", c.path, cookies, c.wantStatus)
				})
			}
		})
	}
}
