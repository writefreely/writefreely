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
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/writeas/impart"
	"github.com/writefreely/writefreely/config"
)

// enableTestEmail configures an email provider that fails to send immediately (nothing listens on the port), so
// confirmation emails are attempted without reaching the network.
func enableTestEmail(cfg *config.Config) {
	cfg.Email.Host = "127.0.0.1"
	cfg.Email.Port = 1
	cfg.Email.Username = "user"
	cfg.Email.Password = "pass"
	cfg.Email.Domain = "example.com"
}

func resetConfirmEmailLimiter() {
	confirmEmailLimiter.Lock()
	confirmEmailLimiter.sent = map[int64][]time.Time{}
	confirmEmailLimiter.Unlock()
}

func getTestUserStatus(t *testing.T, app *App, username string) UserStatus {
	t.Helper()
	u, err := app.db.GetUserForAuth(username)
	if err != nil {
		t.Fatalf("get user %q: %v", username, err)
	}
	return u.Status
}

func webSignup(t *testing.T, app *App, username, email string) (impart.HTTPError, *httptest.ResponseRecorder) {
	t.Helper()
	form := url.Values{}
	form.Set("alias", username)
	form.Set("pass", "sup3rSecret!")
	form.Set("email", email)

	req := httptest.NewRequest("POST", "/auth/signup", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	err := handleWebSignup(app, w, req)
	httpErr, ok := err.(impart.HTTPError)
	if !ok {
		t.Fatalf("expected impart.HTTPError, got %T (%v)", err, err)
	}
	return httpErr, w
}

func TestSignupRequireEmail(t *testing.T) {
	cases := []struct {
		name         string
		requireEmail bool
		emailEnabled bool
		email        string
		wantUser     bool
		wantStatus   UserStatus
		wantRedirect string
	}{
		{"optional, none given", false, true, "", true, UserActive, "/"},
		{"optional, given with email configured", false, true, "me@example.com", true, UserActive, "/"},
		{"optional, invalid address", false, false, "not-an-email", false, 0, ""},
		{"required, none given", true, true, "", false, 0, ""},
		{"required, without email configured", true, false, "me@example.com", true, UserActive, "/"},
		{"required, with email configured", true, true, "me@example.com", true, UserNeedsConfirmation, "/me/confirm"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resetConfirmEmailLimiter()
			app := newSignupTestApp(t)
			app.cfg.App.OpenRegistration = true
			app.cfg.App.RequireEmail = tc.requireEmail
			if tc.emailEnabled {
				enableTestEmail(app.cfg)
			}

			username := "signup-" + strings.ReplaceAll(strings.ReplaceAll(tc.name, " ", "-"), ",", "")
			httpErr, _ := webSignup(t, app, username, tc.email)

			assert.Equal(t, http.StatusFound, httpErr.Status)
			assert.Equal(t, tc.wantUser, userExists(t, app, username))
			if !tc.wantUser {
				return
			}
			assert.Equal(t, tc.wantRedirect, httpErr.Message)

			u, err := app.db.GetUserForAuth(username)
			if err != nil {
				t.Fatalf("get user: %v", err)
			}
			assert.Equal(t, tc.wantStatus, u.Status)

			token, err := app.db.GetEmailConfirmationToken(u.ID)
			assert.NoError(t, err)
			if tc.wantStatus == UserNeedsConfirmation {
				assert.NotEmpty(t, token, "a confirmation token should be issued")
			} else {
				assert.Empty(t, token, "no confirmation token should be issued")
			}
		})
	}
}

