/*
 * Copyright © 2019-2021 Musing Studio LLC.
 *
 * This file is part of WriteFreely.
 *
 * WriteFreely is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License, included
 * in the LICENSE file in this source code package.
 */

package writefreely

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"time"

	"github.com/aymerick/douceur/inliner"
	"github.com/gorilla/mux"
	"github.com/mailgun/mailgun-go"
	stripmd "github.com/writeas/go-strip-markdown/v2"
	"github.com/writeas/impart"
	"github.com/writeas/web-core/data"
	"github.com/writeas/web-core/log"
	"github.com/writefreely/writefreely/key"
	"github.com/writefreely/writefreely/spam"
)

const (
	emailSendDelay = 15
)

type (
	SubmittedSubscription struct {
		CollAlias string
		UserID    int64

		Email string `schema:"email" json:"email"`
		Web   bool   `schema:"web" json:"web"`
		Slug  string `schema:"slug" json:"slug"`
		From  string `schema:"from" json:"from"`
	}

	EmailSubscriber struct {
		ID          string
		CollID      int64
		UserID      sql.NullInt64
		Email       sql.NullString
		Subscribed  time.Time
		Token       string
		Confirmed   bool
		AllowExport bool
		acctEmail   sql.NullString
	}
)

func (es *EmailSubscriber) FinalEmail(keys *key.Keychain) string {
	if !es.UserID.Valid || es.Email.Valid {
		return es.Email.String
	}

	decEmail, err := data.Decrypt(keys.EmailKey, []byte(es.acctEmail.String))
	if err != nil {
		log.Error("Error decrypting user email: %v", err)
		return ""
	}
	return string(decEmail)
}

func (es *EmailSubscriber) SubscribedFriendly() string {
	return es.Subscribed.Format("January 2, 2006")
}

func handleCreateEmailSubscription(app *App, w http.ResponseWriter, r *http.Request) error {
	reqJSON := IsJSON(r)
	vars := mux.Vars(r)
	var err error

	ss := SubmittedSubscription{
		CollAlias: vars["alias"],
	}
	u := getUserSession(app, r)
	if u != nil {
		ss.UserID = u.ID
	}
	if reqJSON {
		// Decode JSON request
		decoder := json.NewDecoder(r.Body)
		err = decoder.Decode(&ss)
		if err != nil {
			log.Error("Couldn't parse new subscription JSON request: %v\n", err)
			return ErrBadJSON
		}
	} else {
		err = r.ParseForm()
		if err != nil {
			log.Error("Couldn't parse new subscription form request: %v\n", err)
			return ErrBadFormData
		}

		err = app.formDecoder.Decode(&ss, r.PostForm)
		if err != nil {
			log.Error("Continuing, but error decoding new subscription form request: %v\n", err)
			//return ErrBadFormData
		}
	}

	c, err := app.db.GetCollection(ss.CollAlias)
	if err != nil {
		log.Error("getCollection: %s", err)
		return err
	}
	c.hostName = app.cfg.App.Host

	from := c.CanonicalURL()
	isAuthorBanned, err := app.db.IsUserSilenced(c.OwnerID)
	if isAuthorBanned {
		log.Info("Author is silenced, so subscription is blocked.")
		return impart.HTTPError{http.StatusFound, from}
	}

	if ss.Web {
		if u != nil && u.ID == c.OwnerID {
			from = "/" + c.Alias + "/"
		}
		from += ss.Slug
	}

	if r.FormValue(spam.HoneypotFieldName()) != "" || r.FormValue("fake_password") != "" {
		log.Info("Honeypot field was filled out! Not subscribing.")
		return impart.HTTPError{http.StatusFound, from}
	}

	if ss.Email == "" && ss.UserID < 1 {
		log.Info("No subscriber data. Not subscribing.")
		return impart.HTTPError{http.StatusFound, from}
	}

	confirmed := app.db.IsSubscriberConfirmed(ss.Email)
	es, err := app.db.AddEmailSubscription(c.ID, ss.UserID, ss.Email, confirmed)
	if err != nil {
		log.Error("addEmailSubscription: %s", err)
		return err
	}

	// Send confirmation email if needed
	if !confirmed {
		err = sendSubConfirmEmail(app, c, ss.Email, es.ID, es.Token)
		if err != nil {
			log.Error("Failed to send subscription confirmation email: %s", err)
			return err
		}
	}

	if ss.Web {
		session, err := app.sessionStore.Get(r, userEmailCookieName)
		if err != nil {
			// The cookie should still save, even if there's an error.
			// Source: https://github.com/gorilla/sessions/issues/16#issuecomment-143642144
			log.Error("Getting user email cookie: %v; ignoring", err)
		}
		if confirmed {
			addSessionFlash(app, w, r, "<strong>Subscribed</strong>. You'll now receive future blog posts via email.", nil)
		} else {
			addSessionFlash(app, w, r, "Please check your email and <strong>click the confirmation link</strong> to subscribe.", nil)
		}
		session.Values[userEmailCookieVal] = ss.Email
		err = session.Save(r, w)
		if err != nil {
			log.Error("save email cookie: %s", err)
			return err
		}

		return impart.HTTPError{http.StatusFound, from}
	}
	return impart.WriteSuccess(w, "", http.StatusAccepted)
}

