//go:build sqlite

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
	"database/sql"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/schema"
	"github.com/stretchr/testify/assert"
	"github.com/writeas/impart"
	"github.com/writefreely/writefreely/config"
	"github.com/writefreely/writefreely/key"
)

var initTemplatesOnce sync.Once

// newSignupTestApp builds a real, sqlite-backed App suitable for exercising
// the HTTP signup handlers end-to-end. It's intentionally minimal: only the
// pieces those handlers actually touch (db, cfg, keys, session store, form
// decoder, and the global page/template caches) are initialized.
func newSignupTestApp(t *testing.T) *App {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "writefreely.db")
	db, err := sql.Open("sqlite3_with_regex", dbPath+"?parseTime=true&cached=shared")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	cfg := config.New()
	cfg.UseSQLite(true)
	cfg.Database.FileName = dbPath
	cfg.App.Host = "http://localhost:0"
	cfg.App.SingleUser = false
	cfg.App.OpenRegistration = false
	cfg.App.MinUsernameLen = 3
	cfg.Server.HashSeed = "test-hash-seed"

	keys := &key.Keychain{}
	if err := keys.GenerateKeys(); err != nil {
		t.Fatalf("generate keys: %v", err)
	}

	app := &App{
		db:   &datastore{DB: db, driverName: driverSQLite},
		cfg:  cfg,
		keys: keys,
	}
	app.formDecoder = schema.NewDecoder()
	app.InitSession()

	if err := adminInitDatabase(app); err != nil {
		t.Fatalf("init schema: %v", err)
	}

	var tmplErr error
	initTemplatesOnce.Do(func() {
		tmplErr = InitTemplates(cfg)
	})
	if tmplErr != nil {
		t.Fatalf("init templates: %v", tmplErr)
	}

	return app
}

// seedInvite inserts a directly-usable invite code owned by ownerID, with no
// expiration or use limit.
func seedInvite(t *testing.T, app *App, code string, ownerID int64) {
	t.Helper()
	_, err := app.db.Exec("INSERT INTO userinvites (id, owner_id, max_uses, created, expires, inactive) VALUES (?, ?, NULL, ?, NULL, 0)",
		code, ownerID, time.Now().UTC().Format("2006-01-02 15:04:05"))
	if err != nil {
		t.Fatalf("seed invite: %v", err)
	}
}

func userExists(t *testing.T, app *App, username string) bool {
	t.Helper()
	var id int64
	err := app.db.QueryRow("SELECT id FROM users WHERE username = ?", username).Scan(&id)
	if err == sql.ErrNoRows {
		return false
	}
	if err != nil {
		t.Fatalf("query user: %v", err)
	}
	return true
}

// --- Regression coverage for #1722: POST /auth/signup (handleWebSignup) ---
// No test previously covered this path even though the fix landed there.

func TestWebSignupClosedRegistration(t *testing.T) {
	cases := []struct {
		name       string
		invite     string
		wantStatus int
		wantUser   bool
	}{
		{"no invite code", "", http.StatusForbidden, false},
		{"bogus invite code", "not-a-real-code", http.StatusForbidden, false},
		{"valid invite code", "webvalid1", http.StatusFound, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := newSignupTestApp(t)
			app.cfg.App.OpenRegistration = false

			if tc.invite == "webvalid1" {
				seedInvite(t, app, "webvalid1", 0)
			}

			username := "websignup-" + strings.ReplaceAll(tc.name, " ", "-")
			form := url.Values{}
			form.Set("alias", username)
			form.Set("pass", "sup3rSecret!")
			form.Set("invite_code", tc.invite)

			req := httptest.NewRequest("POST", "/auth/signup", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			w := httptest.NewRecorder()

			err := handleWebSignup(app, w, req)

			// handleWebSignup always returns an impart.HTTPError: a 302 to
			// "to" on success, or the rejection status on failure.
			httpErr, ok := err.(impart.HTTPError)
			if !ok {
				t.Fatalf("expected impart.HTTPError, got %T (%v)", err, err)
			}
			if httpErr.Status != tc.wantStatus {
				t.Errorf("status = %d, want %d", httpErr.Status, tc.wantStatus)
			}

			assert.Equal(t, tc.wantUser, userExists(t, app, username),
				"unexpected account-creation outcome for case %q", tc.name)
		})
	}
}

