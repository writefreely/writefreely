/*
 * Copyright © 2018-2019 A Bunch Tell LLC.
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
	"html/template"
	"net/http"
	"net/url"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/sessions"
	"github.com/writeas/impart"
	"github.com/writeas/web-core/log"
	"github.com/writeas/writefreely/config"
	"github.com/writeas/writefreely/page"
)

// UserLevel represents the required user level for accessing an endpoint
type UserLevel int

const (
	UserLevelNoneType         UserLevel = iota // user or not -- ignored
	UserLevelOptionalType                      // user or not -- object fetched if user
	UserLevelNoneRequiredType                  // non-user (required)
	UserLevelUserType                          // user (required)
)

func UserLevelNone(cfg *config.Config) UserLevel {
	return UserLevelNoneType
}

func UserLevelOptional(cfg *config.Config) UserLevel {
	return UserLevelOptionalType
}

func UserLevelNoneRequired(cfg *config.Config) UserLevel {
	return UserLevelNoneRequiredType
}

func UserLevelUser(cfg *config.Config) UserLevel {
	return UserLevelUserType
}

// UserLevelReader returns the permission level required for any route where
// users can read published content.
func UserLevelReader(cfg *config.Config) UserLevel {
	if cfg.App.Private {
		return UserLevelUserType
	}
	return UserLevelOptionalType
}

type (
	handlerFunc          func(app *App, w http.ResponseWriter, r *http.Request) error
	userHandlerFunc      func(app *App, u *User, w http.ResponseWriter, r *http.Request) error
	userApperHandlerFunc func(apper Apper, u *User, w http.ResponseWriter, r *http.Request) error
	dataHandlerFunc      func(app *App, w http.ResponseWriter, r *http.Request) ([]byte, string, error)
	authFunc             func(app *App, r *http.Request) (*User, error)
	UserLevelFunc        func(cfg *config.Config) UserLevel
)

type Handler struct {
	errors       *ErrorPages
	sessionStore *sessions.CookieStore
	app          Apper
}

// ErrorPages hold template HTML error pages for displaying errors to the user.
// In each, there should be a defined template named "base".
type ErrorPages struct {
	NotFound            *template.Template
	Gone                *template.Template
	InternalServerError *template.Template
	Blank               *template.Template
}

// NewHandler returns a new Handler instance, using the given StaticPage data,
// and saving alias to the application's CookieStore.
func NewHandler(apper Apper) *Handler {
	h := &Handler{
		errors: &ErrorPages{
			NotFound:            template.Must(template.New("").Parse("{{define \"base\"}}<html><head><title>404</title></head><body><p>Not found.</p></body></html>{{end}}")),
			Gone:                template.Must(template.New("").Parse("{{define \"base\"}}<html><head><title>410</title></head><body><p>Gone.</p></body></html>{{end}}")),
			InternalServerError: template.Must(template.New("").Parse("{{define \"base\"}}<html><head><title>500</title></head><body><p>Internal server error.</p></body></html>{{end}}")),
			Blank:               template.Must(template.New("").Parse("{{define \"base\"}}<html><head><title>{{.Title}}</title></head><body><p>{{.Content}}</p></body></html>{{end}}")),
		},
		sessionStore: apper.App().sessionStore,
		app:          apper,
	}

	return h
}

// NewWFHandler returns a new Handler instance, using WriteFreely template files.
// You MUST call writefreely.InitTemplates() before this.
func NewWFHandler(apper Apper) *Handler {
	h := NewHandler(apper)
	h.SetErrorPages(&ErrorPages{
		NotFound:            pages["404-general.tmpl"],
		Gone:                pages["410.tmpl"],
		InternalServerError: pages["500.tmpl"],
		Blank:               pages["blank.tmpl"],
	})
	return h
}

// SetErrorPages sets the given set of ErrorPages as templates for any errors
// that come up.
func (h *Handler) SetErrorPages(e *ErrorPages) {
	h.errors = e
}

// User handles requests made in the web application by the authenticated user.
// This provides user-friendly HTML pages and actions that work in the browser.
func (h *Handler) User(f userHandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.handleHTTPError(w, r, func() error {
			var status int
			start := time.Now()

			defer func() {
				if e := recover(); e != nil {
					log.Error("%s: %s", e, debug.Stack())
					h.errors.InternalServerError.ExecuteTemplate(w, "base", pageForReq(h.app.App(), r))
					status = http.StatusInternalServerError
				}

				log.Info(h.app.ReqLog(r, status, time.Since(start)))
			}()

			u := getUserSession(h.app.App(), r)
			if u == nil {
				err := ErrNotLoggedIn
				status = err.Status
				return err
			}

			err := f(h.app.App(), u, w, r)
			if err == nil {
				status = http.StatusOK
			} else if err, ok := err.(impart.HTTPError); ok {
				status = err.Status
			} else {
				status = http.StatusInternalServerError
			}

			return err
		}())
	}
}

// Admin handles requests on /admin routes
func (h *Handler) Admin(f userHandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.handleHTTPError(w, r, func() error {
			var status int
			start := time.Now()

			defer func() {
				if e := recover(); e != nil {
					log.Error("%s: %s", e, debug.Stack())
					h.errors.InternalServerError.ExecuteTemplate(w, "base", pageForReq(h.app.App(), r))
					status = http.StatusInternalServerError
				}

				log.Info(h.app.ReqLog(r, status, time.Since(start)))
			}()

			u := getUserSession(h.app.App(), r)
			if u == nil || !u.IsAdmin() {
				err := impart.HTTPError{http.StatusNotFound, ""}
				status = err.Status
				return err
			}

			err := f(h.app.App(), u, w, r)
			if err == nil {
				status = http.StatusOK
			} else if err, ok := err.(impart.HTTPError); ok {
				status = err.Status
			} else {
				status = http.StatusInternalServerError
			}

			return err
		}())
	}
}

// AdminApper handles requests on /admin routes that require an Apper.
func (h *Handler) AdminApper(f userApperHandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.handleHTTPError(w, r, func() error {
			var status int
			start := time.Now()

			defer func() {
				if e := recover(); e != nil {
					log.Error("%s: %s", e, debug.Stack())
					h.errors.InternalServerError.ExecuteTemplate(w, "base", pageForReq(h.app.App(), r))
					status = http.StatusInternalServerError
				}

				log.Info(h.app.ReqLog(r, status, time.Since(start)))
			}()

			u := getUserSession(h.app.App(), r)
			if u == nil || !u.IsAdmin() {
				err := impart.HTTPError{http.StatusNotFound, ""}
				status = err.Status
				return err
			}

			err := f(h.app, u, w, r)
			if err == nil {
				status = http.StatusOK
			} else if err, ok := err.(impart.HTTPError); ok {
				status = err.Status
			} else {
				status = http.StatusInternalServerError
			}

			return err
		}())
	}
}

func apiAuth(app *App, r *http.Request) (*User, error) {
	// Authorize user from Authorization header
	t := r.Header.Get("Authorization")
	if t == "" {
		return nil, ErrNoAccessToken
	}
	u := &User{ID: app.db.GetUserID(t)}
	if u.ID == -1 {
		return nil, ErrBadAccessToken
	}

	return u, nil
}

// optionaAPIAuth is used for endpoints that accept authenticated requests via
// Authorization header or cookie, unlike apiAuth. It returns a different err
// in the case where no Authorization header is present.
func optionalAPIAuth(app *App, r *http.Request) (*User, error) {
	// Authorize user from Authorization header
	t := r.Header.Get("Authorization")
	if t == "" {
		return nil, ErrNotLoggedIn
	}
	u := &User{ID: app.db.GetUserID(t)}
	if u.ID == -1 {
		return nil, ErrBadAccessToken
	}

	return u, nil
}

func webAuth(app *App, r *http.Request) (*User, error) {
	u := getUserSession(app, r)
	if u == nil {
		return nil, ErrNotLoggedIn
	}
	return u, nil
}

// UserAPI handles requests made in the API by the authenticated user.
// This provides user-friendly HTML pages and actions that work in the browser.
func (h *Handler) UserAPI(f userHandlerFunc) http.HandlerFunc {
	return h.UserAll(false, f, apiAuth)
}

func (h *Handler) UserAll(web bool, f userHandlerFunc, a authFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		handleFunc := func() error {
			var status int
			start := time.Now()

			defer func() {
				if e := recover(); e != nil {
					log.Error("%s: %s", e, debug.Stack())
					impart.WriteError(w, impart.HTTPError{http.StatusInternalServerError, "Something didn't work quite right."})
					status = 500
				}

				log.Info(h.app.ReqLog(r, status, time.Since(start)))
			}()

			u, err := a(h.app.App(), r)
			if err != nil {
				if err, ok := err.(impart.HTTPError); ok {
					status = err.Status
				} else {
					status = 500
				}
				return err
			}

			err = f(h.app.App(), u, w, r)
			if err == nil {
				status = 200
			} else if err, ok := err.(impart.HTTPError); ok {
				status = err.Status
			} else {
				status = 500
			}

			return err
		}

		if web {
			h.handleHTTPError(w, r, handleFunc())
		} else {
			h.handleError(w, r, handleFunc())
		}
	}
}

func (h *Handler) RedirectOnErr(f handlerFunc, loc string) handlerFunc {
	return func(app *App, w http.ResponseWriter, r *http.Request) error {
		err := f(app, w, r)
		if err != nil {
			if ie, ok := err.(impart.HTTPError); ok {
				// Override default redirect with returned error's, if it's a
				// redirect error.
				if ie.Status == http.StatusFound {
					return ie
				}
			}
			return impart.HTTPError{http.StatusFound, loc}
		}
		return nil
	}
}

func (h *Handler) Page(n string) http.HandlerFunc {
	return h.Web(func(app *App, w http.ResponseWriter, r *http.Request) error {
		t, ok := pages[n]
		if !ok {
			return impart.HTTPError{http.StatusNotFound, "Page not found."}
		}

		sp := pageForReq(app, r)

		err := t.ExecuteTemplate(w, "base", sp)
		if err != nil {
			log.Error("Unable to render page: %v", err)
		}
		return err
	}, UserLevelOptional)
}

func (h *Handler) WebErrors(f handlerFunc, ul UserLevelFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// TODO: factor out this logic shared with Web()
		h.handleHTTPError(w, r, func() error {
			var status int
			start := time.Now()

			defer func() {
				if e := recover(); e != nil {
					u := getUserSession(h.app.App(), r)
					username := "None"
					if u != nil {
						username = u.Username
					}
					log.Error("User: %s\n\n%s: %s", username, e, debug.Stack())
					h.errors.InternalServerError.ExecuteTemplate(w, "base", pageForReq(h.app.App(), r))
					status = 500
				}

				log.Info(h.app.ReqLog(r, status, time.Since(start)))
			}()

			var session *sessions.Session
			var err error
			if ul(h.app.App().cfg) != UserLevelNoneType {
				session, err = h.sessionStore.Get(r, cookieName)
				if err != nil && (ul(h.app.App().cfg) == UserLevelNoneRequiredType || ul(h.app.App().cfg) == UserLevelUserType) {
					// Cookie is required, but we can ignore this error
					log.Error("Handler: Unable to get session (for user permission %d); ignoring: %v", ul(h.app.App().cfg), err)
				}

				_, gotUser := session.Values[cookieUserVal].(*User)
				if ul(h.app.App().cfg) == UserLevelNoneRequiredType && gotUser {
					to := correctPageFromLoginAttempt(r)
					log.Info("Handler: Required NO user, but got one. Redirecting to %s", to)
					err := impart.HTTPError{http.StatusFound, to}
					status = err.Status
					return err
				} else if ul(h.app.App().cfg) == UserLevelUserType && !gotUser {
					log.Info("Handler: Required a user, but DIDN'T get one. Sending not logged in.")
					err := ErrNotLoggedIn
					status = err.Status
					return err
				}
			}

			// TODO: pass User object to function
			err = f(h.app.App(), w, r)
			if err == nil {
				status = 200
			} else if httpErr, ok := err.(impart.HTTPError); ok {
				status = httpErr.Status
				if status < 300 || status > 399 {
					addSessionFlash(h.app.App(), w, r, httpErr.Message, session)
					return impart.HTTPError{http.StatusFound, r.Referer()}
				}
			} else {
				e := fmt.Sprintf("[Web handler] 500: %v", err)
				if !strings.HasSuffix(e, "write: broken pipe") {
					log.Error(e)
				} else {
					log.Error(e)
				}
				log.Info("Web handler internal error render")
				h.errors.InternalServerError.ExecuteTemplate(w, "base", pageForReq(h.app.App(), r))
				status = 500
			}

			return err
		}())
	}
}

func (h *Handler) CollectionPostOrStatic(w http.ResponseWriter, r *http.Request) {
	if strings.Contains(r.URL.Path, ".") && !isRaw(r) {
		start := time.Now()
		status := 200
		defer func() {
			log.Info(h.app.ReqLog(r, status, time.Since(start)))
		}()

		// Serve static file
		h.app.App().shttp.ServeHTTP(w, r)
		return
	}

	h.Web(viewCollectionPost, UserLevelReader)(w, r)
}

// Web handles requests made in the web application. This provides user-
// friendly HTML pages and actions that work in the browser.
func (h *Handler) Web(f handlerFunc, ul UserLevelFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.handleHTTPError(w, r, func() error {
			var status int
			start := time.Now()

			defer func() {
				if e := recover(); e != nil {
					u := getUserSession(h.app.App(), r)
					username := "None"
					if u != nil {
						username = u.Username
					}
					log.Error("User: %s\n\n%s: %s", username, e, debug.Stack())
					log.Info("Web deferred internal error render")
					h.errors.InternalServerError.ExecuteTemplate(w, "base", pageForReq(h.app.App(), r))
					status = 500
				}

				log.Info(h.app.ReqLog(r, status, time.Since(start)))
			}()

			if ul(h.app.App().cfg) != UserLevelNoneType {
				session, err := h.sessionStore.Get(r, cookieName)
				if err != nil && (ul(h.app.App().cfg) == UserLevelNoneRequiredType || ul(h.app.App().cfg) == UserLevelUserType) {
					// Cookie is required, but we can ignore this error
					log.Error("Handler: Unable to get session (for user permission %d); ignoring: %v", ul(h.app.App().cfg), err)
				}

				_, gotUser := session.Values[cookieUserVal].(*User)
				if ul(h.app.App().cfg) == UserLevelNoneRequiredType && gotUser {
					to := correctPageFromLoginAttempt(r)
					log.Info("Handler: Required NO user, but got one. Redirecting to %s", to)
					err := impart.HTTPError{http.StatusFound, to}
					status = err.Status
					return err
				} else if ul(h.app.App().cfg) == UserLevelUserType && !gotUser {
					log.Info("Handler: Required a user, but DIDN'T get one. Sending not logged in.")
					err := ErrNotLoggedIn
					status = err.Status
					return err
				}
			}

			// TODO: pass User object to function
			err := f(h.app.App(), w, r)
			if err == nil {
				status = 200
			} else if httpErr, ok := err.(impart.HTTPError); ok {
				status = httpErr.Status
			} else {
				e := fmt.Sprintf("[Web handler] 500: %v", err)
				log.Error(e)
				log.Info("Web internal error render")
				h.errors.InternalServerError.ExecuteTemplate(w, "base", pageForReq(h.app.App(), r))
				status = 500
			}

			return err
		}())
	}
}

func (h *Handler) All(f handlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.handleError(w, r, func() error {
			// TODO: return correct "success" status
			status := 200
			start := time.Now()

			defer func() {
				if e := recover(); e != nil {
					log.Error("%s:\n%s", e, debug.Stack())
					impart.WriteError(w, impart.HTTPError{http.StatusInternalServerError, "Something didn't work quite right."})
					status = 500
				}

				log.Info(h.app.ReqLog(r, status, time.Since(start)))
			}()

			// TODO: do any needed authentication

			err := f(h.app.App(), w, r)
			if err != nil {
				if err, ok := err.(impart.HTTPError); ok {
					status = err.Status
				} else {
					status = 500
				}
			}

			return err
		}())
	}
}

func (h *Handler) AllReader(f handlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.handleError(w, r, func() error {
			status := 200
			start := time.Now()

			defer func() {
				if e := recover(); e != nil {
					log.Error("%s:\n%s", e, debug.Stack())
					impart.WriteError(w, impart.HTTPError{http.StatusInternalServerError, "Something didn't work quite right."})
					status = 500
				}

				log.Info(h.app.ReqLog(r, status, time.Since(start)))
			}()

			if h.app.App().cfg.App.Private {
				// This instance is private, so ensure it's being accessed by a valid user
				// Check if authenticated with an access token
				_, apiErr := optionalAPIAuth(h.app.App(), r)
				if apiErr != nil {
					if err, ok := apiErr.(impart.HTTPError); ok {
						status = err.Status
					} else {
						status = 500
					}

					if apiErr == ErrNotLoggedIn {
						// Fall back to web auth since there was no access token given
						_, err := webAuth(h.app.App(), r)
						if err != nil {
							if err, ok := apiErr.(impart.HTTPError); ok {
								status = err.Status
							} else {
								status = 500
							}
							return err
						}
					} else {
						return apiErr
					}
				}
			}

			err := f(h.app.App(), w, r)
			if err != nil {
				if err, ok := err.(impart.HTTPError); ok {
					status = err.Status
				} else {
					status = 500
				}
			}

			return err
		}())
	}
}

func (h *Handler) Download(f dataHandlerFunc, ul UserLevelFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.handleHTTPError(w, r, func() error {
			var status int
			start := time.Now()
			defer func() {
				if e := recover(); e != nil {
					log.Error("%s: %s", e, debug.Stack())
					h.errors.InternalServerError.ExecuteTemplate(w, "base", pageForReq(h.app.App(), r))
					status = 500
				}

				log.Info(h.app.ReqLog(r, status, time.Since(start)))
			}()

			data, filename, err := f(h.app.App(), w, r)
			if err != nil {
				if err, ok := err.(impart.HTTPError); ok {
					status = err.Status
				} else {
					status = 500
				}
				return err
			}

			ext := ".json"
			ct := "application/json"
			if strings.HasSuffix(r.URL.Path, ".csv") {
				ext = ".csv"
				ct = "text/csv"
			} else if strings.HasSuffix(r.URL.Path, ".zip") {
				ext = ".zip"
				ct = "application/zip"
			}
			w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s%s", filename, ext))
			w.Header().Set("Content-Type", ct)
			w.Header().Set("Content-Length", strconv.Itoa(len(data)))
			fmt.Fprint(w, string(data))

			status = 200
			return nil
		}())
	}
}

func (h *Handler) Redirect(url string, ul UserLevelFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.handleHTTPError(w, r, func() error {
			start := time.Now()

			var status int
			if ul(h.app.App().cfg) != UserLevelNoneType {
				session, err := h.sessionStore.Get(r, cookieName)
				if err != nil && (ul(h.app.App().cfg) == UserLevelNoneRequiredType || ul(h.app.App().cfg) == UserLevelUserType) {
					// Cookie is required, but we can ignore this error
					log.Error("Handler: Unable to get session (for user permission %d); ignoring: %v", ul(h.app.App().cfg), err)
				}

				_, gotUser := session.Values[cookieUserVal].(*User)
				if ul(h.app.App().cfg) == UserLevelNoneRequiredType && gotUser {
					to := correctPageFromLoginAttempt(r)
					log.Info("Handler: Required NO user, but got one. Redirecting to %s", to)
					err := impart.HTTPError{http.StatusFound, to}
					status = err.Status
					return err
				} else if ul(h.app.App().cfg) == UserLevelUserType && !gotUser {
					log.Info("Handler: Required a user, but DIDN'T get one. Sending not logged in.")
					err := ErrNotLoggedIn
					status = err.Status
					return err
				}
			}

			status = sendRedirect(w, http.StatusFound, url)

			log.Info(h.app.ReqLog(r, status, time.Since(start)))

			return nil
		}())
	}
}

func (h *Handler) handleHTTPError(w http.ResponseWriter, r *http.Request, err error) {
	if err == nil {
		return
	}

	if err, ok := err.(impart.HTTPError); ok {
		if err.Status >= 300 && err.Status < 400 {
			sendRedirect(w, err.Status, err.Message)
			return
		} else if err.Status == http.StatusUnauthorized {
			q := ""
			if r.URL.RawQuery != "" {
				q = url.QueryEscape("?" + r.URL.RawQuery)
			}
			sendRedirect(w, http.StatusFound, "/login?to="+r.URL.Path+q)
			return
		} else if err.Status == http.StatusGone {
			w.WriteHeader(err.Status)
			p := &struct {
				page.StaticPage
				Content *template.HTML
			}{
				StaticPage: pageForReq(h.app.App(), r),
			}
			if err.Message != "" {
				co := template.HTML(err.Message)
				p.Content = &co
			}
			h.errors.Gone.ExecuteTemplate(w, "base", p)
			return
		} else if err.Status == http.StatusNotFound {
			w.WriteHeader(err.Status)
			if strings.Contains(r.Header.Get("Accept"), "application/activity+json") {
				// This is a fediverse request; simply return the header
				return
			}
			h.errors.NotFound.ExecuteTemplate(w, "base", pageForReq(h.app.App(), r))
			return
		} else if err.Status == http.StatusInternalServerError {
			w.WriteHeader(err.Status)
			log.Info("handleHTTPErorr internal error render")
			h.errors.InternalServerError.ExecuteTemplate(w, "base", pageForReq(h.app.App(), r))
			return
		} else if err.Status == http.StatusAccepted {
			impart.WriteSuccess(w, "", err.Status)
			return
		} else {
			p := &struct {
				page.StaticPage
				Title   string
				Content template.HTML
			}{
				pageForReq(h.app.App(), r),
				fmt.Sprintf("Uh oh (%d)", err.Status),
				template.HTML(fmt.Sprintf("<p style=\"text-align: center\" class=\"introduction\">%s</p>", err.Message)),
			}
			h.errors.Blank.ExecuteTemplate(w, "base", p)
			return
		}
		impart.WriteError(w, err)
		return
	}

	impart.WriteError(w, impart.HTTPError{http.StatusInternalServerError, "This is an unhelpful error message for a miscellaneous internal error."})
}

func (h *Handler) handleError(w http.ResponseWriter, r *http.Request, err error) {
	if err == nil {
		return
	}

	if err, ok := err.(impart.HTTPError); ok {
		if err.Status >= 300 && err.Status < 400 {
			sendRedirect(w, err.Status, err.Message)
			return
		}

		//		if strings.Contains(r.Header.Get("Accept"), "text/html") {
		impart.WriteError(w, err)
		//		}
		return
	}

	if IsJSON(r.Header.Get("Content-Type")) {
		impart.WriteError(w, impart.HTTPError{http.StatusInternalServerError, "This is an unhelpful error message for a miscellaneous internal error."})
		return
	}
	h.errors.InternalServerError.ExecuteTemplate(w, "base", pageForReq(h.app.App(), r))
}

func correctPageFromLoginAttempt(r *http.Request) string {
	to := r.FormValue("to")
	if to == "" {
		to = "/"
	} else if !strings.HasPrefix(to, "/") {
		to = "/" + to
	}
	return to
}

func (h *Handler) LogHandlerFunc(f http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.handleHTTPError(w, r, func() error {
			status := 200
			start := time.Now()

			defer func() {
				if e := recover(); e != nil {
					log.Error("Handler.LogHandlerFunc\n\n%s: %s", e, debug.Stack())
					h.errors.InternalServerError.ExecuteTemplate(w, "base", pageForReq(h.app.App(), r))
					status = 500
				}

				// TODO: log actual status code returned
				log.Info(h.app.ReqLog(r, status, time.Since(start)))
			}()

			if h.app.App().cfg.App.Private {
				// This instance is private, so ensure it's being accessed by a valid user
				// Check if authenticated with an access token
				_, apiErr := optionalAPIAuth(h.app.App(), r)
				if apiErr != nil {
					if err, ok := apiErr.(impart.HTTPError); ok {
						status = err.Status
					} else {
						status = 500
					}

					if apiErr == ErrNotLoggedIn {
						// Fall back to web auth since there was no access token given
						_, err := webAuth(h.app.App(), r)
						if err != nil {
							if err, ok := apiErr.(impart.HTTPError); ok {
								status = err.Status
							} else {
								status = 500
							}
							return err
						}
					} else {
						return apiErr
					}
				}
			}

			f(w, r)

			return nil
		}())
	}
}

func sendRedirect(w http.ResponseWriter, code int, location string) int {
	w.Header().Set("Location", location)
	w.WriteHeader(code)
	return code
}
