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
	"net/url"
	"strings"

	"github.com/gorilla/csrf"
	"github.com/writefreely/writefreely/config"
)

// newCSRFProtector returns CSRF middleware configured for this instance. All
// CSRF-protected routes should use it rather than calling csrf.Protect
// directly.
//
// Since v1.7.3, gorilla/csrf assumes every request is served over HTTPS: it
// rejects http:// Origin and Referer headers unless the request has been
// marked with csrf.PlaintextHTTPRequest. We mark requests from the configured
// host rather than from how the request reached us, because TLS is often
// terminated at a reverse proxy, so WriteFreely sees plain HTTP even when the
// browser is on https://. Requests to a plain-HTTP instance (app.host is
// http://) or to a plain-HTTP Tor hidden service (server.hidden_host) are
// marked as plaintext. HTTPS instances get gorilla/csrf's defaults unchanged.
func newCSRFProtector(cfg *config.Config, key []byte) func(http.Handler) http.Handler {
	secureHost := strings.HasPrefix(cfg.App.Host, "https://")
	hiddenHost := plaintextHiddenHost(cfg.Server.HiddenHost)

	opts := []csrf.Option{
		csrf.Path("/"),
		// Match the session cookie (see InitSession). Browsers drop Secure
		// cookies set over plain HTTP everywhere except localhost, so
		// otherwise no form would work on a LAN or no-TLS instance.
		csrf.Secure(secureHost),
	}
	if !secureHost {
		// An http:// instance may still be reached over HTTPS through a
		// proxy, or with a rewritten Host header, so trust an Origin header
		// naming the configured host with either scheme.
		if u, err := url.Parse(cfg.App.Host); err == nil && u.Host != "" {
			opts = append(opts, csrf.TrustedOrigins([]string{u.Host}))
		}
	}
	protect := csrf.Protect(key, opts...)

	return func(next http.Handler) http.Handler {
		protected := protect(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.TLS == nil && (!secureHost || (hiddenHost != "" && strings.EqualFold(r.Host, hiddenHost))) {
				r = csrf.PlaintextHTTPRequest(r)
			}
			protected.ServeHTTP(w, r)
		})
	}
}

// plaintextHiddenHost returns the host of the configured Tor hidden service if
// it's served over plain HTTP, or "" if there isn't one or it uses HTTPS. A
// value without a scheme is assumed to be plain HTTP.
func plaintextHiddenHost(hiddenHost string) string {
	if hiddenHost == "" || strings.HasPrefix(hiddenHost, "https://") {
		return ""
	}
	if !strings.Contains(hiddenHost, "://") {
		hiddenHost = "http://" + hiddenHost
	}
	u, err := url.Parse(hiddenHost)
	if err != nil {
		return ""
	}
	return u.Host
}

// csrfProtectForm wraps a handler with CSRF protection but exempts JSON
// requests, which authenticate via Authorization header rather than
// session cookies and are therefore not CSRF-exploitable.
func csrfProtectForm(protect func(http.Handler) http.Handler, next http.Handler) http.Handler {
	protected := protect(next)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if IsJSON(r) {
			next.ServeHTTP(w, r)
			return
		}
		protected.ServeHTTP(w, r)
	})
}
