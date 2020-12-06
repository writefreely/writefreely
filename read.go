/*
 * Copyright © 2018-2020 A Bunch Tell LLC.
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
	"fmt"
	"html/template"
	"math"
	"net/http"
	"strconv"
	"time"

	. "github.com/gorilla/feeds"
	"github.com/gorilla/mux"
	stripmd "github.com/writeas/go-strip-markdown"
	"github.com/writeas/impart"
	"github.com/writeas/web-core/log"
	"github.com/writeas/web-core/memo"
	"github.com/writeas/writefreely/page"
)

const (
	tlFeedLimit      = 100
	tlAPIPageLimit   = 10
	tlMaxAuthorPosts = 5
	tlPostsPerPage   = 16
	tlMaxPostCache   = 250
	tlCacheDur       = 10 * time.Minute
)

type localTimeline struct {
	m     *memo.Memo
	posts *[]PublicPost

	// Configuration values
	postsPerPage int
}

type readPublication struct {
	page.StaticPage
	Posts       *[]PublicPost
	CurrentPage int
	TotalPages  int
	SelTopic    string
	IsAdmin     bool
	CanInvite   bool

	// Customizable page content
	ContentTitle string
	Content      template.HTML
}

func initLocalTimeline(app *App) {
	app.timeline = &localTimeline{
		postsPerPage: tlPostsPerPage,
		m:            memo.New(app.FetchPublicPosts, tlCacheDur),
	}
}

// satisfies memo.Func
func (app *App) FetchPublicPosts() (interface{}, error) {
	// Conditions
	limit := fmt.Sprintf("LIMIT %d", tlMaxPostCache)
	// This is better than the hard limit when limiting posts from individual authors
	// ageCond := `p.created >= ` + app.db.dateSub(3, "month") + ` AND `

	// Finds all public posts and posts in a public collection published during the owner's active subscription period and within the last 3 months
	rows, err := app.db.Query(`SELECT p.id, alias, c.title, p.slug, p.title, p.content, p.text_appearance, p.language, p.rtl, p.created, p.updated
	FROM collections c
	LEFT JOIN posts p ON p.collection_id = c.id
	LEFT JOIN users u ON u.id = p.owner_id
	WHERE c.privacy = 1 AND (p.created <= ` + app.db.now() + ` AND pinned_position IS NULL) AND u.status = 0
	ORDER BY p.created DESC
	` + limit)
	if err != nil {
		log.Error("Failed selecting from posts: %v", err)
		return nil, impart.HTTPError{http.StatusInternalServerError, "Couldn't retrieve collection posts." + err.Error()}
	}
	defer rows.Close()

	ap := map[string]uint{}

	posts := []PublicPost{}
	for rows.Next() {
		p := &Post{}
		c := &Collection{}
		var alias, title sql.NullString
		err = rows.Scan(&p.ID, &alias, &title, &p.Slug, &p.Title, &p.Content, &p.Font, &p.Language, &p.RTL, &p.Created, &p.Updated)
		if err != nil {
			log.Error("[READ] Unable to scan row, skipping: %v", err)
			continue
		}
		c.hostName = app.cfg.App.Host

		isCollectionPost := alias.Valid
		if isCollectionPost {
			c.Alias = alias.String
			if c.Alias != "" && ap[c.Alias] == tlMaxAuthorPosts {
				// Don't add post if we've hit the post-per-author limit
				continue
			}

			c.Public = true
			c.Title = title.String
		}

		p.extractData()
		p.HTMLContent = template.HTML(applyMarkdown([]byte(p.Content), "", app.cfg))
		fp := p.processPost()
		if isCollectionPost {
			fp.Collection = &CollectionObj{Collection: *c}
		}

		posts = append(posts, fp)
		ap[c.Alias]++
	}

	return posts, nil
}

func viewLocalTimelineAPI(app *App, w http.ResponseWriter, r *http.Request) error {
	updateTimelineCache(app.timeline, false)

	skip, _ := strconv.Atoi(r.FormValue("skip"))

	posts := []PublicPost{}
	for i := skip; i < skip+tlAPIPageLimit && i < len(*app.timeline.posts); i++ {
		posts = append(posts, (*app.timeline.posts)[i])
	}

	return impart.WriteSuccess(w, posts, http.StatusOK)
}

func viewLocalTimeline(app *App, w http.ResponseWriter, r *http.Request) error {
	if !app.cfg.App.LocalTimeline {
		return impart.HTTPError{http.StatusNotFound, "Page doesn't exist."}
	}

	vars := mux.Vars(r)
	var p int
	page := 1
	p, _ = strconv.Atoi(vars["page"])
	if p > 0 {
		page = p
	}

	return showLocalTimeline(app, w, r, page, vars["author"], vars["tag"])
}

// updateTimelineCache will reset and update the cache if it is stale or
// the boolean passed in is true.
func updateTimelineCache(tl *localTimeline, reset bool) {
	if reset {
		tl.Reset()
	}

	// Fetch posts if the cache is empty, has been reset or enough time has
	// passed since last cache.
	if tl.posts == nil || reset || tl.m.Invalidate() {
		log.Info("[READ] Updating post cache")

		postsInterfaces, err := tl.m.Get()
		if err != nil {
			log.Error("[READ] Unable to cache posts: %v", err)
		}

		castPosts := postsInterfaces.([]PublicPost)
		tl.posts = &castPosts
	}
}

func showLocalTimeline(app *App, w http.ResponseWriter, r *http.Request, page int, author, tag string) error {
	updateTimelineCache(app.timeline, false)

	pl := len(*(app.timeline.posts))
	ttlPages := int(math.Ceil(float64(pl) / float64(app.timeline.postsPerPage)))

	start := 0
	if page > 1 {
		start = app.timeline.postsPerPage * (page - 1)
		if start > pl {
			return impart.HTTPError{http.StatusFound, fmt.Sprintf("/read/p/%d", ttlPages)}
		}
	}
	end := app.timeline.postsPerPage * page
	if end > pl {
		end = pl
	}
	var posts []PublicPost
	if author != "" {
		posts = []PublicPost{}
		for _, p := range *app.timeline.posts {
			if author == "anonymous" {
				if p.Collection == nil {
					posts = append(posts, p)
				}
			} else if p.Collection != nil && p.Collection.Alias == author {
				posts = append(posts, p)
			}
		}
	} else if tag != "" {
		posts = []PublicPost{}
		for _, p := range *app.timeline.posts {
			if p.HasTag(tag) {
				posts = append(posts, p)
			}
		}
	} else {
		posts = *app.timeline.posts
		posts = posts[start:end]
	}

	d := &readPublication{
		StaticPage:  pageForReq(app, r),
		Posts:       &posts,
		CurrentPage: page,
		TotalPages:  ttlPages,
		SelTopic:    tag,
	}
	if app.cfg.App.Chorus {
		u := getUserSession(app, r)
		d.IsAdmin = u != nil && u.IsAdmin()
		d.CanInvite = canUserInvite(app.cfg, d.IsAdmin)
	}
	c, err := getReaderSection(app)
	if err != nil {
		return err
	}
	d.ContentTitle = c.Title.String
	d.Content = template.HTML(applyMarkdown([]byte(c.Content), "", app.cfg))

	err = templates["read"].ExecuteTemplate(w, "base", d)
	if err != nil {
		log.Error("Unable to render reader: %v", err)
		fmt.Fprintf(w, ":(")
	}
	return nil
}

// NextPageURL provides a full URL for the next page of collection posts
func (c *readPublication) NextPageURL(n int) string {
	return fmt.Sprintf("/read/p/%d", n+1)
}

// PrevPageURL provides a full URL for the previous page of collection posts,
// returning a /page/N result for pages >1
func (c *readPublication) PrevPageURL(n int) string {
	if n == 2 {
		// Previous page is 1; no need for /p/ prefix
		return "/read"
	}
	return fmt.Sprintf("/read/p/%d", n-1)
}

// handlePostIDRedirect handles a route where a post ID is given and redirects
// the user to the canonical post URL.
func handlePostIDRedirect(app *App, w http.ResponseWriter, r *http.Request) error {
	vars := mux.Vars(r)
	postID := vars["post"]
	p, err := app.db.GetPost(postID, 0)
	if err != nil {
		return err
	}

	if !p.CollectionID.Valid {
		// No collection; send to normal URL
		// NOTE: not handling single user blogs here since this handler is only used for the Reader
		return impart.HTTPError{http.StatusFound, app.cfg.App.Host + "/" + postID + ".md"}
	}

	c, err := app.db.GetCollectionBy("id = ?", fmt.Sprintf("%d", p.CollectionID.Int64))
	if err != nil {
		return err
	}
	c.hostName = app.cfg.App.Host

	// Retrieve collection information and send user to canonical URL
	return impart.HTTPError{http.StatusFound, c.CanonicalURL() + p.Slug.String}
}

func viewLocalTimelineFeed(app *App, w http.ResponseWriter, req *http.Request) error {
	if !app.cfg.App.LocalTimeline {
		return impart.HTTPError{http.StatusNotFound, "Page doesn't exist."}
	}

	updateTimelineCache(app.timeline, false)

	feed := &Feed{
		Title:       app.cfg.App.SiteName + " Reader",
		Link:        &Link{Href: app.cfg.App.Host},
		Description: "Read the latest posts from " + app.cfg.App.SiteName + ".",
		Created:     time.Now(),
	}

	c := 0
	var title, permalink, author string
	for _, p := range *app.timeline.posts {
		if c == tlFeedLimit {
			break
		}

		title = p.PlainDisplayTitle()
		permalink = p.CanonicalURL(app.cfg.App.Host)
		if p.Collection != nil {
			author = p.Collection.Title
		} else {
			author = "Anonymous"
			permalink += ".md"
		}
		i := &Item{
			Id:          app.cfg.App.Host + "/read/a/" + p.ID,
			Title:       title,
			Link:        &Link{Href: permalink},
			Description: "<![CDATA[" + stripmd.Strip(p.Content) + "]]>",
			Content:     applyMarkdown([]byte(p.Content), "", app.cfg),
			Author:      &Author{author, ""},
			Created:     p.Created,
			Updated:     p.Updated,
		}
		feed.Items = append(feed.Items, i)
		c++
	}

	rss, err := feed.ToRss()
	if err != nil {
		return err
	}

	fmt.Fprint(w, rss)
	return nil
}
