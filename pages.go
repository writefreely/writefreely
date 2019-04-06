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
			Content: defaultAboutPage(app.cfg),
		}
	}
	return c, nil
}

func getPrivacyPage(app *app) (*instanceContent, error) {
	c, err := app.db.GetDynamicContent("privacy")
	if err != nil {
		return nil, err
	}
	if c == nil {
		c = &instanceContent{
			ID:      "privacy",
			Content: defaultPrivacyPolicy(app.cfg),
			Updated: defaultPageUpdatedTime,
		}
	}
	return c, nil
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
