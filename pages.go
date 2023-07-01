/*
 * Copyright © 2018-2019, 2021 Musing Studio LLC.
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
	"github.com/writefreely/writefreely/config"
	"time"
)

var defaultPageUpdatedTime = time.Date(2018, 11, 8, 12, 0, 0, 0, time.Local)

func getAboutPage(app *App) (*instanceContent, error) {
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
	var setLang = localize(cfg.App.Lang)
	title := setLang.Get("About %s", cfg.App.SiteName)
	return sql.NullString{String: title, Valid: true}
}

func getPrivacyPage(app *App) (*instanceContent, error) {
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
		c.Title = defaultPrivacyTitle(app.cfg)
	}
	return c, nil
}

func defaultPrivacyTitle(cfg *config.Config) sql.NullString {
	var setLang = localize(cfg.App.Lang)	
	title := setLang.Get("Privacy Policy");
	return sql.NullString{String: title, Valid: true}
}

func defaultAboutPage(cfg *config.Config) string {
	var setLang = localize(cfg.App.Lang)
	wf_link := "[WriteFreely](https://writefreely.org)"
	content := setLang.Get("_%s_ is a place for you to write and publish, powered by %s.", cfg.App.SiteName, wf_link)
	if cfg.App.Federation {
		content := setLang.Get("_%s_ is an interconnected place for you to write and publish, powered by %s.", cfg.App.SiteName, wf_link)
		return content
	}
	return content
}

func defaultPrivacyPolicy(cfg *config.Config) string {
	var setLang = localize(cfg.App.Lang)
	wf_link := "[WriteFreely](https://writefreely.org)"
	bold_site_name := "**"+cfg.App.SiteName+"**"

	DefPrivPolString := setLang.Get("%s, the software that powers this site, is built to enforce your right to privacy by default.\n\nIt retains as little data about you as possible, not even requiring an email address to sign up. However, if you _do_ give us your email address, it is stored encrypted in our database.\n\nWe salt and hash your account's password.We store log files, or data about what happens on our servers. We also use cookies to keep you logged in to your account.\n\nBeyond this, it's important that you trust whoever runs %s. Software can only do so much to protect you -- your level of privacy protections will ultimately fall on the humans that run this particular service.", wf_link, bold_site_name);
	return DefPrivPolString
}

func getLandingBanner(app *App) (*instanceContent, error) {
	c, err := app.db.GetDynamicContent("landing-banner")
	if err != nil {
		return nil, err
	}
	if c == nil {
		c = &instanceContent{
			ID:      "landing-banner",
			Type:    "section",
			Content: defaultLandingBanner(app.cfg),
			Updated: defaultPageUpdatedTime,
		}
	}
	return c, nil
}

func getLandingBody(app *App) (*instanceContent, error) {
	c, err := app.db.GetDynamicContent("landing-body")
	if err != nil {
		return nil, err
	}
	if c == nil {
		c = &instanceContent{
			ID:      "landing-body",
			Type:    "section",
			Content: defaultLandingBody(app.cfg),
			Updated: defaultPageUpdatedTime,
		}
	}
	return c, nil
}

func defaultLandingBanner(cfg *config.Config) string {
	//var setLang = localize(cfg)
	var setLang = localize(cfg.App.Lang)
	banner := setLang.Get("# Start your blog")
	if cfg.App.Federation {
		banner := setLang.Get("# Start your blog in the fediverse")
		return banner
	}
	return banner
}

func defaultLandingBody(cfg *config.Config) string {
	var setLang = localize(cfg.App.Lang)
	pixelfed := "[PixelFed](https://pixelfed.org)"
	mastodon := "[Mastodon](https://joinmastodon.org)"
	content1 := setLang.Get("## Join the Fediverse\n\nThe fediverse is a large network of platforms that all speak a common language. Imagine if you could reply to _Instagram_ posts from _Twitter_, or interact with your favorite _Medium_ blogs from _Facebook_ -- federated alternatives like %s, %s, and WriteFreely enable you to do these types of things.\n\n",pixelfed, mastodon)
	iframe := `<div style="text-align:center">
				<iframe style="width: 560px; height: 315px; max-width: 100%;" sandbox="allow-same-origin allow-scripts" src="https://video.writeas.org/videos/embed/cc55e615-d204-417c-9575-7b57674cc6f3" frameborder="0" allowfullscreen></iframe>
			</div>`
	content2 := setLang.Get("## Write More Socially\n\nWriteFreely can communicate with other federated platforms like _Mastodon_, so people can follow your blogs, bookmark their favorite posts, and boost them to their followers. Sign up above to create a blog and join the fediverse.")
	if cfg.App.Federation {
		return content1 + "\n\n" + iframe + "\n\n" + content2
	}
	return ""
}

func getReaderSection(app *App) (*instanceContent, error) {
	c, err := app.db.GetDynamicContent("reader")
	if err != nil {
		return nil, err
	}
	if c == nil {
		c = &instanceContent{
			ID:      "reader",
			Type:    "section",
			Content: defaultReaderBanner(app.cfg),
			Updated: defaultPageUpdatedTime,
		}
	}
	if !c.Title.Valid {
		c.Title = defaultReaderTitle(app.cfg)
	}
	return c, nil
}

func defaultReaderTitle(cfg *config.Config) sql.NullString {
	var setLang = localize(cfg.App.Lang)
	title := setLang.Get("Reader")
	return sql.NullString{String: title, Valid: true}
}

func defaultReaderBanner(cfg *config.Config) string {
	var setLang = localize(cfg.App.Lang)
	return setLang.Get("Read the latest posts form %s.", cfg.App.SiteName)
}
