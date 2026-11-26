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
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/csrf"
	"github.com/writefreely/writefreely/config"
)

var testCSRFKey = bytes.Repeat([]byte("k"), 32)

// csrfTestHandler echoes the request's CSRF token so tests can obtain a valid
// token/cookie pair from a GET, then replay it on a POST.
var csrfTestHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(csrf.Token(r)))
})

// getCSRFToken performs a GET through h and returns the masked token from the
// response body along with the CSRF cookie that was set.
func getCSRFToken(t *testing.T, h http.Handler, target string) (string, *http.Cookie) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", target, nil))
	res := rec.Result()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: status %d", target, res.StatusCode)
	}
	body, _ := io.ReadAll(res.Body)
	for _, c := range res.Cookies() {
		if c.Name == "_gorilla_csrf" {
			return string(body), c
		}
	}
	t.Fatalf("GET %s: no CSRF cookie set", target)
	return "", nil
}

func TestCSRFProtector(t *testing.T) {
	tests := []struct {
		name       string
		appHost    string
		hiddenHost string
		// target is the URL WriteFreely itself sees. An http:// target with
		// an https:// Origin simulates TLS terminated at a reverse proxy.
		target  string
		origin  string
		referer string
		noToken bool
		want    int
	}{
		// Plain HTTP instances (local dev, LAN, no TLS)
		{name: "http same origin", appHost: "http://localhost:8080", target: "http://localhost:8080/api/me/self", origin: "http://localhost:8080", want: http.StatusOK},
		{name: "http same-origin referer, no origin", appHost: "http://localhost:8080", target: "http://localhost:8080/api/me/self", referer: "http://localhost:8080/me/settings", want: http.StatusOK},
		{name: "http reached via LAN address", appHost: "http://localhost:8080", target: "http://192.168.1.5:8080/api/me/self", origin: "http://192.168.1.5:8080", want: http.StatusOK},
		{name: "http host behind TLS proxy", appHost: "http://blog.example.com", target: "http://blog.example.com/api/me/self", origin: "https://blog.example.com", want: http.StatusOK},
		{name: "http cross origin", appHost: "http://localhost:8080", target: "http://localhost:8080/api/me/self", origin: "http://evil.example", want: http.StatusForbidden},
		{name: "http missing token", appHost: "http://localhost:8080", target: "http://localhost:8080/api/me/self", origin: "http://localhost:8080", noToken: true, want: http.StatusForbidden},

		// HTTPS instances: behavior must match gorilla/csrf defaults
		{name: "https via TLS proxy", appHost: "https://blog.example.com", target: "http://blog.example.com/api/me/self", origin: "https://blog.example.com", want: http.StatusOK},
		{name: "https direct TLS", appHost: "https://blog.example.com", target: "https://blog.example.com/api/me/self", origin: "https://blog.example.com", want: http.StatusOK},
		{name: "https referer, no origin", appHost: "https://blog.example.com", target: "http://blog.example.com/api/me/self", referer: "https://blog.example.com/me/settings", want: http.StatusOK},
		{name: "https rejects plaintext origin", appHost: "https://blog.example.com", target: "http://blog.example.com/api/me/self", origin: "http://blog.example.com", want: http.StatusForbidden},
		{name: "https rejects plaintext referer", appHost: "https://blog.example.com", target: "http://blog.example.com/api/me/self", referer: "http://blog.example.com/me/settings", want: http.StatusForbidden},
		{name: "https requires referer", appHost: "https://blog.example.com", target: "http://blog.example.com/api/me/self", want: http.StatusForbidden},
		{name: "https cross origin", appHost: "https://blog.example.com", target: "http://blog.example.com/api/me/self", origin: "https://evil.example", want: http.StatusForbidden},

		// Tor hidden service alongside an HTTPS clearnet host
		{name: "onion same origin", appHost: "https://blog.example.com", hiddenHost: "http://abcdefgh.onion", target: "http://abcdefgh.onion/api/me/self", origin: "http://abcdefgh.onion", want: http.StatusOK},
		{name: "onion referer, no origin", appHost: "https://blog.example.com", hiddenHost: "http://abcdefgh.onion", target: "http://abcdefgh.onion/api/me/self", referer: "http://abcdefgh.onion/me/settings", want: http.StatusOK},
		{name: "onion doesn't relax clearnet", appHost: "https://blog.example.com", hiddenHost: "http://abcdefgh.onion", target: "http://blog.example.com/api/me/self", origin: "http://blog.example.com", want: http.StatusForbidden},
		{name: "onion cross origin", appHost: "https://blog.example.com", hiddenHost: "http://abcdefgh.onion", target: "http://abcdefgh.onion/api/me/self", origin: "http://evil.onion", want: http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.New()
			cfg.App.Host = tt.appHost
			cfg.Server.HiddenHost = tt.hiddenHost
			h := newCSRFProtector(cfg, testCSRFKey)(csrfTestHandler)

			token, cookie := getCSRFToken(t, h, tt.target)

			req := httptest.NewRequest("POST", tt.target, nil)
			req.AddCookie(cookie)
			if !tt.noToken {
				req.Header.Set("X-CSRF-Token", token)
			}
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			if tt.referer != "" {
				req.Header.Set("Referer", tt.referer)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if got := rec.Result().StatusCode; got != tt.want {
				t.Errorf("POST %s (Origin %q, Referer %q): got %d, want %d; body: %s",
					tt.target, tt.origin, tt.referer, got, tt.want, rec.Body.String())
			}
		})
	}
}

// The CSRF cookie must follow the same Secure rule as the session cookie:
// browsers drop Secure cookies set over plain HTTP (except on localhost), which
// would make every form on a LAN or no-TLS instance fail with "CSRF token
// invalid".
func TestCSRFCookieSecure(t *testing.T) {
	for host, wantSecure := range map[string]bool{
		"http://192.168.1.5:8080":  false,
		"https://blog.example.com": true,
	} {
		cfg := config.New()
		cfg.App.Host = host
		_, cookie := getCSRFToken(t, newCSRFProtector(cfg, testCSRFKey)(csrfTestHandler), host+"/me/settings")
		if cookie.Secure != wantSecure {
			t.Errorf("host %s: CSRF cookie Secure = %v, want %v", host, cookie.Secure, wantSecure)
		}
	}
}

func TestCSRFProtectFormSkipsJSON(t *testing.T) {
	cfg := config.New()
	cfg.App.Host = "https://blog.example.com"
	h := csrfProtectForm(newCSRFProtector(cfg, testCSRFKey), csrfTestHandler)

	// No token or cookie: JSON clients authenticate with an Authorization
	// header, so CSRF doesn't apply.
	req := httptest.NewRequest("POST", "/api/me/self", nil)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if got := rec.Result().StatusCode; got != http.StatusOK {
		t.Errorf("JSON POST: got %d, want %d", got, http.StatusOK)
	}

	// Form posts without a token are still rejected.
	req = httptest.NewRequest("POST", "/api/me/self", nil)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "https://blog.example.com")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if got := rec.Result().StatusCode; got != http.StatusForbidden {
		t.Errorf("form POST without token: got %d, want %d", got, http.StatusForbidden)
	}
}
