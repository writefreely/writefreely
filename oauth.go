package writefreely

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/guregu/null/zero"
	"github.com/writeas/nerds/store"
	"github.com/writeas/web-core/auth"
	"github.com/writeas/web-core/log"
	"github.com/writeas/writefreely/config"
	"io"
	"io/ioutil"
	"net/http"
	"time"
)

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
	ValidateOAuthState(context.Context, string) (string, string, error)
	GenerateOAuthState(context.Context, string, string) (string, error)

	CreateUser(*config.Config, *User, string) error
	GetUserForAuthByID(int64) (*User, error)
}

type HttpClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type oauthClient interface {
	GetProvider() string
	GetClientID() string
	buildLoginURL(state string) (string, error)
	exchangeOauthCode(ctx context.Context, code string) (*TokenResponse, error)
	inspectOauthAccessToken(ctx context.Context, accessToken string) (*InspectResponse, error)
}

type oauthHandler struct {
	Config      *config.Config
	DB          OAuthDatastore
	Store       sessions.Store
	oauthClient oauthClient
}

func (h oauthHandler) viewOauthInit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	state, err := h.DB.GenerateOAuthState(ctx, h.oauthClient.GetProvider(), h.oauthClient.GetClientID())
	if err != nil {
		failOAuthRequest(w, http.StatusInternalServerError, "could not prepare oauth redirect url")
	}
	location, err := h.oauthClient.buildLoginURL(state)
	if err != nil {
		failOAuthRequest(w, http.StatusInternalServerError, "could not prepare oauth redirect url")
		return
	}
	http.Redirect(w, r, location, http.StatusTemporaryRedirect)
}

func configureSlackOauth(r *mux.Router, app *App) {
	if app.Config().SlackOauth.ClientID != "" {
		oauthClient := slackOauthClient{
			ClientID:         app.Config().SlackOauth.ClientID,
			ClientSecret:     app.Config().SlackOauth.ClientSecret,
			TeamID:           app.Config().SlackOauth.TeamID,
			CallbackLocation: app.Config().App.Host + "/oauth/callback",
			HttpClient:       config.DefaultHTTPClient(),
		}
		configureOauthRoutes(r, app, oauthClient)
	}
}

func configureWriteAsOauth(r *mux.Router, app *App) {
	if app.Config().WriteAsOauth.ClientID != "" {
		oauthClient := writeAsOauthClient{
			ClientID:         app.Config().WriteAsOauth.ClientID,
			ClientSecret:     app.Config().WriteAsOauth.ClientSecret,
			ExchangeLocation: config.OrDefaultString(app.Config().WriteAsOauth.TokenLocation, writeAsExchangeLocation),
			InspectLocation:  config.OrDefaultString(app.Config().WriteAsOauth.InspectLocation, writeAsIdentityLocation),
			AuthLocation:     config.OrDefaultString(app.Config().WriteAsOauth.AuthLocation, writeAsAuthLocation),
			HttpClient:       config.DefaultHTTPClient(),
			CallbackLocation: app.Config().App.Host + "/oauth/callback",
		}
		if oauthClient.ExchangeLocation == "" {

		}
		configureOauthRoutes(r, app, oauthClient)
	}
}

func configureOauthRoutes(r *mux.Router, app *App, oauthClient oauthClient) {
	handler := &oauthHandler{
		Config:      app.Config(),
		DB:          app.DB(),
		Store:       app.SessionStore(),
		oauthClient: oauthClient,
	}
	r.HandleFunc("/oauth/"+oauthClient.GetProvider(), handler.viewOauthInit).Methods("GET")
	r.HandleFunc("/oauth/callback", handler.viewOauthCallback).Methods("GET")
}

func (h oauthHandler) viewOauthCallback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	code := r.FormValue("code")
	state := r.FormValue("state")

	provider, clientID, err := h.DB.ValidateOAuthState(ctx, state)
	if err != nil {
		log.Error("Unable to ValidateOAuthState: %s", err)
		failOAuthRequest(w, http.StatusInternalServerError, err.Error())
		return
	}

	tokenResponse, err := h.oauthClient.exchangeOauthCode(ctx, code)
	if err != nil {
		log.Error("Unable to exchangeOauthCode: %s", err)
		failOAuthRequest(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Now that we have the access token, let's use it real quick to make sur
	// it really really works.
	tokenInfo, err := h.oauthClient.inspectOauthAccessToken(ctx, tokenResponse.AccessToken)
	if err != nil {
		log.Error("Unable to inspectOauthAccessToken: %s", err)
		failOAuthRequest(w, http.StatusInternalServerError, err.Error())
		return
	}

	localUserID, err := h.DB.GetIDForRemoteUser(ctx, tokenInfo.UserID, provider, clientID)
	if err != nil {
		log.Error("Unable to GetIDForRemoteUser: %s", err)
		failOAuthRequest(w, http.StatusInternalServerError, err.Error())
		return
	}

	if localUserID == -1 {
		// We don't have, nor do we want, the password from the origin, so we
		//create a random string. If the user needs to set a password, they
		//can do so through the settings page or through the password reset
		//flow.
		randPass := store.Generate62RandomString(14)
		hashedPass, err := auth.HashPass([]byte(randPass))
		if err != nil {
			failOAuthRequest(w, http.StatusInternalServerError, "unable to create password hash")
			return
		}
		newUser := &User{
			Username:   tokenInfo.Username,
			HashedPass: hashedPass,
			HasPass:    true,
			Email:      zero.NewString(tokenInfo.Email, tokenInfo.Email != ""),
			Created:    time.Now().Truncate(time.Second).UTC(),
		}
		displayName := tokenInfo.DisplayName
		if len(displayName) == 0 {
			displayName = tokenInfo.Username
		}

		err = h.DB.CreateUser(h.Config, newUser, displayName)
		if err != nil {
			failOAuthRequest(w, http.StatusInternalServerError, err.Error())
			return
		}

		err = h.DB.RecordRemoteUserID(ctx, newUser.ID, tokenInfo.UserID, provider, clientID, tokenResponse.AccessToken)
		if err != nil {
			failOAuthRequest(w, http.StatusInternalServerError, err.Error())
			return
		}

		if err := loginOrFail(h.Store, w, r, newUser); err != nil {
			failOAuthRequest(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	user, err := h.DB.GetUserForAuthByID(localUserID)
	if err != nil {
		failOAuthRequest(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err = loginOrFail(h.Store, w, r, user); err != nil {
		failOAuthRequest(w, http.StatusInternalServerError, err.Error())
	}
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

// failOAuthRequest is an HTTP handler helper that formats returned error
// messages.
func failOAuthRequest(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	err := json.NewEncoder(w).Encode(map[string]interface{}{
		"error": message,
	})
	if err != nil {
		panic(err)
	}
}
