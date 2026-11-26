/*
 * Copyright © 2026 Musing Studio LLC.
 *
 * This file is part of WriteFreely.
 *
 * WriteFreely is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License, included
 * in the LICENSE file in this source code package.
 */

package writefreely

import (
	"fmt"
	"html/template"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/csrf"
	"github.com/writeas/impart"
	"github.com/writeas/web-core/log"
	"github.com/writefreely/writefreely/mailer"
)

// maxConfirmEmailsPerDay caps how many confirmation emails one account can be sent in a day, whether by resending
// them or by changing the account's email address.
const maxConfirmEmailsPerDay = 3

var errTooManyConfirmEmails = impart.HTTPError{http.StatusTooManyRequests, "We've already sent you several confirmation emails today. Please check your spam folder, or try again tomorrow."}

var confirmEmailLimiter = struct {
	sync.Mutex
	sent map[int64][]time.Time
}{
	sent: map[int64][]time.Time{},
}

// allowConfirmEmail reports whether the given user may be sent another confirmation email, recording the send when
// they may.
func allowConfirmEmail(userID int64) bool {
	now := time.Now()
	confirmEmailLimiter.Lock()
	defer confirmEmailLimiter.Unlock()

	// Clear out expired entries so the map doesn't grow without bound
	for k, times := range confirmEmailLimiter.sent {
		recent := times[:0]
		for _, t := range times {
			if now.Sub(t) < 24*time.Hour {
				recent = append(recent, t)
			}
		}
		if len(recent) == 0 {
			delete(confirmEmailLimiter.sent, k)
		} else {
			confirmEmailLimiter.sent[k] = recent
		}
	}

	if len(confirmEmailLimiter.sent[userID]) >= maxConfirmEmailsPerDay {
		return false
	}
	confirmEmailLimiter.sent[userID] = append(confirmEmailLimiter.sent[userID], now)
	return true
}

// sendEmailConfirmation emails the user a link to confirm their address. When reissue is true (as on signup, or after
// the address changes), a new token replaces any pending one. Otherwise, the pending token is sent again.
func sendEmailConfirmation(app *App, userID int64, username, toEmail string, reissue bool) error {
	if !app.cfg.Email.Enabled() {
		return fmt.Errorf("email isn't configured")
	}
	if toEmail == "" {
		return fmt.Errorf("user %d has no email", userID)
	}
	if !allowConfirmEmail(userID) {
		return errTooManyConfirmEmails
	}

	var token string
	var err error
	if !reissue {
		token, err = app.db.GetEmailConfirmationToken(userID)
		if err != nil {
			return err
		}
	}
	if token == "" {
		token, err = app.db.CreateEmailConfirmationToken(userID)
		if err != nil {
			return err
		}
	}

	log.Info("[confirm] Sending confirmation email to user %s (ID %d)", username, userID)
	return emailAccountConfirmation(app, toEmail, username, token)
}

func emailAccountConfirmation(app *App, toEmail, username, token string) error {
	mlr, err := mailer.New(app.cfg.Email)
	if err != nil {
		return err
	}

	siteName := app.cfg.App.SiteName
	confirmURL := app.cfg.App.Host + "/me/confirm?t=" + token
	footerPara := "Didn't sign up for " + siteName + "? You can safely ignore this email."

	plainMsg := fmt.Sprintf("Welcome to %s! Your blog \"%s\" is almost ready.\n\nTo finish the signup process and start publishing, you just need to confirm your email address: %s\n\nSee you soon!\n\n- The %s Team\n\n%s", siteName, username, confirmURL, siteName, footerPara)
	m, err := mlr.NewMessage(mailer.FormatAddress(siteName, "noreply-confirm@"+app.cfg.Email.Domain), "Confirm your email address", plainMsg, fmt.Sprintf("<%s>", toEmail))
	if err != nil {
		return err
	}
	m.AddTag("Confirm Email")
	m.SetHTML(fmt.Sprintf(`<html>
	<body style="font-family:Lora, 'Palatino Linotype', Palatino, Baskerville, 'Book Antiqua', 'New York', 'DejaVu serif', serif; font-size: 100%%; margin:1em 2em;">
		<div style="margin:0 auto; max-width: 40em; font-size: 1.2em; line-height: 1.5">
		<h1 style="font-size:1.75em"><a style="text-decoration:none;color:#000;" href="%s">%s</a></h1>
		<p>Welcome to %s! Your blog "<strong>%s</strong>" is almost ready.</p>
		<p>To finish the signup process and start publishing, you just need to confirm your email address:</p>
		<p style="margin-top: 1.5em;margin-bottom: 1.5em"><a href="%s" style="text-decoration: none; padding: 0.5em; color: white; background: rgb(114, 120, 191); -webkit-border-radius: 0.25em; -moz-border-radius: 0.25em; border-radius: 0.25em;">Confirm email address</a></p>
		<p>See you soon!</p>
		<p>- The %s Team</p>
		<p style="font-size: 0.86em;margin:1em auto">%s</p>
		</div>
	</body>
</html>`, app.cfg.App.Host, template.HTMLEscapeString(siteName), template.HTMLEscapeString(siteName), template.HTMLEscapeString(username), confirmURL, template.HTMLEscapeString(siteName), template.HTMLEscapeString(footerPara)))
	return mlr.Send(m)
}