func handleDeleteEmailSubscription(app *App, w http.ResponseWriter, r *http.Request) error {
	alias := collectionAliasFromReq(r)

	vars := mux.Vars(r)
	subID := vars["subscriber"]
	email := r.FormValue("email")
	token := r.FormValue("t")
	slug := r.FormValue("slug")
	isWeb := r.Method == "GET"

	// Display collection if this is a collection
	var c *Collection
	var err error
	if app.cfg.App.SingleUser {
		c, err = app.db.GetCollectionByID(1)
	} else {
		c, err = app.db.GetCollection(alias)
	}
	if err != nil {
		log.Error("Get collection: %s", err)
		return err
	}

	from := c.CanonicalURL()

	if subID != "" {
		// User unsubscribing via email, so assume action is taken by either current
		// user or not current user, and only use the request's information to
		// satisfy this unsubscribe, i.e. subscriberID and token.
		err = app.db.DeleteEmailSubscriber(subID, token)
	} else {
		// User unsubscribing through the web app, so assume action is taken by
		// currently-auth'd user.
		var userID int64
		u := getUserSession(app, r)
		if u != nil {
			// User is logged in
			userID = u.ID
			if userID == c.OwnerID {
				from = "/" + c.Alias + "/"
			}
		}
		if email == "" && userID <= 0 {
			// Get email address from saved cookie
			session, err := app.sessionStore.Get(r, userEmailCookieName)
			if err != nil {
				log.Error("Unable to get email cookie: %s", err)
			} else {
				email = session.Values[userEmailCookieVal].(string)
			}
		}

		if email == "" && userID <= 0 {
			err = fmt.Errorf("No subscriber given.")
			log.Error("Not deleting subscription: %s", err)
			return err
		}

		err = app.db.DeleteEmailSubscriberByUser(email, userID, c.ID)
	}
	if err != nil {
		log.Error("Unable to delete subscriber: %v", err)
		return err
	}

	if isWeb {
		from += slug
		addSessionFlash(app, w, r, "<strong>Unsubscribed</strong>. You will no longer receive these blog posts via email.", nil)
		return impart.HTTPError{http.StatusFound, from}
	}
	return impart.WriteSuccess(w, "", http.StatusAccepted)
}

func handleConfirmEmailSubscription(app *App, w http.ResponseWriter, r *http.Request) error {
	alias := collectionAliasFromReq(r)
	subID := mux.Vars(r)["subscriber"]
	token := r.FormValue("t")

	var c *Collection
	var err error
	if app.cfg.App.SingleUser {
		c, err = app.db.GetCollectionByID(1)
	} else {
		c, err = app.db.GetCollection(alias)
	}
	if err != nil {
		log.Error("Get collection: %s", err)
		return err
	}

	from := c.CanonicalURL()

	err = app.db.UpdateSubscriberConfirmed(subID, token)
	if err != nil {
		addSessionFlash(app, w, r, err.Error(), nil)
		return impart.HTTPError{http.StatusFound, from}
	}

	addSessionFlash(app, w, r, "<strong>Confirmed</strong>! Thanks. Now you'll receive future blog posts via email.", nil)
	return impart.HTTPError{http.StatusFound, from}
}

