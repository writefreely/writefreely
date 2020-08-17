/*
 * Copyright © 2018-2020 A Bunch Tell LLC.
 *
 * This file is part of WriteFreely.
 *
 * WriteFreely is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License, included
 * in the LICENSE file in this source code package.
 */

package writefreely

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/guregu/null/zero"
	"github.com/writeas/impart"
	"github.com/writeas/web-core/auth"
	"github.com/writeas/web-core/data"
	"github.com/writeas/web-core/log"

	"github.com/writeas/writefreely/author"
	"github.com/writeas/writefreely/config"
	"github.com/writeas/writefreely/page"
)

type (
	userSettings struct {
		Username string `schema:"username" json:"username"`
		Email    string `schema:"email" json:"email"`
		NewPass  string `schema:"new-pass" json:"new_pass"`
		OldPass  string `schema:"current-pass" json:"current_pass"`
		IsLogOut bool   `schema:"logout" json:"logout"`
	}

	UserPage struct {
		page.StaticPage

		PageTitle string
		Separator template.HTML
		IsAdmin   bool
		CanInvite bool
	}
)

func NewUserPage(app *App, r *http.Request, u *User, title string, flashes []string) *UserPage {
	up := &UserPage{
		StaticPage: pageForReq(app, r),
		PageTitle:  title,
	}
	up.Username = u.Username
	up.Flashes = flashes
	up.Path = r.URL.Path
	up.IsAdmin = u.IsAdmin()
	up.CanInvite = canUserInvite(app.cfg, up.IsAdmin)
	return up
}

func canUserInvite(cfg *config.Config, isAdmin bool) bool {
	return cfg.App.UserInvites != "" &&
		(isAdmin || cfg.App.UserInvites != "admin")
}

func (up *UserPage) SetMessaging(u *User) {
	// up.NeedsAuth = app.db.DoesUserNeedAuth(u.ID)
}

const (
	loginAttemptExpiration = 3 * time.Second
)

var actuallyUsernameReg = regexp.MustCompile("username is actually ([a-z0-9\\-]+)\\. Please try that, instead")

func apiSignup(app *App, w http.ResponseWriter, r *http.Request) error {
	_, err := signup(app, w, r)
	return err
}

func signup(app *App, w http.ResponseWriter, r *http.Request) (*AuthUser, error) {
	if app.cfg.App.DisablePasswordAuth {
		err := ErrDisabledPasswordAuth
		return nil, err
	}

	reqJSON := IsJSON(r)

	// Get params
	var ur userRegistration
	if reqJSON {
		decoder := json.NewDecoder(r.Body)
		err := decoder.Decode(&ur)
		if err != nil {
			log.Error("Couldn't parse signup JSON request: %v\n", err)
			return nil, ErrBadJSON
		}
	} else {
		// Check if user is already logged in
		u := getUserSession(app, r)
		if u != nil {
			return &AuthUser{User: u}, nil
		}

		err := r.ParseForm()
		if err != nil {
			log.Error("Couldn't parse signup form request: %v\n", err)
			return nil, ErrBadFormData
		}

		err = app.formDecoder.Decode(&ur, r.PostForm)
		if err != nil {
			log.Error("Couldn't decode signup form request: %v\n", err)
			return nil, ErrBadFormData
		}
	}

	return signupWithRegistration(app, ur, w, r)
}

