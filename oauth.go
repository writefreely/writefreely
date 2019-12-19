package writefreely

import (
	"context"
	"encoding/json"
	"github.com/gorilla/sessions"
	"github.com/guregu/null/zero"
	"github.com/writeas/impart"
	"github.com/writeas/web-core/auth"
	"github.com/writeas/web-core/log"
	"github.com/writeas/writefreely/config"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// TokenResponse contains data returned when a token is created either
// through a code exchange or using a refresh token.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
}

// InspectResponse contains data returned when an access token is inspected.
type InspectResponse struct {
	ClientID  string    `json:"client_id"`
	UserID    int64     `json:"user_id"`
	ExpiresAt time.Time `json:"expires_at"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
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
	GenerateOAuthState(context.Context) (string, error)
	ValidateOAuthState(context.Context, string) error
	GetIDForRemoteUser(context.Context, int64) (int64, error)
	CreateUser(*config.Config, *User, string) error
	RecordRemoteUserID(context.Context, int64, int64) error
	GetUserForAuthByID(int64) (*User, error)
}

type HttpClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type oauthHandler struct {
	HttpClient HttpClient
}

// buildAuthURL returns a URL used to initiate authentication.
func buildAuthURL(app OAuthDatastoreProvider, ctx context.Context, clientID, authLocation, callbackURL string) (string, error) {
	state, err := app.DB().GenerateOAuthState(ctx)
	if err != nil {
		return "", err
	}

	u, err := url.Parse(authLocation)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("client_id", clientID)
	q.Set("redirect_uri", callbackURL)
	q.Set("response_type", "code")
	q.Set("state", state)
	u.RawQuery = q.Encode()

	return u.String(), nil
}

func (h oauthHandler) viewOauthInit(app OAuthDatastoreProvider, w http.ResponseWriter, r *http.Request) error {
	location, err := buildAuthURL(app, r.Context(), app.Config().App.OAuthClientID, app.Config().App.OAuthProviderAuthLocation, app.Config().App.OAuthClientCallbackLocation)
	if err != nil {
		log.ErrorLog.Println(err)
		return impart.HTTPError{Status: http.StatusInternalServerError, Message: "Could not prepare OAuth redirect URL."}
	}
	http.Redirect(w, r, location, http.StatusTemporaryRedirect)
	return nil
}

func (h oauthHandler) viewOauthCallback(app OAuthDatastoreProvider, w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	code := r.FormValue("code")
	state := r.FormValue("state")

	err := app.DB().ValidateOAuthState(ctx, state)
	if err != nil {
		return err
	}

	tokenResponse, err := h.exchangeOauthCode(app, ctx, code)
	if err != nil {
		return err
	}

	// Now that we have the access token, let's use it real quick to make sur
	// it really really works.
	tokenInfo, err := h.inspectOauthAccessToken(app, ctx, tokenResponse.AccessToken)
	if err != nil {
		return err
	}

	localUserID, err := app.DB().GetIDForRemoteUser(ctx, tokenInfo.UserID)
	if err != nil {
		return err
	}

	if localUserID == -1 {
		// We don't have, nor do we want, the password from the origin, so we
		//create a random string. If the user needs to set a password, they
		//can do so through the settings page or through the password reset
		//flow.
		randPass, err := randString(14)
		if err != nil {
			return err
		}
		hashedPass, err := auth.HashPass([]byte(randPass))
		if err != nil {
			log.ErrorLog.Println(err)
			return impart.HTTPError{http.StatusInternalServerError, "Could not create password hash."}
		}
		newUser := &User{
			Username:   tokenInfo.Username,
			HashedPass: hashedPass,
			HasPass:    true,
			Email:      zero.NewString("", tokenInfo.Email != ""),
			Created:    time.Now().Truncate(time.Second).UTC(),
		}

		err = app.DB().CreateUser(app.Config(), newUser, newUser.Username)
		if err != nil {
			return err
		}

		err = app.DB().RecordRemoteUserID(ctx, newUser.ID, tokenInfo.UserID)
		if err != nil {
			return err
		}

		return loginOrFail(app, w, r, newUser)
	}

	user, err := app.DB().GetUserForAuthByID(localUserID)
	if err != nil {
		return err
	}
	return loginOrFail(app, w, r, user)
}

func (h oauthHandler) exchangeOauthCode(app OAuthDatastoreProvider, ctx context.Context, code string) (*TokenResponse, error) {
	form := url.Values{}
	form.Add("grant_type", "authorization_code")
	form.Add("redirect_uri", app.Config().App.OAuthClientCallbackLocation)
	form.Add("code", code)
	req, err := http.NewRequest("POST", app.Config().App.OAuthProviderTokenLocation, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.WithContext(ctx)
	req.Header.Set("User-Agent", "writefreely")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(app.Config().App.OAuthClientID, app.Config().App.OAuthClientSecret)

	resp, err := h.HttpClient.Do(req)
	if err != nil {
		return nil, err
	}

	// Nick: I like using limited readers to reduce the risk of an endpoint
	// being broken or compromised.
	lr := io.LimitReader(resp.Body, tokenRequestMaxLen)
	body, err := ioutil.ReadAll(lr)
	if err != nil {
		return nil, err
	}

	var tokenResponse TokenResponse
	err = json.Unmarshal(body, &tokenResponse)
	if err != nil {
		return nil, err
	}
	return &tokenResponse, nil
}

func (h oauthHandler) inspectOauthAccessToken(app OAuthDatastoreProvider, ctx context.Context, accessToken string) (*InspectResponse, error) {
	req, err := http.NewRequest("GET", app.Config().App.OAuthProviderInspectLocation, nil)
	if err != nil {
		return nil, err
	}
	req.WithContext(ctx)
	req.Header.Set("User-Agent", "writefreely")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := h.HttpClient.Do(req)
	if err != nil {
		return nil, err
	}

	// Nick: I like using limited readers to reduce the risk of an endpoint
	// being broken or compromised.
	lr := io.LimitReader(resp.Body, infoRequestMaxLen)
	body, err := ioutil.ReadAll(lr)
	if err != nil {
		return nil, err
	}

	var inspectResponse InspectResponse
	err = json.Unmarshal(body, &inspectResponse)
	if err != nil {
		return nil, err
	}
	return &inspectResponse, nil
}

func loginOrFail(app OAuthDatastoreProvider, w http.ResponseWriter, r *http.Request, user *User) error {
	session, err := app.SessionStore().Get(r, cookieName)
	if err != nil {
		return err
	}
	session.Values[cookieUserVal] = user.Cookie()
	if err = session.Save(r, w); err != nil {
		return err
	}
	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
	return nil
}