func TestAPISignupRequireEmail(t *testing.T) {
	app := newSignupTestApp(t)
	app.cfg.App.OpenRegistration = true
	app.cfg.App.RequireEmail = true

	req := httptest.NewRequest("POST", "/api/auth/signup", strings.NewReader(`{"alias": "apinoemail", "pass": "sup3rSecret!"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	_, err := signup(app, w, req)
	assert.Equal(t, ErrEmailRequired, err)
	assert.False(t, userExists(t, app, "apinoemail"))
}

func TestConfirmAccount(t *testing.T) {
	resetConfirmEmailLimiter()
	app := newSignupTestApp(t)
	app.cfg.App.OpenRegistration = true
	app.cfg.App.RequireEmail = true
	enableTestEmail(app.cfg)

	webSignup(t, app, "confirmme", "me@example.com")
	u, err := app.db.GetUserForAuth("confirmme")
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	assert.True(t, u.NeedsConfirmation())

	// Unconfirmed accounts can't publish, and are hidden like silenced ones
	status, err := app.db.GetUserStatus(u.ID)
	assert.NoError(t, err)
	assert.Equal(t, ErrUserNeedsConfirmation, status.restrictionErr())
	silenced, err := app.db.IsUserSilenced(u.ID)
	assert.NoError(t, err)
	assert.True(t, silenced, "unconfirmed accounts should be treated as silenced")

	token, err := app.db.GetEmailConfirmationToken(u.ID)
	if err != nil || token == "" {
		t.Fatalf("expected a confirmation token, got %q (%v)", token, err)
	}

	confirm := func(method, token string) error {
		form := url.Values{}
		form.Set("t", token)
		var req *http.Request
		if method == http.MethodPost {
			req = httptest.NewRequest(method, "/me/confirm", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		} else {
			req = httptest.NewRequest(method, "/me/confirm?"+form.Encode(), nil)
		}
		return handleConfirmAccount(app, &User{ID: u.ID, Username: u.Username, Status: u.Status}, httptest.NewRecorder(), req)
	}

	// Opening the link alone doesn't confirm anything
	assert.NoError(t, confirm(http.MethodGet, token))
	assert.Equal(t, UserStatus(UserNeedsConfirmation), getTestUserStatus(t, app, "confirmme"))

	// A bad token doesn't confirm anything
	assert.NoError(t, confirm(http.MethodPost, "not-the-right-token"))
	assert.Equal(t, UserStatus(UserNeedsConfirmation), getTestUserStatus(t, app, "confirmme"))

	// Clicking the confirmation button does
	err = confirm(http.MethodPost, token)
	assert.Equal(t, impart.HTTPError{http.StatusFound, "/me/settings"}, err)
	assert.Equal(t, UserStatus(UserActive), getTestUserStatus(t, app, "confirmme"))

	pending, err := app.db.GetEmailConfirmationToken(u.ID)
	assert.NoError(t, err)
	assert.Empty(t, pending, "token should be marked confirmed")
}

func TestConfirmAccountDoesntUnsilence(t *testing.T) {
	resetConfirmEmailLimiter()
	app := newSignupTestApp(t)
	app.cfg.App.OpenRegistration = true
	app.cfg.App.RequireEmail = true
	enableTestEmail(app.cfg)

	webSignup(t, app, "silencedconfirm", "me@example.com")
	u, _ := app.db.GetUserForAuth("silencedconfirm")
	token, _ := app.db.GetEmailConfirmationToken(u.ID)
	// An admin silences the account before it's confirmed
	if err := app.db.SetUserStatus(u.ID, UserSilenced); err != nil {
		t.Fatalf("silence user: %v", err)
	}

	form := url.Values{}
	form.Set("t", token)
	req := httptest.NewRequest(http.MethodPost, "/me/confirm", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	handleConfirmAccount(app, u, httptest.NewRecorder(), req)

	assert.Equal(t, UserStatus(UserSilenced), getTestUserStatus(t, app, "silencedconfirm"))
}

func TestEmailConfirmationTokenReissue(t *testing.T) {
	app := newSignupTestApp(t)

	first, err := app.db.CreateEmailConfirmationToken(42)
	assert.NoError(t, err)
	assert.NoError(t, app.db.ValidateEmailConfirmationToken(42, first))
	assert.Error(t, app.db.ValidateEmailConfirmationToken(43, first), "tokens only work for the user they were issued to")

	// A new token (e.g. after changing email address) invalidates the old one
	second, err := app.db.CreateEmailConfirmationToken(42)
	assert.NoError(t, err)
	assert.Error(t, app.db.ValidateEmailConfirmationToken(42, first))
	assert.NoError(t, app.db.ValidateEmailConfirmationToken(42, second))

	pending, err := app.db.GetEmailConfirmationToken(42)
	assert.NoError(t, err)
	assert.Equal(t, second, pending)
}

func TestConfirmEmailRateLimit(t *testing.T) {
	resetConfirmEmailLimiter()
	for i := 0; i < maxConfirmEmailsPerDay; i++ {
		assert.True(t, allowConfirmEmail(7), "send %d should be allowed", i+1)
	}
	assert.False(t, allowConfirmEmail(7), "sends past the daily limit should be refused")
	assert.True(t, allowConfirmEmail(8), "other users aren't affected")
}

func TestValidEmailAddress(t *testing.T) {
	valid := []string{"me@example.com", "first.last+tag@sub.example.co"}
	invalid := []string{"", "me", "me@", "@example.com", "Me <me@example.com>", "me@example.com\r\nBcc: x@example.com", "a@b@c"}
	for _, e := range valid {
		assert.True(t, validEmailAddress(e), "%q should be valid", e)
	}
	for _, e := range invalid {
		assert.False(t, validEmailAddress(e), "%q should be invalid", e)
	}
}

func TestOAuthSignupRequireEmail(t *testing.T) {
	cases := []struct {
		name         string
		tokenEmail   string
		email        string
		wantUser     bool
		wantStatus   UserStatus
		wantRedirect string
	}{
		{"no email", "", "", false, 0, ""},
		{"provider email kept", "me@example.com", "me@example.com", true, UserActive, "/"},
		{"provider email edited", "me@example.com", "other@example.com", true, UserNeedsConfirmation, "/me/confirm"},
		{"no provider email", "", "me@example.com", true, UserNeedsConfirmation, "/me/confirm"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resetConfirmEmailLimiter()
			app := newSignupTestApp(t)
			app.cfg.App.OpenRegistration = true
			app.cfg.App.RequireEmail = true
			enableTestEmail(app.cfg)
			h := newTestOauthHandler(app)

			username := "oauthemail-" + strings.ReplaceAll(tc.name, " ", "-")
			tp := oauthSignupPageParams{
				AccessToken:     "tok-" + username,
				TokenUsername:   username,
				TokenAlias:      username,
				TokenEmail:      tc.tokenEmail,
				TokenRemoteUser: "remote-" + username,
				ClientID:        "client1",
				Provider:        "generic",
			}

			form := url.Values{}
			form.Set(oauthParamAccessToken, tp.AccessToken)
			form.Set(oauthParamTokenUsername, tp.TokenUsername)
			form.Set(oauthParamTokenAlias, tp.TokenAlias)
			form.Set(oauthParamTokenEmail, tp.TokenEmail)
			form.Set(oauthParamTokenRemoteUserID, tp.TokenRemoteUser)
			form.Set(oauthParamClientID, tp.ClientID)
			form.Set(oauthParamProvider, tp.Provider)
			form.Set(oauthParamHash, signOauthParams(app.cfg.Server.HashSeed, tp))
			form.Set(oauthParamUsername, username)
			form.Set(oauthParamEmail, tc.email)

			req := httptest.NewRequest("POST", "/oauth/signup", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			w := httptest.NewRecorder()

			err := h.viewOauthSignup(app, w, req)
			assert.NoError(t, err)
			assert.Equal(t, tc.wantUser, userExists(t, app, username))
			if !tc.wantUser {
				assert.Contains(t, w.Body.String(), ErrEmailRequired.Message)
				return
			}
			assert.Equal(t, tc.wantStatus, getTestUserStatus(t, app, username))
			assert.Equal(t, tc.wantRedirect, w.Header().Get("Location"))
		})
	}
}

// TestTemplateRendering_UnconfirmedUser renders every page an unconfirmed user (and an admin looking at them) is
// likely to see, to catch template execution errors from the new fields.
func TestTemplateRendering_UnconfirmedUser(t *testing.T) {
	resetConfirmEmailLimiter()
	app, router := newTemplateTestApp(t, func(cfg *config.Config) {
		cfg.App.SingleUser = false
		cfg.App.OpenRegistration = true
		cfg.App.RequireEmail = true
		enableTestEmail(cfg)
	})
	admin, _, _ := createTemplateTestUser(t, app, "admin")
	adminCookie := loginCookie(t, app, admin)

	u := &User{Username: "newbie", HashedPass: []byte("x"), Status: UserNeedsConfirmation, Email: prepareUserEmail("newbie@example.com", app.keys.EmailKey)}
	if err := app.db.CreateUser(app.cfg, u, "", ""); err != nil {
		t.Fatalf("create user: %v", err)
	}
	token, err := app.db.CreateEmailConfirmationToken(u.ID)
	if err != nil {
		t.Fatalf("create token: %v", err)
	}
	cookie := loginCookie(t, app, u)

	cases := []struct {
		name       string
		path       string
		cookie     *http.Cookie
		wantStatus int
		wantBody   string
	}{
		{"confirm page", "/me/confirm", cookie, http.StatusOK, "newbie@example.com"},
		{"confirm link", "/me/confirm?t=" + token, cookie, http.StatusOK, "Confirm Email Address"},
		{"confirm bad link", "/me/confirm?t=nope", cookie, http.StatusOK, "Invalid link"},
		{"settings", "/me/settings", cookie, http.StatusOK, "confirm your email address"},
		{"posts", "/me/posts/", cookie, http.StatusOK, "confirm your email address"},
		{"blogs", "/me/c/", cookie, http.StatusOK, "confirm your email address"},
		{"edit blog", "/me/c/newbie", cookie, http.StatusOK, "confirm your email address"},
		{"stats", "/me/c/newbie/stats", cookie, http.StatusOK, "confirm your email address"},
		{"invites", "/me/invites", cookie, http.StatusOK, "confirm your email address"},
		{"pad", "/new", cookie, http.StatusOK, "var unconfirmed =  true ;"},
		{"own blog", "/newbie/", cookie, http.StatusOK, "confirm your email address"},
		{"blog hidden from others", "/newbie/", nil, http.StatusNotFound, ""},
		{"admin user list", "/admin/users", adminCookie, http.StatusOK, "Unconfirmed"},
		{"admin user", "/admin/user/newbie", adminCookie, http.StatusOK, "Activate"},
		{"admin settings", "/admin/settings", adminCookie, http.StatusOK, "require_email"},
		{"admin confirm page redirects", "/me/confirm", adminCookie, http.StatusFound, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var cookies []*http.Cookie
			if c.cookie != nil {
				cookies = []*http.Cookie{c.cookie}
			}
			rec := assertRendersCleanly(t, router, "GET", c.path, cookies, c.wantStatus)
			if c.wantBody != "" {
				assert.Contains(t, rec.Body.String(), c.wantBody)
			}
		})
	}

	// Signup forms require an email address
	app.cfg.App.Landing = ""
	for _, path := range []string{"/", "/signup"} {
		rec := assertRendersCleanly(t, router, "GET", path, nil, http.StatusOK)
		assert.Contains(t, rec.Body.String(), `tabindex="3" required`, "email should be required on %s", path)
	}
}
