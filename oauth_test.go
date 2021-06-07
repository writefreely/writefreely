/*
 * Copyright © 2019-2021 A Bunch Tell LLC.
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
	"fmt"
	"github.com/gorilla/sessions"
	"github.com/stretchr/testify/assert"
	"github.com/writeas/impart"
	"github.com/writeas/web-core/id"
	"github.com/writefreely/writefreely/config"
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
	DoGenerateOAuthState func(context.Context, string, string, int64, string) (string, error)
	DoValidateOAuthState func(context.Context, string) (string, string, int64, string, error)
	DoGetIDForRemoteUser func(context.Context, string, string, string) (int64, error)
	DoCreateUser         func(*config.Config, *User, string) error
	DoRecordRemoteUserID func(context.Context, int64, string, string, string, string) error
	DoGetUserByID        func(int64) (*User, error)
}

var _ OAuthDatastore = &MockOAuthDatastore{}

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
	cfg.WriteAsOauth = config.WriteAsOauthCfg{
		ClientID:        "development",
		ClientSecret:    "development",
		AuthLocation:    "https://write.as/oauth/login",
		TokenLocation:   "https://write.as/oauth/token",
		InspectLocation: "https://write.as/oauth/inspect",
	}
	cfg.SlackOauth = config.SlackOauthCfg{
		ClientID:     "development",
		ClientSecret: "development",
		TeamID:       "development",
	}
	return cfg
}

func (m *MockOAuthDatastore) ValidateOAuthState(ctx context.Context, state string) (string, string, int64, string, error) {
	if m.DoValidateOAuthState != nil {
		return m.DoValidateOAuthState(ctx, state)
	}
	return "", "", 0, "", nil
}

func (m *MockOAuthDatastore) GetIDForRemoteUser(ctx context.Context, remoteUserID, provider, clientID string) (int64, error) {
	if m.DoGetIDForRemoteUser != nil {
		return m.DoGetIDForRemoteUser(ctx, remoteUserID, provider, clientID)
	}
	return -1, nil
}

func (m *MockOAuthDatastore) CreateUser(cfg *config.Config, u *User, username, description string) error {
	if m.DoCreateUser != nil {
		return m.DoCreateUser(cfg, u, username)
	}
	u.ID = 1
	return nil
}

func (m *MockOAuthDatastore) RecordRemoteUserID(ctx context.Context, localUserID int64, remoteUserID, provider, clientID, accessToken string) error {
	if m.DoRecordRemoteUserID != nil {
		return m.DoRecordRemoteUserID(ctx, localUserID, remoteUserID, provider, clientID, accessToken)
	}
	return nil
}

func (m *MockOAuthDatastore) GetUserByID(userID int64) (*User, error) {
	if m.DoGetUserByID != nil {
		return m.DoGetUserByID(userID)
	}
	user := &User{}
	return user, nil
}

func (m *MockOAuthDatastore) GenerateOAuthState(ctx context.Context, provider string, clientID string, attachUserID int64, inviteCode string) (string, error) {
	if m.DoGenerateOAuthState != nil {
		return m.DoGenerateOAuthState(ctx, provider, clientID, attachUserID, inviteCode)
	}
	return id.Generate62RandomString(14), nil
}

func TestViewOauthInit(t *testing.T) {

	t.Run("success", func(t *testing.T) {
		app := &MockOAuthDatastoreProvider{}
		h := oauthHandler{
			Config:   app.Config(),
			DB:       app.DB(),
			Store:    app.SessionStore(),
			EmailKey: []byte{0xd, 0xe, 0xc, 0xa, 0xf, 0xf, 0xb, 0xa, 0xd},
			oauthClient: writeAsOauthClient{
				ClientID:         app.Config().WriteAsOauth.ClientID,
				ClientSecret:     app.Config().WriteAsOauth.ClientSecret,
				ExchangeLocation: app.Config().WriteAsOauth.TokenLocation,
				InspectLocation:  app.Config().WriteAsOauth.InspectLocation,
				AuthLocation:     app.Config().WriteAsOauth.AuthLocation,
				CallbackLocation: "http://localhost/oauth/callback",
				HttpClient:       nil,
			},
		}
		req, err := http.NewRequest("GET", "/oauth/client", nil)
		assert.NoError(t, err)
		rr := httptest.NewRecorder()
		err = h.viewOauthInit(nil, rr, req)
		assert.NotNil(t, err)
		httpErr, ok := err.(impart.HTTPError)
		assert.True(t, ok)
		assert.Equal(t, http.StatusTemporaryRedirect, httpErr.Status)
		assert.NotEmpty(t, httpErr.Message)
		locURI, err := url.Parse(httpErr.Message)
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
					DoGenerateOAuthState: func(ctx context.Context, provider, clientID string, attachUserID int64, inviteCode string) (string, error) {
						return "", fmt.Errorf("pretend unable to write state error")
					},
				}
			},
		}
		h := oauthHandler{
			Config:   app.Config(),
			DB:       app.DB(),
			Store:    app.SessionStore(),
			EmailKey: []byte{0xd, 0xe, 0xc, 0xa, 0xf, 0xf, 0xb, 0xa, 0xd},
			oauthClient: writeAsOauthClient{
				ClientID:         app.Config().WriteAsOauth.ClientID,
				ClientSecret:     app.Config().WriteAsOauth.ClientSecret,
				ExchangeLocation: app.Config().WriteAsOauth.TokenLocation,
				InspectLocation:  app.Config().WriteAsOauth.InspectLocation,
				AuthLocation:     app.Config().WriteAsOauth.AuthLocation,
				CallbackLocation: "http://localhost/oauth/callback",
				HttpClient:       nil,
			},
		}
		req, err := http.NewRequest("GET", "/oauth/client", nil)
		assert.NoError(t, err)
		rr := httptest.NewRecorder()
		err = h.viewOauthInit(nil, rr, req)
		httpErr, ok := err.(impart.HTTPError)
		assert.True(t, ok)
		assert.NotEmpty(t, httpErr.Message)
		assert.Equal(t, http.StatusInternalServerError, httpErr.Status)
		assert.Equal(t, "could not prepare oauth redirect url", httpErr.Message)
	})
}

func TestViewOauthCallback(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		app := &MockOAuthDatastoreProvider{}
		h := oauthHandler{
			Config:   app.Config(),
			DB:       app.DB(),
			Store:    app.SessionStore(),
			EmailKey: []byte{0xd, 0xe, 0xc, 0xa, 0xf, 0xf, 0xb, 0xa, 0xd},
			oauthClient: writeAsOauthClient{
				ClientID:         app.Config().WriteAsOauth.ClientID,
				ClientSecret:     app.Config().WriteAsOauth.ClientSecret,
				ExchangeLocation: app.Config().WriteAsOauth.TokenLocation,
				InspectLocation:  app.Config().WriteAsOauth.InspectLocation,
				AuthLocation:     app.Config().WriteAsOauth.AuthLocation,
				CallbackLocation: "http://localhost/oauth/callback",
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
								Body:       &StringReadCloser{strings.NewReader(`{"client_id": "development", "user_id": "1", "expires_at": "2019-12-19T11:42:01Z", "username": "nick", "email": "nick@testing.write.as"}`)},
							}, nil
						}

						return &http.Response{
							StatusCode: http.StatusNotFound,
						}, nil
					},
				},
			},
		}
		req, err := http.NewRequest("GET", "/oauth/callback", nil)
		assert.NoError(t, err)
		rr := httptest.NewRecorder()
		err = h.viewOauthCallback(&App{cfg: app.Config(), sessionStore: app.SessionStore()}, rr, req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusTemporaryRedirect, rr.Code)
	})
}