func signupWithRegistration(app *App, signup userRegistration, w http.ResponseWriter, r *http.Request) (*AuthUser, error) {
	reqJSON := IsJSON(r)

	// Validate required params (alias)
	if signup.Alias == "" {
		return nil, impart.HTTPError{http.StatusBadRequest, "A username is required."}
	}
	if signup.Pass == "" {
		return nil, impart.HTTPError{http.StatusBadRequest, "A password is required."}
	}
	var desiredUsername string
	if signup.Normalize {
		// With this option we simply conform the username to what we expect
		// without complaining. Since they might've done something funny, like
		// enter: write.as/Way Out There, we'll use their raw input for the new
		// collection name and sanitize for the slug / username.
		desiredUsername = signup.Alias
		signup.Alias = getSlug(signup.Alias, "")
	}
	if !author.IsValidUsername(app.cfg, signup.Alias) {
		// Ensure the username is syntactically correct.
		return nil, impart.HTTPError{http.StatusPreconditionFailed, "Username is reserved or isn't valid. It must be at least 3 characters long, and can only include letters, numbers, and hyphens."}
	}

	// Handle empty optional params
	// TODO: remove this var
	createdWithPass := true
	hashedPass, err := auth.HashPass([]byte(signup.Pass))
	if err != nil {
		return nil, impart.HTTPError{http.StatusInternalServerError, "Could not create password hash."}
	}

	// Create struct to insert
	u := &User{
		Username:   signup.Alias,
		HashedPass: hashedPass,
		HasPass:    createdWithPass,
		Email:      prepareUserEmail(signup.Email, app.keys.EmailKey),
		Created:    time.Now().Truncate(time.Second).UTC(),
	}

	// Create actual user
	if err := app.db.CreateUser(app.cfg, u, desiredUsername); err != nil {
		return nil, err
	}

	// Log invite if needed
	if signup.InviteCode != "" {
		err = app.db.CreateInvitedUser(signup.InviteCode, u.ID)
		if err != nil {
			return nil, err
		}
	}

	// Add back unencrypted data for response
	if signup.Email != "" {
		u.Email.String = signup.Email
	}

	resUser := &AuthUser{
		User: u,
	}
	if !createdWithPass {
		resUser.Password = signup.Pass
	}
	title := signup.Alias
	if signup.Normalize {
		title = desiredUsername
	}
	resUser.Collections = &[]Collection{
		{
			Alias: signup.Alias,
			Title: title,
		},
	}

	var token string
	if reqJSON && !signup.Web {
		token, err = app.db.GetAccessToken(u.ID)
		if err != nil {
			return nil, impart.HTTPError{http.StatusInternalServerError, "Could not create access token. Try re-authenticating."}
		}
		resUser.AccessToken = token
	} else {
		session, err := app.sessionStore.Get(r, cookieName)
		if err != nil {
			// The cookie should still save, even if there's an error.
			// Source: https://github.com/gorilla/sessions/issues/16#issuecomment-143642144
			log.Error("Session: %v; ignoring", err)
		}
		session.Values[cookieUserVal] = resUser.User.Cookie()
		err = session.Save(r, w)
		if err != nil {
			log.Error("Couldn't save session: %v", err)
			return nil, err
		}
	}
	if reqJSON {
		return resUser, impart.WriteSuccess(w, resUser, http.StatusCreated)
	}

	return resUser, nil
}

func viewLogout(app *App, w http.ResponseWriter, r *http.Request) error {
	session, err := app.sessionStore.Get(r, cookieName)
	if err != nil {
		return ErrInternalCookieSession
	}

	// Ensure user has an email or password set before they go, so they don't
	// lose access to their account.
	val := session.Values[cookieUserVal]
	var u = &User{}
	var ok bool
	if u, ok = val.(*User); !ok {
		log.Error("Error casting user object on logout. Vals: %+v Resetting cookie.", session.Values)

		err = session.Save(r, w)
		if err != nil {
			log.Error("Couldn't save session on logout: %v", err)
			return impart.HTTPError{http.StatusInternalServerError, "Unable to save cookie session."}
		}

		return impart.HTTPError{http.StatusFound, "/"}
	}

	u, err = app.db.GetUserByID(u.ID)
	if err != nil && err != ErrUserNotFound {
		return impart.HTTPError{http.StatusInternalServerError, "Unable to fetch user information."}
	}

	session.Options.MaxAge = -1

	err = session.Save(r, w)
	if err != nil {
		log.Error("Couldn't save session on logout: %v", err)
		return impart.HTTPError{http.StatusInternalServerError, "Unable to save cookie session."}
	}

	return impart.HTTPError{http.StatusFound, "/"}
}

func handleAPILogout(app *App, w http.ResponseWriter, r *http.Request) error {
	accessToken := r.Header.Get("Authorization")
	if accessToken == "" {
		return ErrNoAccessToken
	}
	t := auth.GetToken(accessToken)
	if len(t) == 0 {
		return ErrNoAccessToken
	}
	err := app.db.DeleteToken(t)
	if err != nil {
		return err
	}
	return impart.HTTPError{Status: http.StatusNoContent}
}

