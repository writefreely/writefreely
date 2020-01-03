package writefreely

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/writeas/impart"
	"github.com/writeas/web-core/auth"
	"github.com/writeas/web-core/log"
	"github.com/writeas/writefreely/page"
	"html/template"
	"net/http"
	"strings"
	"time"
)

type viewOauthSignupVars struct {
	page.StaticPage
	To      string
	Message template.HTML
	Flashes []template.HTML

	AccessToken     string
	TokenUsername   string
	TokenAlias      string
	TokenEmail      string
	TokenRemoteUser string
	Provider        string
	ClientID        string
	TokenHash       string

	Username string
	Alias    string
	Email    string
}

const (
	oauthParamAccessToken       = "access_token"
	oauthParamTokenUsername     = "token_username"
	oauthParamTokenAlias        = "token_alias"
	oauthParamTokenEmail        = "token_email"
	oauthParamTokenRemoteUserID = "token_remote_user"
	oauthParamClientID          = "client_id"
	oauthParamProvider          = "provider"
	oauthParamHash              = "signature"
	oauthParamUsername          = "username"
	oauthParamAlias             = "alias"
	oauthParamEmail             = "email"
	oauthParamPassword          = "password"
)

type oauthSignupPageParams struct {
	AccessToken     string
	TokenUsername   string
	TokenAlias      string
	TokenEmail      string
	TokenRemoteUser string
	ClientID        string
	Provider        string
	TokenHash       string
}

func (p oauthSignupPageParams) HashTokenParams(key string) string {
	hasher := sha256.New()
	hasher.Write([]byte(key))
	hasher.Write([]byte(p.AccessToken))
	hasher.Write([]byte(p.TokenUsername))
	hasher.Write([]byte(p.TokenAlias))
	hasher.Write([]byte(p.TokenEmail))
	hasher.Write([]byte(p.TokenRemoteUser))
	hasher.Write([]byte(p.ClientID))
	hasher.Write([]byte(p.Provider))
	return hex.EncodeToString(hasher.Sum(nil))
}

func (h oauthHandler) viewOauthSignup(app *App, w http.ResponseWriter, r *http.Request) error {
	tp := &oauthSignupPageParams{
		AccessToken:     r.FormValue(oauthParamAccessToken),
		TokenUsername:   r.FormValue(oauthParamTokenUsername),
		TokenAlias:      r.FormValue(oauthParamTokenAlias),
		TokenEmail:      r.FormValue(oauthParamTokenEmail),
		TokenRemoteUser: r.FormValue(oauthParamTokenRemoteUserID),
		ClientID:        r.FormValue(oauthParamClientID),
		Provider:        r.FormValue(oauthParamProvider),
	}
	if tp.HashTokenParams(h.Config.Server.HashSeed) != r.FormValue(oauthParamHash) {
		return impart.HTTPError{Status: http.StatusBadRequest, Message: "Request has been tampered with."}
	}
	tp.TokenHash = tp.HashTokenParams(h.Config.Server.HashSeed)
	if err := h.validateOauthSignup(r); err != nil {
		return h.showOauthSignupPage(app, w, r, tp, err)
	}

	hashedPass, err := auth.HashPass([]byte(r.FormValue(oauthParamPassword)))
	if err != nil {
		return h.showOauthSignupPage(app, w, r, tp, fmt.Errorf("unable to hash password"))
	}
	newUser := &User{
		Username:   r.FormValue(oauthParamUsername),
		HashedPass: hashedPass,
		HasPass:    true,
		Email:      prepareUserEmail(r.FormValue(oauthParamEmail), h.EmailKey),
		Created:    time.Now().Truncate(time.Second).UTC(),
	}
	displayName := r.FormValue(oauthParamAlias)
	if len(displayName) == 0 {
		displayName = r.FormValue(oauthParamUsername)
	}

	err = h.DB.CreateUser(h.Config, newUser, displayName)
	if err != nil {
		return h.showOauthSignupPage(app, w, r, tp, err)
	}

	err = h.DB.RecordRemoteUserID(r.Context(), newUser.ID, r.FormValue(oauthParamTokenRemoteUserID), r.FormValue(oauthParamProvider), r.FormValue(oauthParamClientID), r.FormValue(oauthParamAccessToken))
	if err != nil {
		return h.showOauthSignupPage(app, w, r, tp, err)
	}

	if err := loginOrFail(h.Store, w, r, newUser); err != nil {
		return h.showOauthSignupPage(app, w, r, tp, err)
	}
	return nil
}

func (h oauthHandler) validateOauthSignup(r *http.Request) error {
	username := r.FormValue(oauthParamUsername)
	if len(username) < 5 {
		return impart.HTTPError{Status: http.StatusBadRequest, Message: "Username is too short."}
	}
	if len(username) > 20 {
		return impart.HTTPError{Status: http.StatusBadRequest, Message: "Username is too long."}
	}
	alias := r.FormValue(oauthParamAlias)
	if len(alias) < 5 {
		return impart.HTTPError{Status: http.StatusBadRequest, Message: "Alias is too short."}
	}
	if len(alias) > 20 {
		return impart.HTTPError{Status: http.StatusBadRequest, Message: "Alias is too long."}
	}
	password := r.FormValue("password")
	if len(password) < 5 {
		return impart.HTTPError{Status: http.StatusBadRequest, Message: "Password is too short."}
	}
	email := r.FormValue(oauthParamEmail)
	if len(email) > 0 {
		parts := strings.Split(email, "@")
		if len(parts) != 2 || (len(parts[0]) < 1 || len(parts[1]) < 1) {
			return impart.HTTPError{Status: http.StatusBadRequest, Message: "Invalid email address"}
		}
	}
	return nil
}

func (h oauthHandler) showOauthSignupPage(app *App, w http.ResponseWriter, r *http.Request, tp *oauthSignupPageParams, errMsg error) error {
	username := tp.TokenUsername
	alias := tp.TokenAlias
	email := tp.TokenEmail

	session, err := app.sessionStore.Get(r, cookieName)
	if err != nil {
		// Ignore this
		log.Error("Unable to get session; ignoring: %v", err)
	}

	if tmpValue := r.FormValue(oauthParamUsername); len(tmpValue) > 0 {
		username = tmpValue
	}
	if tmpValue := r.FormValue(oauthParamAlias); len(tmpValue) > 0 {
		alias = tmpValue
	}
	if tmpValue := r.FormValue(oauthParamEmail); len(tmpValue) > 0 {
		email = tmpValue
	}

	p := &viewOauthSignupVars{
		StaticPage: pageForReq(app, r),
		To:         r.FormValue("to"),
		Flashes:    []template.HTML{},

		AccessToken:     tp.AccessToken,
		TokenUsername:   tp.TokenUsername,
		TokenAlias:      tp.TokenAlias,
		TokenEmail:      tp.TokenEmail,
		TokenRemoteUser: tp.TokenRemoteUser,
		Provider:        tp.Provider,
		ClientID:        tp.ClientID,
		TokenHash:       tp.TokenHash,

		Username: username,
		Alias:    alias,
		Email:    email,
	}

	// Display any error messages
	flashes, _ := getSessionFlashes(app, w, r, session)
	for _, flash := range flashes {
		p.Flashes = append(p.Flashes, template.HTML(flash))
	}
	if errMsg != nil {
		p.Flashes = append(p.Flashes, template.HTML(errMsg.Error()))
	}
	err = pages["signup-oauth.tmpl"].ExecuteTemplate(w, "base", p)
	if err != nil {
		log.Error("Unable to render signup-oauth: %v", err)
		return err
	}
	return nil
}
