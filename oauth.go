package writefreely

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/writeas/impart"
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
	GetUserByID(int64) (*User, error)
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
	EmailKey    []byte
	oauthClient oauthClient
}

func (h oauthHandler) viewOauthInit(app *App, w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	state, err := h.DB.GenerateOAuthState(ctx, h.oauthClient.GetProvider(), h.oauthClient.GetClientID())
	if err != nil {
		return impart.HTTPError{http.StatusInternalServerError, "could not prepare oauth redirect url"}
	}
	location, err := h.oauthClient.buildLoginURL(state)
	if err != nil {
		return impart.HTTPError{http.StatusInternalServerError, "could not prepare oauth redirect url"}
	}
	return impart.HTTPError{http.StatusTemporaryRedirect, location}
}

func configureSlackOauth(parentHandler *Handler, r *mux.Router, app *App) {
	if app.Config().SlackOauth.ClientID != "" {
		oauthClient := slackOauthClient{
			ClientID:         app.Config().SlackOauth.ClientID,
			ClientSecret:     app.Config().SlackOauth.ClientSecret,
			TeamID:           app.Config().SlackOauth.TeamID,
			CallbackLocation: app.Config().App.Host + "/oauth/callback",
			HttpClient:       config.DefaultHTTPClient(),
		}
		configureOauthRoutes(parentHandler, r, app, oauthClient)
	}
}

func configureWriteAsOauth(parentHandler *Handler, r *mux.Router, app *App) {
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
		configureOauthRoutes(parentHandler, r, app, oauthClient)
	}
}

func configureOauthRoutes(parentHandler *Handler, r *mux.Router, app *App, oauthClient oauthClient) {
	handler := &oauthHandler{
		Config:      app.Config(),
		DB:          app.DB(),
		Store:       app.SessionStore(),
		oauthClient: oauthClient,
		EmailKey:    app.keys.EmailKey,
	}
	r.HandleFunc("/oauth/"+oauthClient.GetProvider(), parentHandler.OAuth(handler.viewOauthInit)).Methods("GET")
	r.HandleFunc("/oauth/callback", parentHandler.OAuth(handler.viewOauthCallback)).Methods("GET")
	r.HandleFunc("/oauth/signup", parentHandler.OAuth(handler.viewOauthSignup)).Methods("POST")
}

func (h oauthHandler) viewOauthCallback(app *App, w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	code := r.FormValue("code")
	state := r.FormValue("state")

	provider, clientID, err := h.DB.ValidateOAuthState(ctx, state)
	if err != nil {
		log.Error("Unable to ValidateOAuthState: %s", err)
		return impart.HTTPError{http.StatusInternalServerError, err.Error()}
	}

	tokenResponse, err := h.oauthClient.exchangeOauthCode(ctx, code)
	if err != nil {
		log.Error("Unable to exchangeOauthCode: %s", err)
		return impart.HTTPError{http.StatusInternalServerError, err.Error()}
	}

	// Now that we have the access token, let's use it real quick to make sur
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

	if localUserID != -1 {
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

	tp := &oauthSignupPageParams{
		AccessToken:     tokenResponse.AccessToken,
		TokenUsername:   tokenInfo.Username,
		TokenAlias:      tokenInfo.DisplayName,
		TokenEmail:      tokenInfo.Email,
		TokenRemoteUser: tokenInfo.UserID,
		Provider:        provider,
		ClientID:        clientID,
	}
	tp.TokenHash = tp.HashTokenParams(h.Config.Server.HashSeed)

	return h.showOauthSignupPage(app, w, r, tp, nil)
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