func viewLogin(app *App, w http.ResponseWriter, r *http.Request) error {
	var earlyError string
	oneTimeToken := r.FormValue("with")
	if oneTimeToken != "" {
		log.Info("Calling login with one-time token.")
		err := login(app, w, r)
		if err != nil {
			log.Info("Received error: %v", err)
			earlyError = fmt.Sprintf("%s", err)
		}
	}

	session, err := app.sessionStore.Get(r, cookieName)
	if err != nil {
		// Ignore this
		log.Error("Unable to get session; ignoring: %v", err)
	}

	p := &struct {
		page.StaticPage
		To                      string
		Message                 template.HTML
		Flashes                 []template.HTML
		LoginUsername           string
		OauthSlack              bool
		OauthWriteAs            bool
		OauthGitlab             bool
		GitlabDisplayName       string
		OauthGeneric            bool
		OauthGenericDisplayName string
		OauthGitea              bool
		GiteaDisplayName        string
	}{
		pageForReq(app, r),
		r.FormValue("to"),
		template.HTML(""),
		[]template.HTML{},
		getTempInfo(app, "login-user", r, w),
		app.Config().SlackOauth.ClientID != "",
		app.Config().WriteAsOauth.ClientID != "",
		app.Config().GitlabOauth.ClientID != "",
		config.OrDefaultString(app.Config().GenericOauth.DisplayName, genericOauthDisplayName),
		app.Config().GenericOauth.ClientID != "",
		config.OrDefaultString(app.Config().GitlabOauth.DisplayName, gitlabDisplayName),
		app.Config().GiteaOauth.ClientID != "",
		config.OrDefaultString(app.Config().GiteaOauth.DisplayName, giteaDisplayName),
	}

	if earlyError != "" {
		p.Flashes = append(p.Flashes, template.HTML(earlyError))
	}

	// Display any error messages
	flashes, _ := getSessionFlashes(app, w, r, session)
	for _, flash := range flashes {
		p.Flashes = append(p.Flashes, template.HTML(flash))
	}
	err = pages["login.tmpl"].ExecuteTemplate(w, "base", p)
	if err != nil {
		log.Error("Unable to render login: %v", err)
		return err
	}
	return nil
}

func webLogin(app *App, w http.ResponseWriter, r *http.Request) error {
	err := login(app, w, r)
	if err != nil {
		username := r.FormValue("alias")
		// Login request was unsuccessful; save the error in the session and redirect them
		if err, ok := err.(impart.HTTPError); ok {
			session, _ := app.sessionStore.Get(r, cookieName)
			if session != nil {
				session.AddFlash(err.Message)
				session.Save(r, w)
			}

			if m := actuallyUsernameReg.FindStringSubmatch(err.Message); len(m) > 0 {
				// Retain fixed username recommendation for the login form
				username = m[1]
			}
		}

		// Pass along certain information
		saveTempInfo(app, "login-user", username, r, w)

		// Retain post-login URL if one was given
		redirectTo := "/login"
		postLoginRedirect := r.FormValue("to")
		if postLoginRedirect != "" {
			redirectTo += "?to=" + postLoginRedirect
		}

		log.Error("Unable to login: %v", err)
		return impart.HTTPError{http.StatusTemporaryRedirect, redirectTo}
	}

	return nil
}

var loginAttemptUsers = sync.Map{}