func emailPost(app *App, p *PublicPost, collID int64) error {
	p.augmentContent()

	// Do some shortcode replacement.
	// Since the user is receiving this email, we can assume they're subscribed via email.
	p.Content = strings.Replace(p.Content, "<!--emailsub-->", `<p id="emailsub">You're subscribed to email updates.</p>`, -1)

	if p.HTMLContent == template.HTML("") {
		p.formatContent(app.cfg, false, false)
	}
	p.augmentReadingDestination()

	title := p.Title.String
	if title != "" {
		title = p.Title.String + "\n\n"
	}
	plainMsg := title + "A new post from " + p.CanonicalURL(app.cfg.App.Host) + "\n\n" + stripmd.Strip(p.Content)
	plainMsg += `

---------------------------------------------------------------------------------

Originally published on ` + p.Collection.DisplayTitle() + ` (` + p.Collection.CanonicalURL() + `), a blog you subscribe to.

Sent to %recipient.to%. Unsubscribe: ` + p.Collection.CanonicalURL() + `email/unsubscribe/%recipient.id%?t=%recipient.token%`

	gun := mailgun.NewMailgun(app.cfg.Email.Domain, app.cfg.Email.MailgunPrivate)
	m := mailgun.NewMessage(p.Collection.DisplayTitle()+" <"+p.Collection.Alias+"@"+app.cfg.Email.Domain+">", stripmd.Strip(p.DisplayTitle()), plainMsg)
	replyTo := app.db.GetCollectionAttribute(collID, collAttrLetterReplyTo)
	if replyTo != "" {
		m.SetReplyTo(replyTo)
	}

	subs, err := app.db.GetEmailSubscribers(collID, true)
	if err != nil {
		log.Error("Unable to get email subscribers: %v", err)
		return err
	}
	if len(subs) == 0 {
		return nil
	}

	if title != "" {
		title = string(`<h2 id="title">` + p.FormattedDisplayTitle() + `</h2>`)
	}
	m.AddTag("New post")

	fontFam := "Lora, Palatino, Baskerville, serif"
	if p.IsSans() {
		fontFam = `"Open Sans", Tahoma, Arial, sans-serif`
	} else if p.IsMonospace() {
		fontFam = `Hack, consolas, Menlo-Regular, Menlo, Monaco, monospace, monospace`
	}

	// TODO: move this to a templated file and LESS-generated stylesheet
	fullHTML := `<html>
	<head>
		<style>
		body {
			font-size: 120%;
			font-family: ` + fontFam + `;
			margin: 1em 2em;
		}
		#article {
			line-height: 1.5;
			margin: 1.5em 0;
			white-space: pre-wrap;
			word-wrap: break-word;
		}
		h1, h2, h3, h4, h5, h6, p, code {
			display: inline
		}
		img, iframe, video {
			max-width: 100%
		}
		#title {
			margin-bottom: 1em;
			display: block;
		}
		.intro {
			font-style: italic;
			font-size: 0.95em;
		}
		div#footer {
			text-align: center;
			max-width: 35em;
			margin: 2em auto;
		}
		div#footer p {
			display: block;
			font-size: 0.86em;
			color: #666;
		}
		hr {
			border: 1px solid #ccc;
			margin: 2em 1em;
		}
		p#emailsub {
			text-align: center;
			display: inline-block !important;
			width: 100%;
			font-style: italic;
		}
		</style>
	</head>
	<body>
		<div id="article">` + title + `<p class="intro">From <a href="` + p.CanonicalURL(app.cfg.App.Host) + `">` + p.DisplayCanonicalURL() + `</a></p>

` + string(p.HTMLContent) + `</div>
		<hr />
		<div id="footer">
			<p>Originally published on <a href="` + p.Collection.CanonicalURL() + `">` + p.Collection.DisplayTitle() + `</a>, a blog you subscribe to.</p>
			<p>Sent to %recipient.to%. <a href="` + p.Collection.CanonicalURL() + `email/unsubscribe/%recipient.id%?t=%recipient.token%">Unsubscribe</a>.</p>
		</div>
	</body>
</html>`

	// inline CSS
	html, err := inliner.Inline(fullHTML)
	if err != nil {
		log.Error("Unable to inline email HTML: %v", err)
		return err
	}

	m.SetHtml(html)

	log.Info("[email] Adding %d recipient(s)", len(subs))
	for _, s := range subs {
		e := s.FinalEmail(app.keys)
		log.Info("[email] Adding %s", e)
		err = m.AddRecipientAndVariables(e, map[string]interface{}{
			"id":    s.ID,
			"to":    e,
			"token": s.Token,
		})
		if err != nil {
			log.Error("Unable to add receipient %s: %s", e, err)
		}
	}

	res, _, err := gun.Send(m)
	log.Info("[email] Send result: %s", res)
	if err != nil {
		log.Error("Unable to send post email: %v", err)
		return err
	}

	return nil
}

func sendSubConfirmEmail(app *App, c *Collection, email, subID, token string) error {
	if email == "" {
		return fmt.Errorf("You must supply an email to verify.")
	}

	// Send email
	gun := mailgun.NewMailgun(app.cfg.Email.Domain, app.cfg.Email.MailgunPrivate)

	plainMsg := "Confirm your subscription to " + c.DisplayTitle() + ` (` + c.CanonicalURL() + `) to start receiving future posts. Simply click the following link (or copy and paste it into your browser):

` + c.CanonicalURL() + "email/confirm/" + subID + "?t=" + token + `

If you didn't subscribe to this site or you're not sure why you're getting this email, you can delete it. You won't be subscribed or receive any future emails.`
	m := mailgun.NewMessage(c.DisplayTitle()+" <"+c.Alias+"@"+app.cfg.Email.Domain+">", "Confirm your subscription to "+c.DisplayTitle(), plainMsg, fmt.Sprintf("<%s>", email))
	m.AddTag("Email Verification")

	m.SetHtml(`<html>
	<body style="font-family:Lora, 'Palatino Linotype', Palatino, Baskerville, 'Book Antiqua', 'New York', 'DejaVu serif', serif; font-size: 100%%; margin:1em 2em;">
		<div style="font-size: 1.2em;">
			<p>Confirm your subscription to <a href="` + c.CanonicalURL() + `">` + c.DisplayTitle() + `</a> to start receiving future posts:</p>
			<p><a href="` + c.CanonicalURL() + `email/confirm/` + subID + `?t=` + token + `">Subscribe to ` + c.DisplayTitle() + `</a></p>
			<p>If you didn't subscribe to this site or you're not sure why you're getting this email, you can delete it. You won't be subscribed or receive any future emails.</p>
        </div>
	</body>
</html>`)
	gun.Send(m)

	return nil
}