// --- Finding A: POST /api/auth/signup (apiSignup -> signup) ---

func TestAPISignupClosedRegistration(t *testing.T) {
	cases := []struct {
		name       string
		invite     string
		wantStatus int
		wantUser   bool
	}{
		{"no invite code", "", http.StatusForbidden, false},
		{"bogus invite code", "not-a-real-code", http.StatusForbidden, false},
		{"valid invite code", "valid1", http.StatusFound /* signupWithRegistration succeeds */, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := newSignupTestApp(t)
			app.cfg.App.OpenRegistration = false

			if tc.invite == "valid1" {
				seedInvite(t, app, "valid1", 0)
			}

			username := "apisignup-" + strings.ReplaceAll(tc.name, " ", "-")
			form := url.Values{}
			form.Set("alias", username)
			form.Set("pass", "sup3rSecret!")
			form.Set("invite_code", tc.invite)

			req := httptest.NewRequest("POST", "/api/auth/signup", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			w := httptest.NewRecorder()

			err := apiSignup(app, w, req)

			if !tc.wantUser {
				httpErr, ok := err.(impart.HTTPError)
				if !ok {
					t.Fatalf("expected impart.HTTPError, got %T (%v)", err, err)
				}
				if httpErr.Status != tc.wantStatus {
					t.Errorf("status = %d, want %d", httpErr.Status, tc.wantStatus)
				}
			} else if err != nil {
				t.Fatalf("expected signup to succeed, got error: %v", err)
			}

			assert.Equal(t, tc.wantUser, userExists(t, app, username),
				"unexpected account-creation outcome for case %q", tc.name)
		})
	}
}