func login(app *App, w http.ResponseWriter, r *http.Request) error {
	reqJSON := IsJSON(r)
	oneTimeToken := r.FormValue("with")
	verbose := r.FormValue("all") == "true" || r.FormValue("verbose") == "1" || r.FormValue("verbose") == "true" || (reqJSON && oneTimeToken != "")

	redirectTo := r.FormValue("to")
	if redirectTo == "" {
		if app.cfg.App.SingleUser {
			redirectTo = "/me/new"
		} else {
			redirectTo = "/"
		}
	}

	var u *User
	var err error
	var signin userCredentials

	if app.cfg.App.DisablePasswordAuth {
		err := ErrDisabledPasswordAuth
		return err
	}

	// Log in with one-time token if one is given
	if oneTimeToken != "" {
		log.Info("Login: Logging user in via token.")
		userID := app.db.GetUserID(oneTimeToken)
		if userID == -1 {
			log.Error("Login: Got user -1 from token")
			err := ErrBadAccessToken
			err.Message = "Expired or invalid login code."
			return err
		}
		log.Info("Login: Found user %d.", userID)

		u, err = app.db.GetUserByID(userID)
		if err != nil {
			log.Error("Unable to fetch user on one-time token login: %v", err)
			return impart.HTTPError{http.StatusInternalServerError, "There was an error retrieving the user you want."}
		}
		log.Info("Login: Got user via token")
	} else {
		// Get params
		if reqJSON {
			decoder := json.NewDecoder(r.Body)
			err := decoder.Decode(&signin)
			if err != nil {
				log.Error("Couldn't parse signin JSON request: %v\n", err)
				return ErrBadJSON
			}
		} else {
			err := r.ParseForm()
			if err != nil {
				log.Error("Couldn't parse signin form request: %v\n", err)
				return ErrBadFormData
			}

			err = app.formDecoder.Decode(&signin, r.PostForm)
			if err != nil {
				log.Error("Couldn't decode signin form request: %v\n", err)
				return ErrBadFormData
			}
		}

		log.Info("Login: Attempting login for '%s'", signin.Alias)

		// Validate required params (all)
		if signin.Alias == "" {
			msg := "Parameter `alias` required."
			if signin.Web {
				msg = "A username is required."
			}
			return impart.HTTPError{http.StatusBadRequest, msg}
		}
		if !signin.EmailLogin && signin.Pass == "" {
			msg := "Parameter `pass` required."
			if signin.Web {
				msg = "A password is required."
			}
			return impart.HTTPError{http.StatusBadRequest, msg}
		}

		// Prevent excessive login attempts on the same account
		// Skip this check in dev environment
		if !app.cfg.Server.Dev {
			now := time.Now()
			attemptExp, att := loginAttemptUsers.LoadOrStore(signin.Alias, now.Add(loginAttemptExpiration))
			if att {
				if attemptExpTime, ok := attemptExp.(time.Time); ok {
					if attemptExpTime.After(now) {
						// This user attempted previously, and the period hasn't expired yet
						return impart.HTTPError{http.StatusTooManyRequests, "You're doing that too much."}
					} else {
						// This user attempted previously, but the time expired; free up space
						loginAttemptUsers.Delete(signin.Alias)
					}
				} else {
					log.Error("Unable to cast expiration to time")
				}
			}
		}

		// Retrieve password
		u, err = app.db.GetUserForAuth(signin.Alias)
		if err != nil {
			log.Info("Unable to getUserForAuth on %s: %v", signin.Alias, err)
			if strings.IndexAny(signin.Alias, "@") > 0 {
				log.Info("Suggesting: %s", ErrUserNotFoundEmail.Message)
				return ErrUserNotFoundEmail
			}
			return err
		}
		// Authenticate
		if u.Email.String == "" {
			// User has no email set, so check if they haven't added a password, either,
			// so we can return a more helpful error message.
			if hasPass, _ := app.db.IsUserPassSet(u.ID); !hasPass {
				log.Info("Tried logging in to %s, but no password or email.", signin.Alias)
				return impart.HTTPError{http.StatusPreconditionFailed, "This user never added a password or email address. Please contact us for help."}
			}
		}
		if len(u.HashedPass) == 0 {
			return impart.HTTPError{http.StatusUnauthorized, "This user never set a password. Perhaps try logging in via OAuth?"}
		}
		if !auth.Authenticated(u.HashedPass, []byte(signin.Pass)) {
			return impart.HTTPError{http.StatusUnauthorized, "Incorrect password."}
		}
	}

	if reqJSON && !signin.Web {
		var token string
		if r.Header.Get("User-Agent") == "" {
			// Get last created token when User-Agent is empty
			token = app.db.FetchLastAccessToken(u.ID)
			if token == "" {
				token, err = app.db.GetAccessToken(u.ID)
			}
		} else {
			token, err = app.db.GetAccessToken(u.ID)
		}
		if err != nil {
			log.Error("Login: Unable to create access token: %v", err)
			return impart.HTTPError{http.StatusInternalServerError, "Could not create access token. Try re-authenticating."}
		}
		resUser := getVerboseAuthUser(app, token, u, verbose)
		return impart.WriteSuccess(w, resUser, http.StatusOK)
	}

	session, err := app.sessionStore.Get(r, cookieName)
	if err != nil {
		// The cookie should still save, even if there's an error.
		log.Error("Login: Session: %v; ignoring", err)
	}

	// Remove unwanted data
	session.Values[cookieUserVal] = u.Cookie()
	err = session.Save(r, w)
	if err != nil {
		log.Error("Login: Couldn't save session: %v", err)
		// TODO: return error
	}

	// Send success
	if reqJSON {
		return impart.WriteSuccess(w, &AuthUser{User: u}, http.StatusOK)
	}
	log.Info("Login: Redirecting to %s", redirectTo)
	w.Header().Set("Location", redirectTo)
	w.WriteHeader(http.StatusFound)
	return nil
}

func getVerboseAuthUser(app *App, token string, u *User, verbose bool) *AuthUser {
	resUser := &AuthUser{
		AccessToken: token,
		User:        u,
	}

	// Fetch verbose user data if requested
	if verbose {
		posts, err := app.db.GetUserPosts(u)
		if err != nil {
			log.Error("Login: Unable to get user posts: %v", err)
		}
		colls, err := app.db.GetCollections(u, app.cfg.App.Host)
		if err != nil {
			log.Error("Login: Unable to get user collections: %v", err)
		}
		passIsSet, err := app.db.IsUserPassSet(u.ID)
		if err != nil {
			// TODO: correct error meesage
			log.Error("Login: Unable to get user collections: %v", err)
		}

		resUser.Posts = posts
		resUser.Collections = colls
		resUser.User.HasPass = passIsSet
	}
	return resUser
}

