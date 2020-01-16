/*
 * Copyright © 2020 A Bunch Tell LLC.
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
	"errors"
	"fmt"
	"github.com/writeas/nerds/store"
	"github.com/writeas/slug"
	"net/http"
	"net/url"
	"strings"
)

type slackOauthClient struct {
	ClientID         string
	ClientSecret     string
	TeamID           string
	CallbackLocation string
	HttpClient       HttpClient
}

type slackExchangeResponse struct {
	OK          bool   `json:"ok"`
	AccessToken string `json:"access_token"`
	Scope       string `json:"scope"`
	TeamName    string `json:"team_name"`
	TeamID      string `json:"team_id"`
	Error       string `json:"error"`
}

type slackIdentity struct {
	Name  string `json:"name"`
	ID    string `json:"id"`
	Email string `json:"email"`
}

type slackTeam struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

type slackUserIdentityResponse struct {
	OK    bool          `json:"ok"`
	User  slackIdentity `json:"user"`
	Team  slackTeam     `json:"team"`
	Error string        `json:"error"`
}

const (
	slackAuthLocation     = "https://slack.com/oauth/authorize"
	slackExchangeLocation = "https://slack.com/api/oauth.access"
	slackIdentityLocation = "https://slack.com/api/users.identity"
)

var _ oauthClient = slackOauthClient{}

func (c slackOauthClient) GetProvider() string {
	return "slack"
}

func (c slackOauthClient) GetClientID() string {
	return c.ClientID
}

func (c slackOauthClient) GetCallbackLocation() string {
	return c.CallbackLocation
}

func (c slackOauthClient) buildLoginURL(state string) (string, error) {
	u, err := url.Parse(slackAuthLocation)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("client_id", c.ClientID)
	q.Set("scope", "identity.basic identity.email identity.team")
	q.Set("redirect_uri", c.CallbackLocation)
	q.Set("state", state)

	// If this param is not set, the user can select which team they
	// authenticate through and then we'd have to match the configured team
	// against the profile get. That is extra work in the post-auth phase
	// that we don't want to do.
	q.Set("team", c.TeamID)

	// The Slack OAuth docs don't explicitly list this one, but it is part of
	// the spec, so we include it anyway.
	q.Set("response_type", "code")
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func (c slackOauthClient) exchangeOauthCode(ctx context.Context, code string) (*TokenResponse, error) {
	form := url.Values{}
	// The oauth.access documentation doesn't explicitly mention this
	// parameter, but it is part of the spec, so we include it anyway.
	// https://api.slack.com/methods/oauth.access
	form.Add("grant_type", "authorization_code")
	form.Add("redirect_uri", c.CallbackLocation)
	form.Add("code", code)
	req, err := http.NewRequest("POST", slackExchangeLocation, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.WithContext(ctx)
	req.Header.Set("User-Agent", "writefreely")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(c.ClientID, c.ClientSecret)

	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("unable to exchange code for access token")
	}

	var tokenResponse slackExchangeResponse
	if err := limitedJsonUnmarshal(resp.Body, tokenRequestMaxLen, &tokenResponse); err != nil {
		return nil, err
	}
	if !tokenResponse.OK {
		return nil, errors.New(tokenResponse.Error)
	}
	return tokenResponse.TokenResponse(), nil
}

func (c slackOauthClient) inspectOauthAccessToken(ctx context.Context, accessToken string) (*InspectResponse, error) {
	req, err := http.NewRequest("GET", slackIdentityLocation, nil)
	if err != nil {
		return nil, err
	}
	req.WithContext(ctx)
	req.Header.Set("User-Agent", "writefreely")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("unable to inspect access token")
	}

	var inspectResponse slackUserIdentityResponse
	if err := limitedJsonUnmarshal(resp.Body, infoRequestMaxLen, &inspectResponse); err != nil {
		return nil, err
	}
	if !inspectResponse.OK {
		return nil, errors.New(inspectResponse.Error)
	}
	return inspectResponse.InspectResponse(), nil
}

func (resp slackUserIdentityResponse) InspectResponse() *InspectResponse {
	return &InspectResponse{
		UserID:      resp.User.ID,
		Username:    fmt.Sprintf("%s-%s", slug.Make(resp.User.Name), store.GenerateRandomString("0123456789bcdfghjklmnpqrstvwxyz", 5)),
		DisplayName: resp.User.Name,
		Email:       resp.User.Email,
	}
}

func (resp slackExchangeResponse) TokenResponse() *TokenResponse {
	return &TokenResponse{
		AccessToken: resp.AccessToken,
	}
}
