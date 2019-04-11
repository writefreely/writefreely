/*
 * Copyright © 2018 A Bunch Tell LLC.
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
	"github.com/writeas/writefreely/config"
	"time"
)

var defaultPageUpdatedTime = time.Date(2018, 11, 8, 12, 0, 0, 0, time.Local)

func getAboutPage(app *app) (*instanceContent, error) {
	c, err := app.db.GetDynamicContent("about")
	if err != nil {
		return nil, err
	}
	if c == nil {
		c = &instanceContent{
			ID:      "about",
			Type:    "page",
			Content: defaultAboutPage(app.cfg),
		}
	}
	if !c.Title.Valid {
		c.Title = defaultAboutTitle(app.cfg)
	}
	return c, nil
}

func defaultAboutTitle(cfg *config.Config) sql.NullString {
	return sql.NullString{String: "About " + cfg.App.SiteName, Valid: true}
}

func getPrivacyPage(app *app) (*instanceContent, error) {
	c, err := app.db.GetDynamicContent("privacy")
	if err != nil {
		return nil, err
	}
	if c == nil {
		c = &instanceContent{
			ID:      "privacy",
			Type:    "page",
			Content: defaultPrivacyPolicy(app.cfg),
			Updated: defaultPageUpdatedTime,
		}
	}
	if !c.Title.Valid {
		c.Title = defaultPrivacyTitle()
	}
	return c, nil
}

func defaultPrivacyTitle() sql.NullString {
	return sql.NullString{String: "Privacy Policy", Valid: true}
}

func defaultAboutPage(cfg *config.Config) string {
	if cfg.App.Federation {
		return `_` + cfg.App.SiteName + `_ is an interconnected place for you to write and publish, powered by WriteFreely and ActivityPub.`
	}
	return `_` + cfg.App.SiteName + `_ is a place for you to write and publish, powered by WriteFreely.`
}

func defaultPrivacyPolicy(cfg *config.Config) string {
	return `[Write Freely](https://writefreely.org), the software that powers this site, is built to enforce your right to privacy by default.

It retains as little data about you as possible, not even requiring an email address to sign up. However, if you _do_ give us your email address, it is stored encrypted in our database. We salt and hash your account's password.

We store log files, or data about what happens on our servers. We also use cookies to keep you logged in to your account.

Beyond this, it's important that you trust whoever runs **` + cfg.App.SiteName + `**. Software can only do so much to protect you -- your level of privacy protections will ultimately fall on the humans that run this particular service.`
}