func viewExportOptions(app *App, u *User, w http.ResponseWriter, r *http.Request) error {
	// Fetch extra user data
	p := NewUserPage(app, r, u, "Export", nil)

	showUserPage(w, "export", p)
	return nil
}

func viewExportPosts(app *App, w http.ResponseWriter, r *http.Request) ([]byte, string, error) {
	var filename string
	var u = &User{}
	reqJSON := IsJSON(r)
	if reqJSON {
		// Use given Authorization header
		accessToken := r.Header.Get("Authorization")
		if accessToken == "" {
			return nil, filename, ErrNoAccessToken
		}

		userID := app.db.GetUserID(accessToken)
		if userID == -1 {
			return nil, filename, ErrBadAccessToken
		}

		var err error
		u, err = app.db.GetUserByID(userID)
		if err != nil {
			return nil, filename, impart.HTTPError{http.StatusInternalServerError, "Unable to retrieve requested user."}
		}
	} else {
		// Use user cookie
		session, err := app.sessionStore.Get(r, cookieName)
		if err != nil {
			// The cookie should still save, even if there's an error.
			log.Error("Session: %v; ignoring", err)
		}

		val := session.Values[cookieUserVal]
		var ok bool
		if u, ok = val.(*User); !ok {
			return nil, filename, ErrNotLoggedIn
		}
	}

	filename = u.Username + "-posts-" + time.Now().Truncate(time.Second).UTC().Format("200601021504")

	// Fetch data we're exporting
	var err error
	var data []byte
	posts, err := app.db.GetUserPosts(u)
	if err != nil {
		return data, filename, err
	}

	// Export as CSV
	if strings.HasSuffix(r.URL.Path, ".csv") {
		data = exportPostsCSV(app.cfg.App.Host, u, posts)
		return data, filename, err
	}
	if strings.HasSuffix(r.URL.Path, ".zip") {
		data = exportPostsZip(u, posts)
		return data, filename, err
	}

	if r.FormValue("pretty") == "1" {
		data, err = json.MarshalIndent(posts, "", "\t")
	} else {
		data, err = json.Marshal(posts)
	}
	return data, filename, err
}

func viewExportFull(app *App, w http.ResponseWriter, r *http.Request) ([]byte, string, error) {
	var err error
	filename := ""
	u := getUserSession(app, r)
	if u == nil {
		return nil, filename, ErrNotLoggedIn
	}
	filename = u.Username + "-" + time.Now().Truncate(time.Second).UTC().Format("200601021504")

	exportUser := compileFullExport(app, u)

	var data []byte
	if r.FormValue("pretty") == "1" {
		data, err = json.MarshalIndent(exportUser, "", "\t")
	} else {
		data, err = json.Marshal(exportUser)
	}
	return data, filename, err
}

func viewMeAPI(app *App, w http.ResponseWriter, r *http.Request) error {
	reqJSON := IsJSON(r)
	uObj := struct {
		ID       int64  `json:"id,omitempty"`
		Username string `json:"username,omitempty"`
	}{}
	var err error

	if reqJSON {
		_, uObj.Username, err = app.db.GetUserDataFromToken(r.Header.Get("Authorization"))
		if err != nil {
			return err
		}
	} else {
		u := getUserSession(app, r)
		if u == nil {
			return impart.WriteSuccess(w, uObj, http.StatusOK)
		}
		uObj.Username = u.Username
	}

	return impart.WriteSuccess(w, uObj, http.StatusOK)
}

func viewMyPostsAPI(app *App, u *User, w http.ResponseWriter, r *http.Request) error {
	reqJSON := IsJSON(r)
	if !reqJSON {
		return ErrBadRequestedType
	}

	var err error
	p := GetPostsCache(u.ID)
	if p == nil {
		userPostsCache.Lock()
		if userPostsCache.users[u.ID].ready == nil {
			userPostsCache.users[u.ID] = postsCacheItem{ready: make(chan struct{})}
			userPostsCache.Unlock()

			p, err = app.db.GetUserPosts(u)
			if err != nil {
				return err
			}

			CachePosts(u.ID, p)
		} else {
			userPostsCache.Unlock()

			<-userPostsCache.users[u.ID].ready
			p = GetPostsCache(u.ID)
		}
	}

	return impart.WriteSuccess(w, p, http.StatusOK)
}

func viewMyCollectionsAPI(app *App, u *User, w http.ResponseWriter, r *http.Request) error {
	reqJSON := IsJSON(r)
	if !reqJSON {
		return ErrBadRequestedType
	}

	p, err := app.db.GetCollections(u, app.cfg.App.Host)
	if err != nil {
		return err
	}

	return impart.WriteSuccess(w, p, http.StatusOK)
}

