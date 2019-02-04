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
	"fmt"
	. "github.com/gorilla/feeds"
	"github.com/gorilla/mux"
	stripmd "github.com/writeas/go-strip-markdown"
	"github.com/writeas/impart"
	"github.com/writeas/web-core/log"
	"github.com/writeas/web-core/memo"
	"github.com/writeas/writefreely/page"
	"html/template"
	"math"
	"net/http"
	"strconv"
	"time"
)

const (
	tlFeedLimit      = 100
	tlAPIPageLimit   = 10
	tlMaxAuthorPosts = 5
	tlPostsPerPage   = 16
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
}

func initLocalTimeline(app *app) {
	app.timeline = &localTimeline{
		postsPerPage: tlPostsPerPage,
		m:            memo.New(app.db.FetchPublicPosts, 10*time.Minute),
	}
}

// satisfies memo.Func
func (db *datastore) FetchPublicPosts() (interface{}, error) {
	// Finds all public posts and posts in a public collection published during the owner's active subscription period and within the last 3 months
	rows, err := db.Query(`SELECT p.id, alias, c.title, p.slug, p.title, p.content, p.text_appearance, p.language, p.rtl, p.created, p.updated
	FROM collections c
	LEFT JOIN posts p ON p.collection_id = c.id
	WHERE c.privacy = 1 AND (p.created >= ` + db.dateSub(3, "month") + ` AND p.created <= ` + db.now() + ` AND pinned_position IS NULL)
	ORDER BY p.created DESC`)
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
		p.HTMLContent = template.HTML(applyMarkdown([]byte(p.Content), ""))
		fp := p.processPost()
		if isCollectionPost {
			fp.Collection = &CollectionObj{Collection: *c}
		}

		posts = append(posts, fp)
		ap[c.Alias]++
	}

	return posts, nil
}

func viewLocalTimelineAPI(app *app, w http.ResponseWriter, r *http.Request) error {
	updateTimelineCache(app.timeline)

	skip, _ := strconv.Atoi(r.FormValue("skip"))

	posts := []PublicPost{}
	for i := skip; i < skip+tlAPIPageLimit && i < len(*app.timeline.posts); i++ {
		posts = append(posts, (*app.timeline.posts)[i])
	}

	return impart.WriteSuccess(w, posts, http.StatusOK)
}

func viewLocalTimeline(app *app, w http.ResponseWriter, r *http.Request) error {
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

func updateTimelineCache(tl *localTimeline) {
	// Fetch posts if enough time has passed since last cache
	if tl.posts == nil || tl.m.Invalidate() {
		log.Info("[READ] Updating post cache")
		var err error
		var postsInterfaces interface{}
		postsInterfaces, err = tl.m.Get()
		if err != nil {
			log.Error("[READ] Unable to cache posts: %v", err)
		} else {
			castPosts := postsInterfaces.([]PublicPost)
			tl.posts = &castPosts
		}
	}
}

func showLocalTimeline(app *app, w http.ResponseWriter, r *http.Request, page int, author, tag string) error {
	updateTimelineCache(app.timeline)

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
		pageForReq(app, r),
		&posts,
		page,
		ttlPages,
	}

	err := templates["read"].ExecuteTemplate(w, "base", d)
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
func handlePostIDRedirect(app *app, w http.ResponseWriter, r *http.Request) error {
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

	// Retrieve collection information and send user to canonical URL
	return impart.HTTPError{http.StatusFound, c.CanonicalURL() + p.Slug.String}
}

func viewLocalTimelineFeed(app *app, w http.ResponseWriter, req *http.Request) error {
	if !app.cfg.App.LocalTimeline {
		return impart.HTTPError{http.StatusNotFound, "Page doesn't exist."}
	}

	updateTimelineCache(app.timeline)

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
		permalink = p.CanonicalURL()
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
			Content:     applyMarkdown([]byte(p.Content), ""),
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