func TestAPISignupOpenRegistrationStillWorks(t *testing.T) {
	app := newSignupTestApp(t)
	app.cfg.App.OpenRegistration = true

	form := url.Values{}
	form.Set("alias", "apisignup-open")
	form.Set("pass", "sup3rSecret!")

	req := httptest.NewRequest("POST", "/api/auth/signup", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	if err := apiSignup(app, w, req); err != nil {
		t.Fatalf("expected signup to succeed with open registration, got: %v", err)
	}
	assert.True(t, userExists(t, app, "apisignup-open"))
}

// TestSignupDisabledPasswordAuth guards against creating a password-based
// account when password auth is disabled.
func TestSignupDisabledPasswordAuth(t *testing.T) {
	newReq := func(path, alias string) (*http.Request, *httptest.ResponseRecorder) {
		form := url.Values{}
		form.Set("alias", alias)
		form.Set("pass", "sup3rSecret!")
		req := httptest.NewRequest("POST", path, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		return req, httptest.NewRecorder()
	}

	t.Run("api", func(t *testing.T) {
		app := newSignupTestApp(t)
		app.cfg.App.OpenRegistration = true
		app.cfg.App.DisablePasswordAuth = true

		req, w := newReq("/api/auth/signup", "pwauth-api")
		if err := apiSignup(app, w, req); err != ErrDisabledPasswordAuth {
			t.Fatalf("expected ErrDisabledPasswordAuth, got: %v", err)
		}
		assert.False(t, userExists(t, app, "pwauth-api"))
	})

	t.Run("web", func(t *testing.T) {
		app := newSignupTestApp(t)
		app.cfg.App.OpenRegistration = true
		app.cfg.App.DisablePasswordAuth = true

		req, w := newReq("/auth/signup", "pwauth-web")
		// handleWebSignup converts the rejection into a flash + 302 redirect
		// rather than returning the error verbatim, so the real signal is that
		// no account was created.
		err := handleWebSignup(app, w, req)
		if httpErr, ok := err.(impart.HTTPError); !ok || httpErr.Status != http.StatusFound {
			t.Fatalf("expected 302 redirect, got: %v", err)
		}
		assert.False(t, userExists(t, app, "pwauth-web"))
	})
}

// TestInitRoutesAlwaysRegistersAPISignup guards against the routing-time
// bypass in Finding A: /api/auth/signup used to only be registered when
// OpenRegistration was true *at boot*, so toggling the setting off at
// runtime (via the admin panel) left the route live with no invite check
// at all. It must now always resolve, regardless of the setting.
func TestInitRoutesAlwaysRegistersAPISignup(t *testing.T) {
	app := newSignupTestApp(t)
	app.cfg.App.OpenRegistration = false

	router := InitRoutes(app, mux.NewRouter())

	req := httptest.NewRequest("POST", "/api/auth/signup", nil)
	var match mux.RouteMatch
	if !router.Match(req, &match) {
		t.Fatal("expected /api/auth/signup to match a route even when OpenRegistration is false")
	}
}

// --- Finding B: POST /oauth/signup (viewOauthSignup) ---

func newTestOauthHandler(app *App) oauthHandler {
	return oauthHandler{
		Config:   app.cfg,
		DB:       app.db,
		Store:    app.sessionStore,
		EmailKey: app.keys.EmailKey,
	}
}

func signOauthParams(hashSeed string, p oauthSignupPageParams) string {
	return p.HashTokenParams(hashSeed)
}

func TestOAuthSignupClosedRegistration(t *testing.T) {
	cases := []struct {
		name     string
		invite   string
		wantUser bool
	}{
		{"no invite code", "", false},
		{"bogus invite code", "not-a-real-code", false},
		{"valid invite code", "oauthok", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := newSignupTestApp(t)
			app.cfg.App.OpenRegistration = false
			h := newTestOauthHandler(app)

			if tc.invite == "oauthok" {
				seedInvite(t, app, "oauthok", 0)
			}

			username := "oauthsignup-" + strings.ReplaceAll(tc.name, " ", "-")
			tp := oauthSignupPageParams{
				AccessToken:     "tok-" + username,
				TokenUsername:   username,
				TokenAlias:      username,
				TokenEmail:      "",
				TokenRemoteUser: "remote-" + username,
				ClientID:        "client1",
				Provider:        "generic",
				InviteCode:      tc.invite,
			}
			sig := signOauthParams(app.cfg.Server.HashSeed, tp)

			form := url.Values{}
			form.Set(oauthParamAccessToken, tp.AccessToken)
			form.Set(oauthParamTokenUsername, tp.TokenUsername)
			form.Set(oauthParamTokenAlias, tp.TokenAlias)
			form.Set(oauthParamTokenEmail, tp.TokenEmail)
			form.Set(oauthParamTokenRemoteUserID, tp.TokenRemoteUser)
			form.Set(oauthParamClientID, tp.ClientID)
			form.Set(oauthParamProvider, tp.Provider)
			form.Set(oauthParamInviteCode, tp.InviteCode)
			form.Set(oauthParamHash, sig)
			form.Set(oauthParamUsername, username)

			req := httptest.NewRequest("POST", "/oauth/signup", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			w := httptest.NewRecorder()

			err := h.viewOauthSignup(app, w, req)

			// On rejection, viewOauthSignup re-renders the signup-oauth page
			// inline with a flash message rather than returning an
			// impart.HTTPError (mirroring showOauthSignupPage's contract), so
			// the account-creation outcome in the DB is the real signal here.
			if !tc.wantUser {
				if err != nil {
					t.Fatalf("expected inline re-render (nil error), got: %v", err)
				}
				if !strings.Contains(w.Body.String(), "Registration is closed") {
					t.Errorf("expected rejection message in response body, got: %s", w.Body.String())
				}
			} else if err != nil {
				t.Fatalf("expected oauth signup to succeed, got error: %v", err)
			}

			assert.Equal(t, tc.wantUser, userExists(t, app, username),
				"unexpected account-creation outcome for case %q", tc.name)
		})
	}
}

// TestOAuthSignupCannotSwapInviteCodeWithoutInvalidatingSignature proves the
// replay bypass from Finding B is closed: a signature minted for one
// invite_code (or none) can no longer be reused with a different invite_code
// value, since InviteCode is now part of the signed payload.
func TestOAuthSignupCannotSwapInviteCodeWithoutInvalidatingSignature(t *testing.T) {
	app := newSignupTestApp(t)
	app.cfg.App.OpenRegistration = true // signature minted while registration was open
	h := newTestOauthHandler(app)

	username := "oauthsignup-swap"
	tp := oauthSignupPageParams{
		AccessToken:     "tok-swap",
		TokenUsername:   username,
		TokenAlias:      username,
		TokenRemoteUser: "remote-swap",
		ClientID:        "client1",
		Provider:        "generic",
		InviteCode:      "", // signed with an empty invite code
	}
	sig := signOauthParams(app.cfg.Server.HashSeed, tp)

	// Registration is closed by the time the (replayed) form is submitted.
	app.cfg.App.OpenRegistration = false

	form := url.Values{}
	form.Set(oauthParamAccessToken, tp.AccessToken)
	form.Set(oauthParamTokenUsername, tp.TokenUsername)
	form.Set(oauthParamTokenAlias, tp.TokenAlias)
	form.Set(oauthParamTokenRemoteUserID, tp.TokenRemoteUser)
	form.Set(oauthParamClientID, tp.ClientID)
	form.Set(oauthParamProvider, tp.Provider)
	form.Set(oauthParamInviteCode, "swapped-in-bogus-code") // tampered after signing
	form.Set(oauthParamHash, sig)
	form.Set(oauthParamUsername, username)

	req := httptest.NewRequest("POST", "/oauth/signup", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	err := h.viewOauthSignup(app, w, req)
	httpErr, ok := err.(impart.HTTPError)
	if !ok {
		t.Fatalf("expected impart.HTTPError, got %T (%v)", err, err)
	}
	if httpErr.Status != http.StatusBadRequest {
		t.Errorf("status = %d, want %d (tampered request)", httpErr.Status, http.StatusBadRequest)
	}
	assert.False(t, userExists(t, app, username))
}

// TestOAuthSignupNormalizesUsername is regression coverage for #648 and #844:
// usernames created via OAuth must be run through the same slug normalization as
// normal registration, so the resulting collection alias is always lowercase.
// Otherwise the blog 404s, since collection URLs are redirected to their
// lowercase form on request but no matching (lowercase) alias exists.
func TestOAuthSignupNormalizesUsername(t *testing.T) {
	app := newSignupTestApp(t)
	app.cfg.App.OpenRegistration = true
	h := newTestOauthHandler(app)

	const submitted = "MixedCaseUser"
	const normalized = "mixedcaseuser"

	tp := oauthSignupPageParams{
		AccessToken:     "tok-mixed",
		TokenUsername:   submitted,
		TokenAlias:      submitted,
		TokenRemoteUser: "remote-mixed",
		ClientID:        "client1",
		Provider:        "generic",
	}
	sig := signOauthParams(app.cfg.Server.HashSeed, tp)

	form := url.Values{}
	form.Set(oauthParamAccessToken, tp.AccessToken)
	form.Set(oauthParamTokenUsername, tp.TokenUsername)
	form.Set(oauthParamTokenAlias, tp.TokenAlias)
	form.Set(oauthParamTokenRemoteUserID, tp.TokenRemoteUser)
	form.Set(oauthParamClientID, tp.ClientID)
	form.Set(oauthParamProvider, tp.Provider)
	form.Set(oauthParamHash, sig)
	form.Set(oauthParamUsername, submitted)

	req := httptest.NewRequest("POST", "/oauth/signup", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	if err := h.viewOauthSignup(app, w, req); err != nil {
		t.Fatalf("expected oauth signup to succeed, got error: %v", err)
	}

	assert.True(t, userExists(t, app, normalized), "user should be stored with a lowercased username")
	assert.False(t, userExists(t, app, submitted), "user should not be stored with the raw mixed-case username")

	var alias string
	err := app.db.QueryRow("SELECT alias FROM collections WHERE alias = ?", normalized).Scan(&alias)
	if err != nil {
		t.Fatalf("expected collection with lowercased alias %q: %v", normalized, err)
	}
	assert.Equal(t, normalized, alias, "collection alias must be lowercase to be reachable")
}
