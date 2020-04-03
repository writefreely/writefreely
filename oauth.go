/*
 * Copyright © 2019-2020 A Bunch Tell LLC.
 *
 * This file is part of WriteFreely.
 *
 * WriteFreely is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License, included
 * in the LICENSE file in this source code package.
 */

package writefreely

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/writeas/impart"
	"github.com/writeas/web-core/log"
	"github.com/writeas/writefreely/config"
)

// OAuthButtons holds display information for different OAuth providers we support.
type OAuthButtons struct {
	SlackEnabled      bool
	WriteAsEnabled    bool
	GitLabEnabled     bool
	GitLabDisplayName string
}

// NewOAuthButtons creates a new OAuthButtons struct based on our app configuration.
func NewOAuthButtons(cfg *config.Config) *OAuthButtons {
	return &OAuthButtons{
		SlackEnabled:      cfg.SlackOauth.ClientID != "",
		WriteAsEnabled:    cfg.WriteAsOauth.ClientID != "",
		GitLabEnabled:     cfg.GitlabOauth.ClientID != "",
		GitLabDisplayName: config.OrDefaultString(cfg.GitlabOauth.DisplayName, gitlabDisplayName),
	}
}

// TokenResponse contains data returned when a token is created either
// through a code exchange or using a refresh token.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	Error        string `json:"error"`
}

// InspectResponse contains data returned when an access token is inspected.
type InspectResponse struct {
	ClientID    string    `json:"client_id"`
	UserID      string    `json:"user_id"`
	ExpiresAt   time.Time `json:"expires_at"`
	Username    string    `json:"username"`
	DisplayName string    `json:"-"`
	Email       string    `json:"email"`
	Error       string    `json:"error"`
}

// tokenRequestMaxLen is the most bytes that we'll read from the /oauth/token
// endpoint. One megabyte is plenty.
const tokenRequestMaxLen = 1000000

// infoRequestMaxLen is the most bytes that we'll read from the
// /oauth/inspect endpoint.
const infoRequestMaxLen = 1000000

// OAuthDatastoreProvider provides a minimal interface of data store, config,
// and session store for use with the oauth handlers.
type OAuthDatastoreProvider interface {
	DB() OAuthDatastore
	Config() *config.Config
	SessionStore() sessions.Store
}

// OAuthDatastore provides a minimal interface of data store methods used in
// oauth functionality.
type OAuthDatastore interface {
	GetIDForRemoteUser(context.Context, string, string, string) (int64, error)
	RecordRemoteUserID(context.Context, int64, string, string, string, string) error
	ValidateOAuthState(context.Context, string) (string, string, int64, string, error)
	GenerateOAuthState(context.Context, string, string, int64, string) (string, error)

	CreateUser(*config.Config, *User, string) error
	GetUserByID(int64) (*User, error)
}

type HttpClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type oauthClient interface {
	GetProvider() string
	GetClientID() string
	GetCallbackLocation() string
	buildLoginURL(state string) (string, error)
	exchangeOauthCode(ctx context.Context, code string) (*TokenResponse, error)
	inspectOauthAccessToken(ctx context.Context, accessToken string) (*InspectResponse, error)
}

type callbackProxyClient struct {
	server           string
	callbackLocation string
	httpClient       HttpClient
}

type oauthHandler struct {
	Config        *config.Config
	DB            OAuthDatastore
	Store         sessions.Store
	EmailKey      []byte
	oauthClient   oauthClient
	callbackProxy *callbackProxyClient
}

