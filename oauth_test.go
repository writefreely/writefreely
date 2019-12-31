package writefreely

import (
	"context"
	"fmt"
	"github.com/gorilla/sessions"
	"github.com/stretchr/testify/assert"
	"github.com/writeas/nerds/store"
	"github.com/writeas/writefreely/config"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

type MockOAuthDatastoreProvider struct {
	DoDB           func() OAuthDatastore
	DoConfig       func() *config.Config
	DoSessionStore func() sessions.Store
}

type MockOAuthDatastore struct {
	DoGenerateOAuthState func(ctx context.Context) (string, error)
	DoValidateOAuthState func(context.Context, string) error
	DoGetIDForRemoteUser func(context.Context, int64) (int64, error)
	DoCreateUser         func(*config.Config, *User, string) error
	DoRecordRemoteUserID func(context.Context, int64, int64) error
	DoGetUserForAuthByID func(int64) (*User, error)
}

type StringReadCloser struct {
	*strings.Reader
}

func (src *StringReadCloser) Close() error {
	return nil
}

type MockHTTPClient struct {
	DoDo func(req *http.Request) (*http.Response, error)
}

func (m *MockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	if m.DoDo != nil {
		return m.DoDo(req)
	}
	return &http.Response{}, nil
}

func (m *MockOAuthDatastoreProvider) SessionStore() sessions.Store {
	if m.DoSessionStore != nil {
		return m.DoSessionStore()
	}
	return sessions.NewCookieStore([]byte("secret-key"))
}

func (m *MockOAuthDatastoreProvider) DB() OAuthDatastore {
	if m.DoDB != nil {
		return m.DoDB()
	}
	return &MockOAuthDatastore{}
}

func (m *MockOAuthDatastoreProvider) Config() *config.Config {
	if m.DoConfig != nil {
		return m.DoConfig()
	}
	cfg := config.New()
	cfg.UseSQLite(true)
	cfg.App.EnableOAuth = true
	cfg.App.OAuthProviderAuthLocation = "https://write.as/oauth/login"
	cfg.App.OAuthProviderTokenLocation = "https://write.as/oauth/token"
	cfg.App.OAuthProviderInspectLocation = "https://write.as/oauth/inspect"
	cfg.App.OAuthClientCallbackLocation = "http://localhost/oauth/callback"
	cfg.App.OAuthClientID = "development"
	cfg.App.OAuthClientSecret = "development"
	return cfg
}

func (m *MockOAuthDatastore) ValidateOAuthState(ctx context.Context, state string) error {
	if m.DoValidateOAuthState != nil {
		return m.DoValidateOAuthState(ctx, state)
	}
	return nil
}

func (m *MockOAuthDatastore) GetIDForRemoteUser(ctx context.Context, remoteUserID int64) (int64, error) {
	if m.DoGetIDForRemoteUser != nil {
		return m.DoGetIDForRemoteUser(ctx, remoteUserID)
	}
	return -1, nil
}

func (m *MockOAuthDatastore) CreateUser(cfg *config.Config, u *User, username string) error {
	if m.DoCreateUser != nil {
		return m.DoCreateUser(cfg, u, username)
	}
	u.ID = 1
	return nil
}

func (m *MockOAuthDatastore) RecordRemoteUserID(ctx context.Context, localUserID int64, remoteUserID int64) error {
	if m.DoRecordRemoteUserID != nil {
		return m.DoRecordRemoteUserID(ctx, localUserID, remoteUserID)
	}
	return nil
}

func (m *MockOAuthDatastore) GetUserForAuthByID(userID int64) (*User, error) {
	if m.DoGetUserForAuthByID != nil {
		return m.DoGetUserForAuthByID(userID)
	}
	user := &User{

	}
	return user, nil
}

func (m *MockOAuthDatastore) GenerateOAuthState(ctx context.Context) (string, error) {
	if m.DoGenerateOAuthState != nil {
		return m.DoGenerateOAuthState(ctx)
	}
	return store.Generate62RandomString(14), nil
}

func TestViewOauthInit(t *testing.T) {

	t.Run("success", func(t *testing.T) {
		app := &MockOAuthDatastoreProvider{}
		h := oauthHandler{
			Config: app.Config(),
			DB:     app.DB(),
			Store:  app.SessionStore(),
		}
		req, err := http.NewRequest("GET", "/oauth/client", nil)
		assert.NoError(t, err)
		rr := httptest.NewRecorder()
		h.viewOauthInit(rr, req)
		assert.Equal(t, http.StatusTemporaryRedirect, rr.Code)
		locURI, err := url.Parse(rr.Header().Get("Location"))
		assert.NoError(t, err)
		assert.Equal(t, "/oauth/login", locURI.Path)
		assert.Equal(t, "development", locURI.Query().Get("client_id"))
		assert.Equal(t, "http://localhost/oauth/callback", locURI.Query().Get("redirect_uri"))
		assert.Equal(t, "code", locURI.Query().Get("response_type"))
		assert.NotEmpty(t, locURI.Query().Get("state"))
	})

	t.Run("state failure", func(t *testing.T) {
		app := &MockOAuthDatastoreProvider{
			DoDB: func() OAuthDatastore {
				return &MockOAuthDatastore{
					DoGenerateOAuthState: func(ctx context.Context) (string, error) {
						return "", fmt.Errorf("pretend unable to write state error")
					},
				}
			},
		}
		h := oauthHandler{
			Config: app.Config(),
			DB:     app.DB(),
			Store:  app.SessionStore(),
		}
		req, err := http.NewRequest("GET", "/oauth/client", nil)
		assert.NoError(t, err)
		rr := httptest.NewRecorder()
		h.viewOauthInit(rr, req)
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		expected := `{"error":"could not prepare oauth redirect url"}` + "\n"
		assert.Equal(t, expected, rr.Body.String())
	})
}

func TestViewOauthCallback(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		app := &MockOAuthDatastoreProvider{}
		h := oauthHandler{
			Config: app.Config(),
			DB:     app.DB(),
			Store:  app.SessionStore(),
			HttpClient: &MockHTTPClient{
				DoDo: func(req *http.Request) (*http.Response, error) {
					switch req.URL.String() {
					case "https://write.as/oauth/token":
						return &http.Response{
							StatusCode: 200,
							Body:       &StringReadCloser{strings.NewReader(`{"access_token": "access_token", "expires_in": 1000, "refresh_token": "refresh_token", "token_type": "access"}`)},
						}, nil
					case "https://write.as/oauth/inspect":
						return &http.Response{
							StatusCode: 200,
							Body:       &StringReadCloser{strings.NewReader(`{"client_id": "development", "user_id": 1, "expires_at": "2019-12-19T11:42:01Z", "username": "nick", "email": "nick@testing.write.as"}`)},
						}, nil
					}

					return &http.Response{
						StatusCode: http.StatusNotFound,
					}, nil
				},
			},
		}
		req, err := http.NewRequest("GET", "/oauth/callback", nil)
		assert.NoError(t, err)
		rr := httptest.NewRecorder()
		h.viewOauthCallback(rr, req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusTemporaryRedirect, rr.Code)

	})
}