// handleConfirmAccount tells an unconfirmed user to check their email and, when they follow the link in it, shows a
// button that confirms their address. Opening the link alone never confirms anything, because email security scanners
// open links automatically.
func handleConfirmAccount(app *App, u *User, w http.ResponseWriter, r *http.Request) error {
	errMsg := ""
	token := r.FormValue("t")

	// Retrieve the full user, so we can display their email address and check their current status.
	fullUser, err := app.db.GetUserForAuthByID(u.ID)
	if err != nil {
		return err
	}
	if token == "" && !fullUser.NeedsConfirmation() {
		// Nothing to confirm
		return impart.HTTPError{http.StatusFound, "/me/settings"}
	}
	if token != "" {
		// Confirm this is a valid token for this user.
		err := app.db.ValidateEmailConfirmationToken(u.ID, token)
		if err != nil {
			log.Info("Invalid email confirmation token for user %d: %s", u.ID, err)
			errMsg = err.Error()
		}
	}

	if errMsg == "" && r.Method == http.MethodPost {
		// Confirm account email address
		if token == "" {
			errMsg = "No token given."
		} else {
			if err := app.db.MarkEmailConfirmed(u.ID, token); err != nil {
				log.Error("Unable to mark email confirmed for user %s (ID %d): %s", u.Username, u.ID, err)
			}
			if fullUser.IsSilenced() {
				addSessionFlash(app, w, r, "Thanks for confirming your email address! But your account is silenced, so you can't publish.", nil)
				return impart.HTTPError{http.StatusFound, "/me/settings"}
			}

			// Update user status
			var err error
			if fullUser.NeedsConfirmation() {
				err = app.db.SetUserStatus(u.ID, UserActive)
			}
			if err != nil {
				errMsg = err.Error()
			} else {
				// Keep the session's copy of the user current
				u.Status = UserActive
				addSessionFlash(app, w, r, "Thanks for confirming your email address! Now you can start publishing.", nil)
				return impart.HTTPError{http.StatusFound, "/me/settings"}
			}
		}
	}

	flashes, _ := getSessionFlashes(app, w, r, nil)
	p := struct {
		*UserPage
		Email        string
		Token        string
		ErrMsg       string
		EmailEnabled bool
		CSRFField    template.HTML
	}{
		UserPage:     NewUserPage(app, r, u, "Confirm Email Address", flashes),
		Email:        fullUser.EmailClear(app.keys),
		Token:        token,
		ErrMsg:       errMsg,
		EmailEnabled: app.cfg.Email.Enabled(),
		CSRFField:    csrf.TemplateField(r),
	}
	p.NeedsConfirmation = fullUser.NeedsConfirmation()
	showUserPage(w, "confirm-email", p)
	return nil
}

// handleResendConfirmation sends an unconfirmed user another email with their confirmation link.
func handleResendConfirmation(app *App, u *User, w http.ResponseWriter, r *http.Request) error {
	returnLoc := impart.HTTPError{http.StatusFound, "/me/confirm"}

	fullUser, err := app.db.GetUserForAuthByID(u.ID)
	if err != nil {
		return err
	}
	if !fullUser.NeedsConfirmation() {
		return impart.HTTPError{http.StatusFound, "/me/settings"}
	}

	toEmail := fullUser.EmailClear(app.keys)
	err = sendEmailConfirmation(app, fullUser.ID, fullUser.Username, toEmail, false)
	if err != nil {
		if err == errTooManyConfirmEmails {
			addSessionFlash(app, w, r, errTooManyConfirmEmails.Message, nil)
			return returnLoc
		}
		log.Error("Unable to resend confirmation email for %s: %s", fullUser.Username, err)
		addSessionFlash(app, w, r, "We weren't able to send a confirmation email right now. Please try again later.", nil)
		return returnLoc
	}

	addSessionFlash(app, w, r, "We sent another confirmation email to "+toEmail+".", nil)
	return returnLoc
}