func viewArticles(app *App, u *User, w http.ResponseWriter, r *http.Request) error {
	p, err := app.db.GetAnonymousPosts(u)
	if err != nil {
		log.Error("unable to fetch anon posts: %v", err)
	}
	// nil-out AnonymousPosts slice for easy detection in the template
	if p != nil && len(*p) == 0 {
		p = nil
	}

	f, err := getSessionFlashes(app, w, r, nil)
	if err != nil {
		log.Error("unable to fetch flashes: %v", err)
	}

	c, err := app.db.GetPublishableCollections(u, app.cfg.App.Host)
	if err != nil {
		log.Error("unable to fetch collections: %v", err)
	}

	silenced, err := app.db.IsUserSilenced(u.ID)
	if err != nil {
		log.Error("view articles: %v", err)
	}
	d := struct {
		*UserPage
		AnonymousPosts *[]PublicPost
		Collections    *[]Collection
		Silenced       bool
	}{
		UserPage:       NewUserPage(app, r, u, u.Username+"'s Posts", f),
		AnonymousPosts: p,
		Collections:    c,
		Silenced:       silenced,
	}
	d.UserPage.SetMessaging(u)
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Expires", "Thu, 04 Oct 1990 20:00:00 GMT")
	showUserPage(w, "articles", d)

	return nil
}

func viewCollections(app *App, u *User, w http.ResponseWriter, r *http.Request) error {
	c, err := app.db.GetCollections(u, app.cfg.App.Host)
	if err != nil {
		log.Error("unable to fetch collections: %v", err)
		return fmt.Errorf("No collections")
	}

	f, _ := getSessionFlashes(app, w, r, nil)

	uc, _ := app.db.GetUserCollectionCount(u.ID)
	// TODO: handle any errors

	silenced, err := app.db.IsUserSilenced(u.ID)
	if err != nil {
		log.Error("view collections %v", err)
		return fmt.Errorf("view collections: %v", err)
	}
	d := struct {
		*UserPage
		Collections *[]Collection

		UsedCollections, TotalCollections int

		NewBlogsDisabled bool
		Silenced         bool
	}{
		UserPage:         NewUserPage(app, r, u, u.Username+"'s Blogs", f),
		Collections:      c,
		UsedCollections:  int(uc),
		NewBlogsDisabled: !app.cfg.App.CanCreateBlogs(uc),
		Silenced:         silenced,
	}
	d.UserPage.SetMessaging(u)
	showUserPage(w, "collections", d)

	return nil
}

func viewEditCollection(app *App, u *User, w http.ResponseWriter, r *http.Request) error {
	vars := mux.Vars(r)
	c, err := app.db.GetCollection(vars["collection"])
	if err != nil {
		return err
	}
	if c.OwnerID != u.ID {
		return ErrCollectionNotFound
	}

	silenced, err := app.db.IsUserSilenced(u.ID)
	if err != nil {
		log.Error("view edit collection %v", err)
		return fmt.Errorf("view edit collection: %v", err)
	}
	flashes, _ := getSessionFlashes(app, w, r, nil)
	obj := struct {
		*UserPage
		*Collection
		Silenced bool
	}{
		UserPage:   NewUserPage(app, r, u, "Edit "+c.DisplayTitle(), flashes),
		Collection: c,
		Silenced:   silenced,
	}

	showUserPage(w, "collection", obj)
	return nil
}

func updateSettings(app *App, w http.ResponseWriter, r *http.Request) error {
	reqJSON := IsJSON(r)

	var s userSettings
	var u *User
	var sess *sessions.Session
	var err error
	if reqJSON {
		accessToken := r.Header.Get("Authorization")
		if accessToken == "" {
			return ErrNoAccessToken
		}

		u, err = app.db.GetAPIUser(accessToken)
		if err != nil {
			return ErrBadAccessToken
		}

		decoder := json.NewDecoder(r.Body)
		err := decoder.Decode(&s)
		if err != nil {
			log.Error("Couldn't parse settings JSON request: %v\n", err)
			return ErrBadJSON
		}

		// Prevent all username updates
		// TODO: support changing username via JSON API request
		s.Username = ""
	} else {
		u, sess = getUserAndSession(app, r)
		if u == nil {
			return ErrNotLoggedIn
		}

		err := r.ParseForm()
		if err != nil {
			log.Error("Couldn't parse settings form request: %v\n", err)
			return ErrBadFormData
		}

		err = app.formDecoder.Decode(&s, r.PostForm)
		if err != nil {
			log.Error("Couldn't decode settings form request: %v\n", err)
			return ErrBadFormData
		}
	}

	// Do update
	postUpdateReturn := r.FormValue("return")
	redirectTo := "/me/settings"
	if s.IsLogOut {
		redirectTo += "?logout=1"
	} else if postUpdateReturn != "" {
		redirectTo = postUpdateReturn
	}

	// Only do updates on values we need
	if s.Username != "" && s.Username == u.Username {
		// Username hasn't actually changed; blank it out
		s.Username = ""
	}
	err = app.db.ChangeSettings(app, u, &s)
	if err != nil {
		if reqJSON {
			return err
		}

		if err, ok := err.(impart.HTTPError); ok {
			addSessionFlash(app, w, r, err.Message, nil)
		}
	} else {
		// Successful update.
		if reqJSON {
			return impart.WriteSuccess(w, u, http.StatusOK)
		}

		if s.IsLogOut {
			redirectTo = "/me/logout"
		} else {
			sess.Values[cookieUserVal] = u.Cookie()
			addSessionFlash(app, w, r, "Account updated.", nil)
		}
	}

	w.Header().Set("Location", redirectTo)
	w.WriteHeader(http.StatusFound)
	return nil
}