func (h oauthHandler) viewOauthInit(app *App, w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	var attachUser int64
	if attach := r.URL.Query().Get("attach"); attach == "t" {
		user, _ := getUserAndSession(app, r)
		if user == nil {
			return impart.HTTPError{http.StatusInternalServerError, "cannot attach auth to user: user not found in session"}
		}
		attachUser = user.ID
	}

	state, err := h.DB.GenerateOAuthState(ctx, h.oauthClient.GetProvider(), h.oauthClient.GetClientID(), attachUser, r.FormValue("invite_code"))
	if err != nil {
		log.Error("viewOauthInit error: %s", err)
		return impart.HTTPError{http.StatusInternalServerError, "could not prepare oauth redirect url"}
	}

	if h.callbackProxy != nil {
		if err := h.callbackProxy.register(ctx, state); err != nil {
			log.Error("viewOauthInit error: %s", err)
			return impart.HTTPError{http.StatusInternalServerError, "could not register state server"}
		}
	}

	location, err := h.oauthClient.buildLoginURL(state)
	if err != nil {
		log.Error("viewOauthInit error: %s", err)
		return impart.HTTPError{http.StatusInternalServerError, "could not prepare oauth redirect url"}
	}
	return impart.HTTPError{http.StatusTemporaryRedirect, location}
}

func configureSlackOauth(parentHandler *Handler, r *mux.Router, app *App) {
	if app.Config().SlackOauth.ClientID != "" {
		callbackLocation := app.Config().App.Host + "/oauth/callback/slack"

		var stateRegisterClient *callbackProxyClient = nil
		if app.Config().SlackOauth.CallbackProxyAPI != "" {
			stateRegisterClient = &callbackProxyClient{
				server:           app.Config().SlackOauth.CallbackProxyAPI,
				callbackLocation: app.Config().App.Host + "/oauth/callback/slack",
				httpClient:       config.DefaultHTTPClient(),
			}
			callbackLocation = app.Config().SlackOauth.CallbackProxy
		}
		oauthClient := slackOauthClient{
			ClientID:         app.Config().SlackOauth.ClientID,
			ClientSecret:     app.Config().SlackOauth.ClientSecret,
			TeamID:           app.Config().SlackOauth.TeamID,
			HttpClient:       config.DefaultHTTPClient(),
			CallbackLocation: callbackLocation,
		}
		configureOauthRoutes(parentHandler, r, app, oauthClient, stateRegisterClient)
	}
}

func configureWriteAsOauth(parentHandler *Handler, r *mux.Router, app *App) {
	if app.Config().WriteAsOauth.ClientID != "" {
		callbackLocation := app.Config().App.Host + "/oauth/callback/write.as"

		var callbackProxy *callbackProxyClient = nil
		if app.Config().WriteAsOauth.CallbackProxy != "" {
			callbackProxy = &callbackProxyClient{
				server:           app.Config().WriteAsOauth.CallbackProxyAPI,
				callbackLocation: app.Config().App.Host + "/oauth/callback/write.as",
				httpClient:       config.DefaultHTTPClient(),
			}
			callbackLocation = app.Config().WriteAsOauth.CallbackProxy
		}

		oauthClient := writeAsOauthClient{
			ClientID:         app.Config().WriteAsOauth.ClientID,
			ClientSecret:     app.Config().WriteAsOauth.ClientSecret,
			ExchangeLocation: config.OrDefaultString(app.Config().WriteAsOauth.TokenLocation, writeAsExchangeLocation),
			InspectLocation:  config.OrDefaultString(app.Config().WriteAsOauth.InspectLocation, writeAsIdentityLocation),
			AuthLocation:     config.OrDefaultString(app.Config().WriteAsOauth.AuthLocation, writeAsAuthLocation),
			HttpClient:       config.DefaultHTTPClient(),
			CallbackLocation: callbackLocation,
		}
		configureOauthRoutes(parentHandler, r, app, oauthClient, callbackProxy)
	}
}

