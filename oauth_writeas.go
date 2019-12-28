package writefreely

import (
	"context"
	"net/http"
	"net/url"
	"strings"
)

type writeAsOauthClient struct {
	ClientID         string
	ClientSecret     string
	AuthLocation     string
	ExchangeLocation string
	InspectLocation  string
	CallbackLocation string
	HttpClient       HttpClient
}

var _ oauthClient = writeAsOauthClient{}

func (c writeAsOauthClient) GetProvider() string {
	return "write.as"
}

func (c writeAsOauthClient) GetClientID() string {
	return c.ClientID
}

func (c writeAsOauthClient) buildLoginURL(state string) (string, error) {
	u, err := url.Parse(c.AuthLocation)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("client_id", c.ClientID)
	q.Set("redirect_uri", c.CallbackLocation)
	q.Set("response_type", "code")
	q.Set("state", state)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func (c writeAsOauthClient) exchangeOauthCode(ctx context.Context, code string) (*TokenResponse, error) {
	form := url.Values{}
	form.Add("grant_type", "authorization_code")
	form.Add("redirect_uri", c.CallbackLocation)
	form.Add("code", code)
	req, err := http.NewRequest("POST", c.ExchangeLocation, strings.NewReader(form.Encode()))
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

	var tokenResponse TokenResponse
	if err := limitedJsonUnmarshal(resp.Body, tokenRequestMaxLen, &tokenResponse); err != nil {
		return nil, err
	}
	return &tokenResponse, nil
}

func (c writeAsOauthClient) inspectOauthAccessToken(ctx context.Context, accessToken string) (*InspectResponse, error) {
	req, err := http.NewRequest("GET", c.InspectLocation, nil)
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

	var inspectResponse InspectResponse
	if err := limitedJsonUnmarshal(resp.Body, infoRequestMaxLen, &inspectResponse); err != nil {
		return nil, err
	}
	return &inspectResponse, nil
}