func updatePassphrase(app *App, w http.ResponseWriter, r *http.Request) error {
	accessToken := r.Header.Get("Authorization")
	if accessToken == "" {
		return ErrNoAccessToken
	}

	curPass := r.FormValue("current")
	newPass := r.FormValue("new")
	// Ensure a new password is given (always required)
	if newPass == "" {
		return impart.HTTPError{http.StatusBadRequest, "Provide a new password."}
	}

	userID, sudo := app.db.GetUserIDPrivilege(accessToken)
	if userID == -1 {
		return ErrBadAccessToken
	}

	// Ensure a current password is given if the access token doesn't have sudo
	// privileges.
	if !sudo && curPass == "" {
		return impart.HTTPError{http.StatusBadRequest, "Provide current password."}
	}

	// Hash the new password
	hashedPass, err := auth.HashPass([]byte(newPass))
	if err != nil {
		return impart.HTTPError{http.StatusInternalServerError, "Could not create password hash."}
	}

	// Do update
	err = app.db.ChangePassphrase(userID, sudo, curPass, hashedPass)
	if err != nil {
		return err
	}

	return impart.WriteSuccess(w, struct{}{}, http.StatusOK)
}

func viewStats(app *App, u *User, w http.ResponseWriter, r *http.Request) error {
	var c *Collection
	var err error
	vars := mux.Vars(r)
	alias := vars["collection"]
	if alias != "" {
		c, err = app.db.GetCollection(alias)
		if err != nil {
			return err
		}
		if c.OwnerID != u.ID {
			return ErrCollectionNotFound
		}
	}

	topPosts, err := app.db.GetTopPosts(u, alias)
	if err != nil {
		log.Error("Unable to get top posts: %v", err)
		return err
	}

	flashes, _ := getSessionFlashes(app, w, r, nil)
	titleStats := ""
	if c != nil {
		titleStats = c.DisplayTitle() + " "
	}

	silenced, err := app.db.IsUserSilenced(u.ID)
	if err != nil {
		log.Error("view stats: %v", err)
		return err
	}
	obj := struct {
		*UserPage
		VisitsBlog  string
		Collection  *Collection
		TopPosts    *[]PublicPost
		APFollowers int
		Silenced    bool
	}{
		UserPage:   NewUserPage(app, r, u, titleStats+"Stats", flashes),
		VisitsBlog: alias,
		Collection: c,
		TopPosts:   topPosts,
		Silenced:   silenced,
	}
	if app.cfg.App.Federation {
		folls, err := app.db.GetAPFollowers(c)
		if err != nil {
			return err
		}
		obj.APFollowers = len(*folls)
	}

	showUserPage(w, "stats", obj)
	return nil
}