func configureGitlabOauth(parentHandler *Handler, r *mux.Router, app *App) {
	if app.Config().GitlabOauth.ClientID != "" {
		callbackLocation := app.Config().App.Host + "/oauth/callback/gitlab"

		var callbackProxy *callbackProxyClient = nil
		if app.Config().GitlabOauth.CallbackProxy != "" {
			callbackProxy = &callbackProxyClient{
				server:           app.Config().GitlabOauth.CallbackProxyAPI,
				callbackLocation: app.Config().App.Host + "/oauth/callback/gitlab",
				httpClient:       config.DefaultHTTPClient(),
			}
			callbackLocation = app.Config().GitlabOauth.CallbackProxy
		}

		address := config.OrDefaultString(app.Config().GitlabOauth.Host, gitlabHost)
		oauthClient := gitlabOauthClient{
			ClientID:         app.Config().GitlabOauth.ClientID,
			ClientSecret:     app.Config().GitlabOauth.ClientSecret,
			ExchangeLocation: address + "/oauth/token",
			InspectLocation:  address + "/api/v4/user",
			AuthLocation:     address + "/oauth/authorize",
			HttpClient:       config.DefaultHTTPClient(),
			CallbackLocation: callbackLocation,
		}
		configureOauthRoutes(parentHandler, r, app, oauthClient, callbackProxy)
	}
}

func configureGenericOauth(parentHandler *Handler, r *mux.Router, app *App) {
	if app.Config().GenericOauth.ClientID != "" {
		callbackLocation := app.Config().App.Host + "/oauth/callback/generic"

		var callbackProxy *callbackProxyClient = nil
		if app.Config().GenericOauth.CallbackProxy != "" {
			callbackProxy = &callbackProxyClient{
				server:           app.Config().GenericOauth.CallbackProxyAPI,
				callbackLocation: app.Config().App.Host + "/oauth/callback/generic",
				httpClient:       config.DefaultHTTPClient(),
			}
			callbackLocation = app.Config().GenericOauth.CallbackProxy
		}

		oauthClient := genericOauthClient{
			ClientID:         app.Config().GenericOauth.ClientID,
			ClientSecret:     app.Config().GenericOauth.ClientSecret,
			ExchangeLocation: app.Config().GenericOauth.TokenEndpoint,
			InspectLocation:  app.Config().GenericOauth.InspectEndpoint,
			AuthLocation:     app.Config().GenericOauth.AuthEndpoint,
			HttpClient:       config.DefaultHTTPClient(),
			CallbackLocation: callbackLocation,
		}
		configureOauthRoutes(parentHandler, r, app, oauthClient, callbackProxy)
	}
}

func configureOauthRoutes(parentHandler *Handler, r *mux.Router, app *App, oauthClient oauthClient, callbackProxy *callbackProxyClient) {
	handler := &oauthHandler{
		Config:        app.Config(),
		DB:            app.DB(),
		Store:         app.SessionStore(),
		oauthClient:   oauthClient,
		EmailKey:      app.keys.EmailKey,
		callbackProxy: callbackProxy,
	}
	r.HandleFunc("/oauth/"+oauthClient.GetProvider(), parentHandler.OAuth(handler.viewOauthInit)).Methods("GET")
	r.HandleFunc("/oauth/callback/"+oauthClient.GetProvider(), parentHandler.OAuth(handler.viewOauthCallback)).Methods("GET")
	r.HandleFunc("/oauth/signup", parentHandler.OAuth(handler.viewOauthSignup)).Methods("POST")
}

