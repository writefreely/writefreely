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
	"github.com/writeas/writefreely/page"
)

type UserLevel int

const (
	UserLevelNone         UserLevel = iota // user or not -- ignored
	UserLevelOptional                      // user or not -- object fetched if user
	UserLevelNoneRequired                  // non-user (required)
	UserLevelUser                          // user (required)
)

type (
	handlerFunc     func(app *app, w http.ResponseWriter, r *http.Request) error
	userHandlerFunc func(app *app, u *User, w http.ResponseWriter, r *http.Request) error
	dataHandlerFunc func(app *app, w http.ResponseWriter, r *http.Request) ([]byte, string, error)
	authFunc        func(app *app, r *http.Request) (*User, error)
)

type Handler struct {
	errors       *ErrorPages
	sessionStore *sessions.CookieStore
	app          *app
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
func NewHandler(app *app) *Handler {
	h := &Handler{
		errors: &ErrorPages{
			NotFound:            template.Must(template.New("").Parse("{{define \"base\"}}<html><head><title>404</title></head><body><p>Not found.</p></body></html>{{end}}")),
			Gone:                template.Must(template.New("").Parse("{{define \"base\"}}<html><head><title>410</title></head><body><p>Gone.</p></body></html>{{end}}")),
			InternalServerError: template.Must(template.New("").Parse("{{define \"base\"}}<html><head><title>500</title></head><body><p>Internal server error.</p></body></html>{{end}}")),
			Blank:               template.Must(template.New("").Parse("{{define \"base\"}}<html><head><title>{{.Title}}</title></head><body><p>{{.Content}}</p></body></html>{{end}}")),
		},
		sessionStore: app.sessionStore,
		app:          app,
	}

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
					h.errors.InternalServerError.ExecuteTemplate(w, "base", pageForReq(h.app, r))
					status = http.StatusInternalServerError
				}

				log.Info("\"%s %s\" %d %s \"%s\"", r.Method, r.RequestURI, status, time.Since(start), r.UserAgent())
			}()

			u := getUserSession(h.app, r)
			if u == nil {
				err := ErrNotLoggedIn
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

// Admin handles requests on /admin routes
func (h *Handler) Admin(f userHandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.handleHTTPError(w, r, func() error {
			var status int
			start := time.Now()

			defer func() {
				if e := recover(); e != nil {
					log.Error("%s: %s", e, debug.Stack())
					h.errors.InternalServerError.ExecuteTemplate(w, "base", pageForReq(h.app, r))
					status = http.StatusInternalServerError
				}

				log.Info(fmt.Sprintf("\"%s %s\" %d %s \"%s\"", r.Method, r.RequestURI, status, time.Since(start), r.UserAgent()))
			}()

			u := getUserSession(h.app, r)
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

// UserAPI handles requests made in the API by the authenticated user.
// This provides user-friendly HTML pages and actions that work in the browser.
func (h *Handler) UserAPI(f userHandlerFunc) http.HandlerFunc {
	return h.UserAll(false, f, func(app *app, r *http.Request) (*User, error) {
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
	})
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

				log.Info("\"%s %s\" %d %s \"%s\"", r.Method, r.RequestURI, status, time.Since(start), r.UserAgent())
			}()

			u, err := a(h.app, r)
			if err != nil {
				if err, ok := err.(impart.HTTPError); ok {
					status = err.Status
				} else {
					status = 500
				}
				return err
			}

			err = f(h.app, u, w, r)
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
	return func(app *app, w http.ResponseWriter, r *http.Request) error {
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
	return h.Web(func(app *app, w http.ResponseWriter, r *http.Request) error {
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

func (h *Handler) WebErrors(f handlerFunc, ul UserLevel) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// TODO: factor out this logic shared with Web()
		h.handleHTTPError(w, r, func() error {
			var status int
			start := time.Now()

			defer func() {
				if e := recover(); e != nil {
					u := getUserSession(h.app, r)
					username := "None"
					if u != nil {
						username = u.Username
					}
					log.Error("User: %s\n\n%s: %s", username, e, debug.Stack())
					h.errors.InternalServerError.ExecuteTemplate(w, "base", pageForReq(h.app, r))
					status = 500
				}

				log.Info("\"%s %s\" %d %s \"%s\"", r.Method, r.RequestURI, status, time.Since(start), r.UserAgent())
			}()

			var session *sessions.Session
			var err error
			if ul != UserLevelNone {
				session, err = h.sessionStore.Get(r, cookieName)
				if err != nil && (ul == UserLevelNoneRequired || ul == UserLevelUser) {
					// Cookie is required, but we can ignore this error
					log.Error("Handler: Unable to get session (for user permission %d); ignoring: %v", ul, err)
				}

				_, gotUser := session.Values[cookieUserVal].(*User)
				if ul == UserLevelNoneRequired && gotUser {
					to := correctPageFromLoginAttempt(r)
					log.Info("Handler: Required NO user, but got one. Redirecting to %s", to)
					err := impart.HTTPError{http.StatusFound, to}
					status = err.Status
					return err
				} else if ul == UserLevelUser && !gotUser {
					log.Info("Handler: Required a user, but DIDN'T get one. Sending not logged in.")
					err := ErrNotLoggedIn
					status = err.Status
					return err
				}
			}

			// TODO: pass User object to function
			err = f(h.app, w, r)
			if err == nil {
				status = 200
			} else if httpErr, ok := err.(impart.HTTPError); ok {
				status = httpErr.Status
				if status < 300 || status > 399 {
					addSessionFlash(h.app, w, r, httpErr.Message, session)
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
				h.errors.InternalServerError.ExecuteTemplate(w, "base", pageForReq(h.app, r))
				status = 500
			}

			return err
		}())
	}
}

// Web handles requests made in the web application. This provides user-
// friendly HTML pages and actions that work in the browser.
func (h *Handler) Web(f handlerFunc, ul UserLevel) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.handleHTTPError(w, r, func() error {
			var status int
			start := time.Now()

			defer func() {
				if e := recover(); e != nil {
					u := getUserSession(h.app, r)
					username := "None"
					if u != nil {
						username = u.Username
					}
					log.Error("User: %s\n\n%s: %s", username, e, debug.Stack())
					log.Info("Web deferred internal error render")
					h.errors.InternalServerError.ExecuteTemplate(w, "base", pageForReq(h.app, r))
					status = 500
				}

				log.Info("\"%s %s\" %d %s \"%s\"", r.Method, r.RequestURI, status, time.Since(start), r.UserAgent())
			}()

			if ul != UserLevelNone {
				session, err := h.sessionStore.Get(r, cookieName)
				if err != nil && (ul == UserLevelNoneRequired || ul == UserLevelUser) {
					// Cookie is required, but we can ignore this error
					log.Error("Handler: Unable to get session (for user permission %d); ignoring: %v", ul, err)
				}

				_, gotUser := session.Values[cookieUserVal].(*User)
				if ul == UserLevelNoneRequired && gotUser {
					to := correctPageFromLoginAttempt(r)
					log.Info("Handler: Required NO user, but got one. Redirecting to %s", to)
					err := impart.HTTPError{http.StatusFound, to}
					status = err.Status
					return err
				} else if ul == UserLevelUser && !gotUser {
					log.Info("Handler: Required a user, but DIDN'T get one. Sending not logged in.")
					err := ErrNotLoggedIn
					status = err.Status
					return err
				}
			}

			// TODO: pass User object to function
			err := f(h.app, w, r)
			if err == nil {
				status = 200
			} else if httpErr, ok := err.(impart.HTTPError); ok {
				status = httpErr.Status
			} else {
				e := fmt.Sprintf("[Web handler] 500: %v", err)
				log.Error(e)
				log.Info("Web internal error render")
				h.errors.InternalServerError.ExecuteTemplate(w, "base", pageForReq(h.app, r))
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

				log.Info("\"%s %s\" %d %s \"%s\"", r.Method, r.RequestURI, status, time.Since(start), r.UserAgent())
			}()

			// TODO: do any needed authentication

			err := f(h.app, w, r)
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

func (h *Handler) Download(f dataHandlerFunc, ul UserLevel) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.handleHTTPError(w, r, func() error {
			var status int
			start := time.Now()
			defer func() {
				if e := recover(); e != nil {
					log.Error("%s: %s", e, debug.Stack())
					h.errors.InternalServerError.ExecuteTemplate(w, "base", pageForReq(h.app, r))
					status = 500
				}

				log.Info("\"%s %s\" %d %s \"%s\"", r.Method, r.RequestURI, status, time.Since(start), r.UserAgent())
			}()

			data, filename, err := f(h.app, w, r)
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

func (h *Handler) Redirect(url string, ul UserLevel) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.handleHTTPError(w, r, func() error {
			start := time.Now()

			var status int
			if ul != UserLevelNone {
				session, err := h.sessionStore.Get(r, cookieName)
				if err != nil && (ul == UserLevelNoneRequired || ul == UserLevelUser) {
					// Cookie is required, but we can ignore this error
					log.Error("Handler: Unable to get session (for user permission %d); ignoring: %v", ul, err)
				}

				_, gotUser := session.Values[cookieUserVal].(*User)
				if ul == UserLevelNoneRequired && gotUser {
					to := correctPageFromLoginAttempt(r)
					log.Info("Handler: Required NO user, but got one. Redirecting to %s", to)
					err := impart.HTTPError{http.StatusFound, to}
					status = err.Status
					return err
				} else if ul == UserLevelUser && !gotUser {
					log.Info("Handler: Required a user, but DIDN'T get one. Sending not logged in.")
					err := ErrNotLoggedIn
					status = err.Status
					return err
				}
			}

			status = sendRedirect(w, http.StatusFound, url)

			log.Info("\"%s %s\" %d %s \"%s\"", r.Method, r.RequestURI, status, time.Since(start), r.UserAgent())

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
			p := &struct {
				page.StaticPage
				Content *template.HTML
			}{
				StaticPage: pageForReq(h.app, r),
			}
			if err.Message != "" {
				co := template.HTML(err.Message)
				p.Content = &co
			}
			h.errors.Gone.ExecuteTemplate(w, "base", p)
			return
		} else if err.Status == http.StatusNotFound {
			h.errors.NotFound.ExecuteTemplate(w, "base", pageForReq(h.app, r))
			return
		} else if err.Status == http.StatusInternalServerError {
			log.Info("handleHTTPErorr internal error render")
			h.errors.InternalServerError.ExecuteTemplate(w, "base", pageForReq(h.app, r))
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
				pageForReq(h.app, r),
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
	h.errors.InternalServerError.ExecuteTemplate(w, "base", pageForReq(h.app, r))
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
					h.errors.InternalServerError.ExecuteTemplate(w, "base", pageForReq(h.app, r))
					status = 500
				}

				// TODO: log actual status code returned
				log.Info("\"%s %s\" %d %s \"%s\"", r.Method, r.RequestURI, status, time.Since(start), r.UserAgent())
			}()

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