func viewSettings(app *App, u *User, w http.ResponseWriter, r *http.Request) error {
	fullUser, err := app.db.GetUserByID(u.ID)
	if err != nil {
		log.Error("Unable to get user for settings: %s", err)
		return impart.HTTPError{http.StatusInternalServerError, "Unable to retrieve user data. The humans have been alerted."}
	}

	passIsSet, err := app.db.IsUserPassSet(u.ID)
	if err != nil {
		log.Error("Unable to get isUserPassSet for settings: %s", err)
		return impart.HTTPError{http.StatusInternalServerError, "Unable to retrieve user data. The humans have been alerted."}
	}

	flashes, _ := getSessionFlashes(app, w, r, nil)

	enableOauthSlack := app.Config().SlackOauth.ClientID != ""
	enableOauthWriteAs := app.Config().WriteAsOauth.ClientID != ""
	enableOauthGitLab := app.Config().GitlabOauth.ClientID != ""
	enableOauthGeneric := app.Config().GenericOauth.ClientID != ""
	enableOauthGitea := app.Config().GiteaOauth.ClientID != ""

	oauthAccounts, err := app.db.GetOauthAccounts(r.Context(), u.ID)
	if err != nil {
		log.Error("Unable to get oauth accounts for settings: %s", err)
		return impart.HTTPError{http.StatusInternalServerError, "Unable to retrieve user data. The humans have been alerted."}
	}
	for idx, oauthAccount := range oauthAccounts {
		switch oauthAccount.Provider {
		case "slack":
			enableOauthSlack = false
		case "write.as":
			enableOauthWriteAs = false
		case "gitlab":
			enableOauthGitLab = false
		case "generic":
			oauthAccounts[idx].DisplayName = app.Config().GenericOauth.DisplayName
			oauthAccounts[idx].AllowDisconnect = app.Config().GenericOauth.AllowDisconnect
			enableOauthGeneric = false
		case "gitea":
			enableOauthGitea = false
		}
	}

	displayOauthSection := enableOauthSlack || enableOauthWriteAs || enableOauthGitLab || enableOauthGeneric || enableOauthGitea || len(oauthAccounts) > 0

	obj := struct {
		*UserPage
		Email                   string
		HasPass                 bool
		IsLogOut                bool
		Silenced                bool
		OauthSection            bool
		OauthAccounts           []oauthAccountInfo
		OauthSlack              bool
		OauthWriteAs            bool
		OauthGitLab             bool
		GitLabDisplayName       string
		OauthGeneric            bool
		OauthGenericDisplayName string
		OauthGitea              bool
		GiteaDisplayName        string
	}{
		UserPage:                NewUserPage(app, r, u, "Account Settings", flashes),
		Email:                   fullUser.EmailClear(app.keys),
		HasPass:                 passIsSet,
		IsLogOut:                r.FormValue("logout") == "1",
		Silenced:                fullUser.IsSilenced(),
		OauthSection:            displayOauthSection,
		OauthAccounts:           oauthAccounts,
		OauthSlack:              enableOauthSlack,
		OauthWriteAs:            enableOauthWriteAs,
		OauthGitLab:             enableOauthGitLab,
		GitLabDisplayName:       config.OrDefaultString(app.Config().GitlabOauth.DisplayName, gitlabDisplayName),
		OauthGeneric:            enableOauthGeneric,
		OauthGenericDisplayName: config.OrDefaultString(app.Config().GenericOauth.DisplayName, genericOauthDisplayName),
		OauthGitea:              enableOauthGitea,
		GiteaDisplayName:        config.OrDefaultString(app.Config().GiteaOauth.DisplayName, giteaDisplayName),
	}

	showUserPage(w, "settings", obj)
	return nil
}

func saveTempInfo(app *App, key, val string, r *http.Request, w http.ResponseWriter) error {
	session, err := app.sessionStore.Get(r, "t")
	if err != nil {
		return ErrInternalCookieSession
	}

	session.Values[key] = val
	err = session.Save(r, w)
	if err != nil {
		log.Error("Couldn't saveTempInfo for key-val (%s:%s): %v", key, val, err)
	}
	return err
}

func getTempInfo(app *App, key string, r *http.Request, w http.ResponseWriter) string {
	session, err := app.sessionStore.Get(r, "t")
	if err != nil {
		return ""
	}

	// Get the information
	var s = ""
	var ok bool
	if s, ok = session.Values[key].(string); !ok {
		return ""
	}

	// Delete cookie
	session.Options.MaxAge = -1
	err = session.Save(r, w)
	if err != nil {
		log.Error("Couldn't erase temp data for key %s: %v", key, err)
	}

	// Return value
	return s
}

func removeOauth(app *App, u *User, w http.ResponseWriter, r *http.Request) error {
	provider := r.FormValue("provider")
	clientID := r.FormValue("client_id")
	remoteUserID := r.FormValue("remote_user_id")

	err := app.db.RemoveOauth(r.Context(), u.ID, provider, clientID, remoteUserID)
	if err != nil {
		return impart.HTTPError{Status: http.StatusInternalServerError, Message: err.Error()}
	}

	return impart.HTTPError{Status: http.StatusFound, Message: "/me/settings"}
}

func prepareUserEmail(input string, emailKey []byte) zero.String {
	email := zero.NewString("", input != "")
	if len(input) > 0 {
		encEmail, err := data.Encrypt(emailKey, input)
		if err != nil {
			log.Error("Unable to encrypt email: %s\n", err)
		} else {
			email.String = string(encEmail)
		}
	}
	return email
}