func (h oauthHandler) viewOauthCallback(app *App, w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	code := r.FormValue("code")
	state := r.FormValue("state")

	provider, clientID, attachUserID, inviteCode, err := h.DB.ValidateOAuthState(ctx, state)
	if err != nil {
		log.Error("Unable to ValidateOAuthState: %s", err)
		return impart.HTTPError{http.StatusInternalServerError, err.Error()}
	}

	tokenResponse, err := h.oauthClient.exchangeOauthCode(ctx, code)
	if err != nil {
		log.Error("Unable to exchangeOauthCode: %s", err)
		return impart.HTTPError{http.StatusInternalServerError, err.Error()}
	}

	// Now that we have the access token, let's use it real quick to make sure
	// it really really works.
	tokenInfo, err := h.oauthClient.inspectOauthAccessToken(ctx, tokenResponse.AccessToken)
	if err != nil {
		log.Error("Unable to inspectOauthAccessToken: %s", err)
		return impart.HTTPError{http.StatusInternalServerError, err.Error()}
	}

	localUserID, err := h.DB.GetIDForRemoteUser(ctx, tokenInfo.UserID, provider, clientID)
	if err != nil {
		log.Error("Unable to GetIDForRemoteUser: %s", err)
		return impart.HTTPError{http.StatusInternalServerError, err.Error()}
	}

	if localUserID != -1 && attachUserID > 0 {
		if err = addSessionFlash(app, w, r, "This Slack account is already attached to another user.", nil); err != nil {
			return impart.HTTPError{Status: http.StatusInternalServerError, Message: err.Error()}
		}
		return impart.HTTPError{http.StatusFound, "/me/settings"}
	}

	if localUserID != -1 {
		// Existing user, so log in now
		user, err := h.DB.GetUserByID(localUserID)
		if err != nil {
			log.Error("Unable to GetUserByID %d: %s", localUserID, err)
			return impart.HTTPError{http.StatusInternalServerError, err.Error()}
		}
		if err = loginOrFail(h.Store, w, r, user); err != nil {
			log.Error("Unable to loginOrFail %d: %s", localUserID, err)
			return impart.HTTPError{http.StatusInternalServerError, err.Error()}
		}
		return nil
	}
	if attachUserID > 0 {
		log.Info("attaching to user %d", attachUserID)
		err = h.DB.RecordRemoteUserID(r.Context(), attachUserID, tokenInfo.UserID, provider, clientID, tokenResponse.AccessToken)
		if err != nil {
			return impart.HTTPError{http.StatusInternalServerError, err.Error()}
		}
		return impart.HTTPError{http.StatusFound, "/me/settings"}
	}

	// New user registration below.
	// First, verify that user is allowed to register
	if inviteCode != "" {
		// Verify invite code is valid
		i, err := app.db.GetUserInvite(inviteCode)
		if err != nil {
			return impart.HTTPError{http.StatusInternalServerError, err.Error()}
		}
		if !i.Active(app.db) {
			return impart.HTTPError{http.StatusNotFound, "Invite link has expired."}
		}
	} else if !app.cfg.App.OpenRegistration {
		addSessionFlash(app, w, r, ErrUserNotFound.Error(), nil)
		return impart.HTTPError{http.StatusFound, "/login"}
	}

	displayName := tokenInfo.DisplayName
	if len(displayName) == 0 {
		displayName = tokenInfo.Username
	}

	tp := &oauthSignupPageParams{
		AccessToken:     tokenResponse.AccessToken,
		TokenUsername:   tokenInfo.Username,
		TokenAlias:      tokenInfo.DisplayName,
		TokenEmail:      tokenInfo.Email,
		TokenRemoteUser: tokenInfo.UserID,
		Provider:        provider,
		ClientID:        clientID,
		InviteCode:      inviteCode,
	}
	tp.TokenHash = tp.HashTokenParams(h.Config.Server.HashSeed)

	return h.showOauthSignupPage(app, w, r, tp, nil)
}

func (r *callbackProxyClient) register(ctx context.Context, state string) error {
	form := url.Values{}
	form.Add("state", state)
	form.Add("location", r.callbackLocation)
	req, err := http.NewRequestWithContext(ctx, "POST", r.server, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "writefreely")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("unable register state location: %d", resp.StatusCode)
	}

	return nil
}

func limitedJsonUnmarshal(body io.ReadCloser, n int, thing interface{}) error {
	lr := io.LimitReader(body, int64(n+1))
	data, err := ioutil.ReadAll(lr)
	if err != nil {
		return err
	}
	if len(data) == n+1 {
		return fmt.Errorf("content larger than max read allowance: %d", n)
	}
	return json.Unmarshal(data, thing)
}

func loginOrFail(store sessions.Store, w http.ResponseWriter, r *http.Request, user *User) error {
	// An error may be returned, but a valid session should always be returned.
	session, _ := store.Get(r, cookieName)
	session.Values[cookieUserVal] = user.Cookie()
	if err := session.Save(r, w); err != nil {
		fmt.Println("error saving session", err)
		return err
	}
	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
	return nil
}
