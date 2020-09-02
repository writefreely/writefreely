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
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/guregu/null"
	"github.com/guregu/null/zero"
	"github.com/kylemcc/twitter-text-go/extract"
	"github.com/microcosm-cc/bluemonday"
	stripmd "github.com/writeas/go-strip-markdown"
	"github.com/writeas/impart"
	"github.com/writeas/monday"
	"github.com/writeas/slug"
	"github.com/writeas/web-core/activitystreams"
	"github.com/writeas/web-core/bots"
	"github.com/writeas/web-core/converter"
	"github.com/writeas/web-core/i18n"
	"github.com/writeas/web-core/log"
	"github.com/writeas/web-core/tags"
	"github.com/writeas/writefreely/page"
	"github.com/writeas/writefreely/parse"
)

const (
	// Post ID length bounds
	minIDLen      = 10
	maxIDLen      = 10
	userPostIDLen = 10
	postIDLen     = 10

	postMetaDateFormat = "2006-01-02 15:04:05"
)

type (
	AnonymousPost struct {
		ID          string
		Content     string
		HTMLContent template.HTML
		Font        string
		Language    string
		Direction   string
		Title       string
		GenTitle    string
		Description string
		Author      string
		Views       int64
		Images      []string
		IsPlainText bool
		IsCode      bool
		IsLinkable  bool
	}

	AuthenticatedPost struct {
		ID  string `json:"id" schema:"id"`
		Web bool   `json:"web" schema:"web"`
		*SubmittedPost
	}

	// SubmittedPost represents a post supplied by a client for publishing or
	// updating. Since Title and Content can be updated to "", they are
	// pointers that can be easily tested to detect changes.
	SubmittedPost struct {
		Slug     *string                  `json:"slug" schema:"slug"`
		Title    *string                  `json:"title" schema:"title"`
		Content  *string                  `json:"body" schema:"body"`
		Font     string                   `json:"font" schema:"font"`
		IsRTL    converter.NullJSONBool   `json:"rtl" schema:"rtl"`
		Language converter.NullJSONString `json:"lang" schema:"lang"`
		Created  *string                  `json:"created" schema:"created"`
	}

	// Post represents a post as found in the database.
	Post struct {
		ID             string        `db:"id" json:"id"`
		Slug           null.String   `db:"slug" json:"slug,omitempty"`
		Font           string        `db:"text_appearance" json:"appearance"`
		Language       zero.String   `db:"language" json:"language"`
		RTL            zero.Bool     `db:"rtl" json:"rtl"`
		Privacy        int64         `db:"privacy" json:"-"`
		OwnerID        null.Int      `db:"owner_id" json:"-"`
		CollectionID   null.Int      `db:"collection_id" json:"-"`
		PinnedPosition null.Int      `db:"pinned_position" json:"-"`
		Created        time.Time     `db:"created" json:"created"`
		Updated        time.Time     `db:"updated" json:"updated"`
		ViewCount      int64         `db:"view_count" json:"-"`
		Title          zero.String   `db:"title" json:"title"`
		HTMLTitle      template.HTML `db:"title" json:"-"`
		Content        string        `db:"content" json:"body"`
		HTMLContent    template.HTML `db:"content" json:"-"`
		HTMLExcerpt    template.HTML `db:"content" json:"-"`
		Tags           []string      `json:"tags"`
		Images         []string      `json:"images,omitempty"`

		OwnerName string `json:"owner,omitempty"`
	}

	// PublicPost holds properties for a publicly returned post, i.e. a post in
	// a context where the viewer may not be the owner. As such, sensitive
	// metadata for the post is hidden and properties supporting the display of
	// the post are added.
	PublicPost struct {
		*Post
		IsSubdomain bool           `json:"-"`
		IsTopLevel  bool           `json:"-"`
		DisplayDate string         `json:"-"`
		Views       int64          `json:"views"`
		Owner       *PublicUser    `json:"-"`
		IsOwner     bool           `json:"-"`
		Collection  *CollectionObj `json:"collection,omitempty"`
	}

	RawPost struct {
		Id, Slug     string
		Title        string
		Content      string
		Views        int64
		Font         string
		Created      time.Time
		Updated      time.Time
		IsRTL        sql.NullBool
		Language     sql.NullString
		OwnerID      int64
		CollectionID sql.NullInt64

		Found bool
		Gone  bool
	}

	AnonymousAuthPost struct {
		ID    string `json:"id"`
		Token string `json:"token"`
	}
	ClaimPostRequest struct {
		*AnonymousAuthPost
		CollectionAlias  string `json:"collection"`
		CreateCollection bool   `json:"create_collection"`

		// Generated properties
		Slug string `json:"-"`
	}
	ClaimPostResult struct {
		ID           string      `json:"id,omitempty"`
		Code         int         `json:"code,omitempty"`
		ErrorMessage string      `json:"error_msg,omitempty"`
		Post         *PublicPost `json:"post,omitempty"`
	}
)

func (p *Post) Direction() string {
	if p.RTL.Valid {
		if p.RTL.Bool {
			return "rtl"
		}
		return "ltr"
	}
	return "auto"
}

// DisplayTitle dynamically generates a title from the Post's contents if it
// doesn't already have an explicit title.
func (p *Post) DisplayTitle() string {
	if p.Title.String != "" {
		return p.Title.String
	}
	t := friendlyPostTitle(p.Content, p.ID)
	return t
}

// PlainDisplayTitle dynamically generates a title from the Post's contents if it
// doesn't already have an explicit title.
func (p *Post) PlainDisplayTitle() string {
	if t := stripmd.Strip(p.DisplayTitle()); t != "" {
		return t
	}
	return p.ID
}

// FormattedDisplayTitle dynamically generates a title from the Post's contents if it
// doesn't already have an explicit title.
func (p *Post) FormattedDisplayTitle() template.HTML {
	if p.HTMLTitle != "" {
		return p.HTMLTitle
	}
	return template.HTML(p.DisplayTitle())
}

// Summary gives a shortened summary of the post based on the post's title,
// especially for display in a longer list of posts. It extracts a summary for
// posts in the Title\n\nBody format, returning nothing if the entire was short
// enough that the extracted title == extracted summary.
func (p Post) Summary() string {
	if p.Content == "" {
		return ""
	}
	// Strip out HTML
	p.Content = bluemonday.StrictPolicy().Sanitize(p.Content)
	// and Markdown
	p.Content = stripmd.Strip(p.Content)

	title := p.Title.String
	var desc string
	if title == "" {
		// No title, so generate one
		title = friendlyPostTitle(p.Content, p.ID)
		desc = postDescription(p.Content, title, p.ID)
		if desc == title {
			return ""
		}
		return desc
	}

	return shortPostDescription(p.Content)
}

func (p Post) SummaryHTML() template.HTML {
	return template.HTML(p.Summary())
}

// Excerpt shows any text that comes before a (more) tag.
// TODO: use HTMLExcerpt in templates instead of this method
func (p *Post) Excerpt() template.HTML {
	return p.HTMLExcerpt
}

func (p *Post) CreatedDate() string {
	return p.Created.Format("2006-01-02")
}

func (p *Post) Created8601() string {
	return p.Created.Format("2006-01-02T15:04:05Z")
}

func (p *Post) IsScheduled() bool {
	return p.Created.After(time.Now())
}

func (p *Post) HasTag(tag string) bool {
	// Regexp looks for tag and has a non-capturing group at the end looking
	// for the end of the word.
	// Assisted by: https://stackoverflow.com/a/35192941/1549194
	hasTag, _ := regexp.MatchString("#"+tag+`(?:[[:punct:]]|\s|\z)`, p.Content)
	return hasTag
}

func (p *Post) HasTitleLink() bool {
	if p.Title.String == "" {
		return false
	}
	hasLink, _ := regexp.MatchString(`([^!]+|^)\[.+\]\(.+\)`, p.Title.String)
	return hasLink
}

func handleViewPost(app *App, w http.ResponseWriter, r *http.Request) error {
	vars := mux.Vars(r)
	friendlyID := vars["post"]

	// NOTE: until this is done better, be sure to keep this in parity with
	// isRaw() and viewCollectionPost()
	isJSON := strings.HasSuffix(friendlyID, ".json")
	isXML := strings.HasSuffix(friendlyID, ".xml")
	isCSS := strings.HasSuffix(friendlyID, ".css")
	isMarkdown := strings.HasSuffix(friendlyID, ".md")
	isRaw := strings.HasSuffix(friendlyID, ".txt") || isJSON || isXML || isCSS || isMarkdown

	// Display reserved page if that is requested resource
	if t, ok := pages[r.URL.Path[1:]+".tmpl"]; ok {
		return handleTemplatedPage(app, w, r, t)
	} else if (strings.Contains(r.URL.Path, ".") && !isRaw && !isMarkdown) || r.URL.Path == "/robots.txt" || r.URL.Path == "/manifest.json" {
		// Serve static file
		app.shttp.ServeHTTP(w, r)
		return nil
	}

	// Display collection if this is a collection
	c, _ := app.db.GetCollection(friendlyID)
	if c != nil {
		return impart.HTTPError{http.StatusMovedPermanently, fmt.Sprintf("/%s/", friendlyID)}
	}

	// Normalize the URL, redirecting user to consistent post URL
	if friendlyID != strings.ToLower(friendlyID) {
		return impart.HTTPError{http.StatusMovedPermanently, fmt.Sprintf("/%s", strings.ToLower(friendlyID))}
	}

	ext := ""
	if isRaw {
		parts := strings.Split(friendlyID, ".")
		friendlyID = parts[0]
		if len(parts) > 1 {
			ext = "." + parts[1]
		}
	}

	var ownerID sql.NullInt64
	var title string
	var content string
	var font string
	var language []byte
	var rtl []byte
	var views int64
	var post *AnonymousPost
	var found bool
	var gone bool

	fixedID := slug.Make(friendlyID)
	if fixedID != friendlyID {
		return impart.HTTPError{http.StatusFound, fmt.Sprintf("/%s%s", fixedID, ext)}
	}

	err := app.db.QueryRow(fmt.Sprintf("SELECT owner_id, title, content, text_appearance, view_count, language, rtl FROM posts WHERE id = ?"), friendlyID).Scan(&ownerID, &title, &content, &font, &views, &language, &rtl)
	switch {
	case err == sql.ErrNoRows:
		found = false

		// Output the error in the correct format
		if isJSON {
			content = "{\"error\": \"Post not found.\"}"
		} else if isRaw {
			content = "Post not found."
		} else {
			return ErrPostNotFound
		}
	case err != nil:
		found = false

		log.Error("Post loading err: %s\n", err)
		return ErrInternalGeneral
	default:
		found = true

		var d string
		if len(rtl) == 0 {
			d = "auto"
		} else if rtl[0] == 49 {
			// TODO: find a cleaner way to get this (possibly NULL) value
			d = "rtl"
		} else {
			d = "ltr"
		}
		generatedTitle := friendlyPostTitle(content, friendlyID)
		sanitizedContent := content
		if font != "code" {
			sanitizedContent = template.HTMLEscapeString(content)
		}
		var desc string
		if title == "" {
			desc = postDescription(content, title, friendlyID)
		} else {
			desc = shortPostDescription(content)
		}
		post = &AnonymousPost{
			ID:          friendlyID,
			Content:     sanitizedContent,
			Title:       title,
			GenTitle:    generatedTitle,
			Description: desc,
			Author:      "",
			Font:        font,
			IsPlainText: isRaw,
			IsCode:      font == "code",
			IsLinkable:  font != "code",
			Views:       views,
			Language:    string(language),
			Direction:   d,
		}
		if !isRaw {
			post.HTMLContent = template.HTML(applyMarkdown([]byte(content), "", app.cfg))
			post.Images = extractImages(post.Content)
		}
	}

	var silenced bool
	if found {
		silenced, err = app.db.IsUserSilenced(ownerID.Int64)
		if err != nil {
			log.Error("view post: %v", err)
		}
	}

	// Check if post has been unpublished
	if content == "" {
		gone = true

		if isJSON {
			content = "{\"error\": \"Post was unpublished.\"}"
		} else if isCSS {
			content = ""
		} else if isRaw {
			content = "Post was unpublished."
		} else {
			return ErrPostUnpublished
		}
	}

	var u = &User{}
	if isRaw {
		contentType := "text/plain"
		if isJSON {
			contentType = "application/json"
		} else if isCSS {
			contentType = "text/css"
		} else if isXML {
			contentType = "application/xml"
		} else if isMarkdown {
			contentType = "text/markdown"
		}
		w.Header().Set("Content-Type", fmt.Sprintf("%s; charset=utf-8", contentType))
		if isMarkdown && post.Title != "" {
			fmt.Fprintf(w, "%s\n", post.Title)
			for i := 1; i <= len(post.Title); i++ {
				fmt.Fprintf(w, "=")
			}
			fmt.Fprintf(w, "\n\n")
		}
		fmt.Fprint(w, content)

		if !found {
			return ErrPostNotFound
		} else if gone {
			return ErrPostUnpublished
		}
	} else {
		var err error
		page := struct {
			*AnonymousPost
			page.StaticPage
			Username string
			IsOwner  bool
			SiteURL  string
			Silenced bool
		}{
			AnonymousPost: post,
			StaticPage:    pageForReq(app, r),
			SiteURL:       app.cfg.App.Host,
		}
		if u = getUserSession(app, r); u != nil {
			page.Username = u.Username
			page.IsOwner = ownerID.Valid && ownerID.Int64 == u.ID
		}

		if !page.IsOwner && silenced {
			return ErrPostNotFound
		}
		page.Silenced = silenced
		err = templates["post"].ExecuteTemplate(w, "post", page)
		if err != nil {
			log.Error("Post template execute error: %v", err)
		}
	}

	go func() {
		if u != nil && ownerID.Valid && ownerID.Int64 == u.ID {
			// Post is owned by someone; skip view increment since that person is viewing this post.
			return
		}
		// Update stats for non-raw post views
		if !isRaw && r.Method != "HEAD" && !bots.IsBot(r.UserAgent()) {
			_, err := app.db.Exec("UPDATE posts SET view_count = view_count + 1 WHERE id = ?", friendlyID)
			if err != nil {
				log.Error("Unable to update posts count: %v", err)
			}
		}
	}()

	return nil
}

// API v2 funcs
// newPost creates a new post with or without an owning Collection.
//
// Endpoints:
//   /posts
//   /posts?collection={alias}
// ? /collections/{alias}/posts
func newPost(app *App, w http.ResponseWriter, r *http.Request) error {
	reqJSON := IsJSON(r)
	vars := mux.Vars(r)
	collAlias := vars["alias"]
	if collAlias == "" {
		collAlias = r.FormValue("collection")
	}
	accessToken := r.Header.Get("Authorization")
	if accessToken == "" {
		// TODO: remove this
		accessToken = r.FormValue("access_token")
	}

	// FIXME: determine web submission with Content-Type header
	var u *User
	var userID int64 = -1
	var username string
	if accessToken == "" {
		u = getUserSession(app, r)
		if u != nil {
			userID = u.ID
			username = u.Username
		}
	} else {
		userID = app.db.GetUserID(accessToken)
	}
	silenced, err := app.db.IsUserSilenced(userID)
	if err != nil {
		log.Error("new post: %v", err)
	}
	if silenced {
		return ErrUserSilenced
	}

	if userID == -1 {
		return ErrNotLoggedIn
	}

	if accessToken == "" && u == nil && collAlias != "" {
		return impart.HTTPError{http.StatusBadRequest, "Parameter `access_token` required."}
	}

	// Get post data
	var p *SubmittedPost
	if reqJSON {
		decoder := json.NewDecoder(r.Body)
		err = decoder.Decode(&p)
		if err != nil {
			log.Error("Couldn't parse new post JSON request: %v\n", err)
			return ErrBadJSON
		}
		if p.Title == nil {
			t := ""
			p.Title = &t
		}
		if strings.TrimSpace(*(p.Content)) == "" {
			return ErrNoPublishableContent
		}
	} else {
		post := r.FormValue("body")
		appearance := r.FormValue("font")
		title := r.FormValue("title")
		rtlValue := r.FormValue("rtl")
		langValue := r.FormValue("lang")
		if strings.TrimSpace(post) == "" {
			return ErrNoPublishableContent
		}

		var isRTL, rtlValid bool
		if rtlValue == "auto" && langValue != "" {
			isRTL = i18n.LangIsRTL(langValue)
			rtlValid = true
		} else {
			isRTL = rtlValue == "true"
			rtlValid = rtlValue != "" && langValue != ""
		}

		// Create a new post
		p = &SubmittedPost{
			Title:    &title,
			Content:  &post,
			Font:     appearance,
			IsRTL:    converter.NullJSONBool{sql.NullBool{Bool: isRTL, Valid: rtlValid}},
			Language: converter.NullJSONString{sql.NullString{String: langValue, Valid: langValue != ""}},
		}
	}
	if !p.isFontValid() {
		p.Font = "norm"
	}

	var newPost *PublicPost = &PublicPost{}
	var coll *Collection
	if accessToken != "" {
		newPost, err = app.db.CreateOwnedPost(p, accessToken, collAlias, app.cfg.App.Host)
	} else {
		//return ErrNotLoggedIn
		// TODO: verify user is logged in
		var collID int64
		if collAlias != "" {
			coll, err = app.db.GetCollection(collAlias)
			if err != nil {
				return err
			}
			coll.hostName = app.cfg.App.Host
			if coll.OwnerID != u.ID {
				return ErrForbiddenCollection
			}
			collID = coll.ID
		}
		// TODO: return PublicPost from createPost
		newPost.Post, err = app.db.CreatePost(userID, collID, p)
	}
	if err != nil {
		return err
	}
	if coll != nil {
		coll.ForPublic()
		newPost.Collection = &CollectionObj{Collection: *coll}
	}

	newPost.extractData()
	newPost.OwnerName = username

	// Write success now
	response := impart.WriteSuccess(w, newPost, http.StatusCreated)

	if newPost.Collection != nil && !app.cfg.App.Private && app.cfg.App.Federation && !newPost.Created.After(time.Now()) {
		go federatePost(app, newPost, newPost.Collection.ID, false)
	}

	return response
}

func existingPost(app *App, w http.ResponseWriter, r *http.Request) error {
	reqJSON := IsJSON(r)
	vars := mux.Vars(r)
	postID := vars["post"]

	p := AuthenticatedPost{ID: postID}
	var err error

	if reqJSON {
		// Decode JSON request
		decoder := json.NewDecoder(r.Body)
		err = decoder.Decode(&p)
		if err != nil {
			log.Error("Couldn't parse post update JSON request: %v\n", err)
			return ErrBadJSON
		}
	} else {
		err = r.ParseForm()
		if err != nil {
			log.Error("Couldn't parse post update form request: %v\n", err)
			return ErrBadFormData
		}

		// Can't decode to a nil SubmittedPost property, so create instance now
		p.SubmittedPost = &SubmittedPost{}
		err = app.formDecoder.Decode(&p, r.PostForm)
		if err != nil {
			log.Error("Couldn't decode post update form request: %v\n", err)
			return ErrBadFormData
		}
	}

	if p.Web {
		p.IsRTL.Valid = true
	}

	if p.SubmittedPost == nil {
		return ErrPostNoUpdatableVals
	}

	// Ensure an access token was given
	accessToken := r.Header.Get("Authorization")
	// Get user's cookie session if there's no token
	var u *User
	//var username string
	if accessToken == "" {
		u = getUserSession(app, r)
		if u != nil {
			//username = u.Username
		}
	}
	if u == nil && accessToken == "" {
		return ErrNoAccessToken
	}

	// Get user ID from current session or given access token, if one was given.
	var userID int64
	if u != nil {
		userID = u.ID
	} else if accessToken != "" {
		userID, err = AuthenticateUser(app.db, accessToken)
		if err != nil {
			return err
		}
	}

	silenced, err := app.db.IsUserSilenced(userID)
	if err != nil {
		log.Error("existing post: %v", err)
	}
	if silenced {
		return ErrUserSilenced
	}

	// Modify post struct
	p.ID = postID

	err = app.db.UpdateOwnedPost(&p, userID)
	if err != nil {
		if reqJSON {
			return err
		}

		if err, ok := err.(impart.HTTPError); ok {
			addSessionFlash(app, w, r, err.Message, nil)
		} else {
			addSessionFlash(app, w, r, err.Error(), nil)
		}
	}

	var pRes *PublicPost
	pRes, err = app.db.GetPost(p.ID, 0)
	if reqJSON {
		if err != nil {
			return err
		}
		pRes.extractData()
	}

	if pRes.CollectionID.Valid {
		coll, err := app.db.GetCollectionBy("id = ?", pRes.CollectionID.Int64)
		if err == nil && !app.cfg.App.Private && app.cfg.App.Federation {
			coll.hostName = app.cfg.App.Host
			pRes.Collection = &CollectionObj{Collection: *coll}
			go federatePost(app, pRes, pRes.Collection.ID, true)
		}
	}

	// Write success now
	if reqJSON {
		return impart.WriteSuccess(w, pRes, http.StatusOK)
	}

	addSessionFlash(app, w, r, "Changes saved.", nil)
	collectionAlias := vars["alias"]
	redirect := "/" + postID + "/meta"
	if collectionAlias != "" {
		collPre := "/" + collectionAlias
		if app.cfg.App.SingleUser {
			collPre = ""
		}
		redirect = collPre + "/" + pRes.Slug.String + "/edit/meta"
	} else {
		if app.cfg.App.SingleUser {
			redirect = "/d" + redirect
		}
	}
	w.Header().Set("Location", redirect)
	w.WriteHeader(http.StatusFound)

	return nil
}

func deletePost(app *App, w http.ResponseWriter, r *http.Request) error {
	vars := mux.Vars(r)
	friendlyID := vars["post"]
	editToken := r.FormValue("token")

	var ownerID int64
	var u *User
	accessToken := r.Header.Get("Authorization")
	if accessToken == "" && editToken == "" {
		u = getUserSession(app, r)
		if u == nil {
			return ErrNoAccessToken
		}
	}

	var res sql.Result
	var t *sql.Tx
	var err error
	var collID sql.NullInt64
	var coll *Collection
	var pp *PublicPost
	if editToken != "" {
		// TODO: SELECT owner_id, as well, and return appropriate error if NULL instead of running two queries
		var dummy int64
		err = app.db.QueryRow("SELECT 1 FROM posts WHERE id = ?", friendlyID).Scan(&dummy)
		switch {
		case err == sql.ErrNoRows:
			return impart.HTTPError{http.StatusNotFound, "Post not found."}
		}
		err = app.db.QueryRow("SELECT 1 FROM posts WHERE id = ? AND owner_id IS NULL", friendlyID).Scan(&dummy)
		switch {
		case err == sql.ErrNoRows:
			// Post already has an owner. This could provide a bad experience
			// for the user, but it's more important to ensure data isn't lost
			// unexpectedly. So prevent deletion via token.
			return impart.HTTPError{http.StatusConflict, "This post belongs to some user (hopefully yours). Please log in and delete it from that user's account."}
		}
		res, err = app.db.Exec("DELETE FROM posts WHERE id = ? AND modify_token = ? AND owner_id IS NULL", friendlyID, editToken)
	} else if accessToken != "" || u != nil {
		// Caller provided some way to authenticate; assume caller expects the
		// post to be deleted based on a specific post owner, thus we should
		// return corresponding errors.
		if accessToken != "" {
			ownerID = app.db.GetUserID(accessToken)
			if ownerID == -1 {
				return ErrBadAccessToken
			}
		} else {
			ownerID = u.ID
		}

		// TODO: don't make two queries
		var realOwnerID sql.NullInt64
		err = app.db.QueryRow("SELECT collection_id, owner_id FROM posts WHERE id = ?", friendlyID).Scan(&collID, &realOwnerID)
		if err != nil {
			return err
		}
		if !collID.Valid {
			// There's no collection; simply delete the post
			res, err = app.db.Exec("DELETE FROM posts WHERE id = ? AND owner_id = ?", friendlyID, ownerID)
		} else {
			// Post belongs to a collection; do any additional clean up
			coll, err = app.db.GetCollectionBy("id = ?", collID.Int64)
			if err != nil {
				log.Error("Unable to get collection: %v", err)
				return err
			}
			if app.cfg.App.Federation {
				// First fetch full post for federation
				pp, err = app.db.GetOwnedPost(friendlyID, ownerID)
				if err != nil {
					log.Error("Unable to get owned post: %v", err)
					return err
				}
				collObj := &CollectionObj{Collection: *coll}
				pp.Collection = collObj
			}

			t, err = app.db.Begin()
			if err != nil {
				log.Error("No begin: %v", err)
				return err
			}
			res, err = t.Exec("DELETE FROM posts WHERE id = ? AND owner_id = ?", friendlyID, ownerID)
		}
	} else {
		return impart.HTTPError{http.StatusBadRequest, "No authenticated user or post token given."}
	}
	if err != nil {
		return err
	}

	affected, err := res.RowsAffected()
	if err != nil {
		if t != nil {
			t.Rollback()
			log.Error("Rows affected err! Rolling back")
		}
		return err
	} else if affected == 0 {
		if t != nil {
			t.Rollback()
			log.Error("No rows affected! Rolling back")
		}
		return impart.HTTPError{http.StatusForbidden, "Post not found, or you're not the owner."}
	}
	if t != nil {
		t.Commit()
	}
	if coll != nil && !app.cfg.App.Private && app.cfg.App.Federation {
		go deleteFederatedPost(app, pp, collID.Int64)
	}

	return impart.HTTPError{Status: http.StatusNoContent}
}

// addPost associates a post with the authenticated user.
func addPost(app *App, w http.ResponseWriter, r *http.Request) error {
	var ownerID int64

	// Authenticate user
	at := r.Header.Get("Authorization")
	if at != "" {
		ownerID = app.db.GetUserID(at)
		if ownerID == -1 {
			return ErrBadAccessToken
		}
	} else {
		u := getUserSession(app, r)
		if u == nil {
			return ErrNotLoggedIn
		}
		ownerID = u.ID
	}

	silenced, err := app.db.IsUserSilenced(ownerID)
	if err != nil {
		log.Error("add post: %v", err)
	}
	if silenced {
		return ErrUserSilenced
	}

	// Parse claimed posts in format:
	// [{"id": "...", "token": "..."}]
	var claims *[]ClaimPostRequest
	decoder := json.NewDecoder(r.Body)
	err = decoder.Decode(&claims)
	if err != nil {
		return ErrBadJSONArray
	}

	vars := mux.Vars(r)
	collAlias := vars["alias"]

	// Update all given posts
	res, err := app.db.ClaimPosts(app.cfg, ownerID, collAlias, claims)
	if err != nil {
		return err
	}

	if !app.cfg.App.Private && app.cfg.App.Federation {
		for _, pRes := range *res {
			if pRes.Code != http.StatusOK {
				continue
			}
			if !pRes.Post.Created.After(time.Now()) {
				pRes.Post.Collection.hostName = app.cfg.App.Host
				go federatePost(app, pRes.Post, pRes.Post.Collection.ID, false)
			}
		}
	}
	return impart.WriteSuccess(w, res, http.StatusOK)
}

func dispersePost(app *App, w http.ResponseWriter, r *http.Request) error {
	var ownerID int64

	// Authenticate user
	at := r.Header.Get("Authorization")
	if at != "" {
		ownerID = app.db.GetUserID(at)
		if ownerID == -1 {
			return ErrBadAccessToken
		}
	} else {
		u := getUserSession(app, r)
		if u == nil {
			return ErrNotLoggedIn
		}
		ownerID = u.ID
	}

	// Parse posts in format:
	// ["..."]
	var postIDs []string
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&postIDs)
	if err != nil {
		return ErrBadJSONArray
	}

	// Update all given posts
	res, err := app.db.DispersePosts(ownerID, postIDs)
	if err != nil {
		return err
	}
	return impart.WriteSuccess(w, res, http.StatusOK)
}

type (
	PinPostResult struct {
		ID           string `json:"id,omitempty"`
		Code         int    `json:"code,omitempty"`
		ErrorMessage string `json:"error_msg,omitempty"`
	}
)

// pinPost pins a post to a blog
func pinPost(app *App, w http.ResponseWriter, r *http.Request) error {
	var userID int64

	// Authenticate user
	at := r.Header.Get("Authorization")
	if at != "" {
		userID = app.db.GetUserID(at)
		if userID == -1 {
			return ErrBadAccessToken
		}
	} else {
		u := getUserSession(app, r)
		if u == nil {
			return ErrNotLoggedIn
		}
		userID = u.ID
	}

	silenced, err := app.db.IsUserSilenced(userID)
	if err != nil {
		log.Error("pin post: %v", err)
	}
	if silenced {
		return ErrUserSilenced
	}

	// Parse request
	var posts []struct {
		ID       string `json:"id"`
		Position int64  `json:"position"`
	}
	decoder := json.NewDecoder(r.Body)
	err = decoder.Decode(&posts)
	if err != nil {
		return ErrBadJSONArray
	}

	// Validate data
	vars := mux.Vars(r)
	collAlias := vars["alias"]

	coll, err := app.db.GetCollection(collAlias)
	if err != nil {
		return err
	}
	if coll.OwnerID != userID {
		return ErrForbiddenCollection
	}

	// Do (un)pinning
	isPinning := r.URL.Path[strings.LastIndex(r.URL.Path, "/"):] == "/pin"
	res := []PinPostResult{}
	for _, p := range posts {
		err = app.db.UpdatePostPinState(isPinning, p.ID, coll.ID, userID, p.Position)
		ppr := PinPostResult{ID: p.ID}
		if err != nil {
			ppr.Code = http.StatusInternalServerError
			// TODO: set error messsage
		} else {
			ppr.Code = http.StatusOK
		}
		res = append(res, ppr)
	}
	return impart.WriteSuccess(w, res, http.StatusOK)
}

func fetchPost(app *App, w http.ResponseWriter, r *http.Request) error {
	var collID int64
	var coll *Collection
	var err error
	vars := mux.Vars(r)
	if collAlias := vars["alias"]; collAlias != "" {
		// Fetch collection information, since an alias is provided
		coll, err = app.db.GetCollection(collAlias)
		if err != nil {
			return err
		}
		collID = coll.ID
	}

	p, err := app.db.GetPost(vars["post"], collID)
	if err != nil {
		return err
	}
	if coll == nil && p.CollectionID.Valid {
		// Collection post is getting fetched by post ID, not coll alias + post slug, so get coll info now.
		coll, err = app.db.GetCollectionByID(p.CollectionID.Int64)
		if err != nil {
			return err
		}
	}
	if coll != nil {
		coll.hostName = app.cfg.App.Host
		_, err = apiCheckCollectionPermissions(app, r, coll)
		if err != nil {
			return err
		}
	}

	silenced, err := app.db.IsUserSilenced(p.OwnerID.Int64)
	if err != nil {
		log.Error("fetch post: %v", err)
	}
	if silenced {
		return ErrPostNotFound
	}

	p.extractData()

	accept := r.Header.Get("Accept")
	if strings.Contains(accept, "application/activity+json") {
		if coll == nil {
			// This is a draft post; 404 for now
			// TODO: return ActivityObject
			return impart.HTTPError{http.StatusNotFound, ""}
		}

		p.Collection = &CollectionObj{Collection: *coll}
		po := p.ActivityObject(app)
		po.Context = []interface{}{activitystreams.Namespace}
		setCacheControl(w, apCacheTime)
		return impart.RenderActivityJSON(w, po, http.StatusOK)
	}

	return impart.WriteSuccess(w, p, http.StatusOK)
}

func fetchPostProperty(app *App, w http.ResponseWriter, r *http.Request) error {
	vars := mux.Vars(r)
	p, err := app.db.GetPostProperty(vars["post"], 0, vars["property"])
	if err != nil {
		return err
	}

	return impart.WriteSuccess(w, p, http.StatusOK)
}

func (p *Post) processPost() PublicPost {
	res := &PublicPost{Post: p, Views: 0}
	res.Views = p.ViewCount
	// TODO: move to own function
	loc := monday.FuzzyLocale(p.Language.String)
	res.DisplayDate = monday.Format(p.Created, monday.LongFormatsByLocale[loc], loc)

	return *res
}

func (p *PublicPost) CanonicalURL(hostName string) string {
	if p.Collection == nil || p.Collection.Alias == "" {
		return hostName + "/" + p.ID
	}
	return p.Collection.CanonicalURL() + p.Slug.String
}

func (p *PublicPost) ActivityObject(app *App) *activitystreams.Object {
	cfg := app.cfg
	var o *activitystreams.Object
	if strings.Index(p.Content, "\n\n") == -1 {
		o = activitystreams.NewNoteObject()
	} else {
		o = activitystreams.NewArticleObject()
	}
	o.ID = p.Collection.FederatedAPIBase() + "api/posts/" + p.ID
	o.Published = p.Created
	o.URL = p.CanonicalURL(cfg.App.Host)
	o.AttributedTo = p.Collection.FederatedAccount()
	o.CC = []string{
		p.Collection.FederatedAccount() + "/followers",
	}
	o.Name = p.DisplayTitle()
	p.augmentContent()
	if p.HTMLContent == template.HTML("") {
		p.formatContent(cfg, false)
	}
	o.Content = string(p.HTMLContent)
	if p.Language.Valid {
		o.ContentMap = map[string]string{
			p.Language.String: string(p.HTMLContent),
		}
	}
	if len(p.Tags) == 0 {
		o.Tag = []activitystreams.Tag{}
	} else {
		var tagBaseURL string
		if isSingleUser {
			tagBaseURL = p.Collection.CanonicalURL() + "tag:"
		} else {
			if cfg.App.Chorus {
				tagBaseURL = fmt.Sprintf("%s/read/t/", p.Collection.hostName)
			} else {
				tagBaseURL = fmt.Sprintf("%s/%s/tag:", p.Collection.hostName, p.Collection.Alias)
			}
		}
		for _, t := range p.Tags {
			o.Tag = append(o.Tag, activitystreams.Tag{
				Type: activitystreams.TagHashtag,
				HRef: tagBaseURL + t,
				Name: "#" + t,
			})
		}
	}
	// Find mentioned users
	mentionedUsers := make(map[string]string)

	stripper := bluemonday.StrictPolicy()
	content := stripper.Sanitize(p.Content)
	mentions := mentionReg.FindAllString(content, -1)

	for _, handle := range mentions {
		actorIRI, err := app.db.GetProfilePageFromHandle(app, handle)
		if err != nil {
			log.Info("Couldn't find user '%s' locally or remotely", handle)
			continue
		}
		mentionedUsers[handle] = actorIRI
	}

	for handle, iri := range mentionedUsers {
		o.CC = append(o.CC, iri)
		o.Tag = append(o.Tag, activitystreams.Tag{Type: "Mention", HRef: iri, Name: handle})
	}
	return o
}

// TODO: merge this into getSlugFromPost or phase it out
func getSlug(title, lang string) string {
	return getSlugFromPost("", title, lang)
}

func getSlugFromPost(title, body, lang string) string {
	if title == "" {
		title = postTitle(body, body)
	}
	title = parse.PostLede(title, false)
	// Truncate lede if needed
	title, _ = parse.TruncToWord(title, 80)
	var s string
	if lang != "" && len(lang) == 2 {
		s = slug.MakeLang(title, lang)
	} else {
		s = slug.Make(title)
	}

	// Transliteration may cause the slug to expand past the limit, so truncate again
	s, _ = parse.TruncToWord(s, 80)
	return strings.TrimFunc(s, func(r rune) bool {
		// TruncToWord doesn't respect words in a slug, since spaces are replaced
		// with hyphens. So remove any trailing hyphens.
		return r == '-'
	})
}

// isFontValid returns whether or not the submitted post's appearance is valid.
func (p *SubmittedPost) isFontValid() bool {
	validFonts := map[string]bool{
		"norm": true,
		"sans": true,
		"mono": true,
		"wrap": true,
		"code": true,
	}

	_, valid := validFonts[p.Font]
	return valid
}

func getRawPost(app *App, friendlyID string) *RawPost {
	var content, font, title string
	var isRTL sql.NullBool
	var lang sql.NullString
	var ownerID sql.NullInt64
	var created, updated time.Time

	err := app.db.QueryRow("SELECT title, content, text_appearance, language, rtl, created, updated, owner_id FROM posts WHERE id = ?", friendlyID).Scan(&title, &content, &font, &lang, &isRTL, &created, &updated, &ownerID)
	switch {
	case err == sql.ErrNoRows:
		return &RawPost{Content: "", Found: false, Gone: false}
	case err != nil:
		return &RawPost{Content: "", Found: true, Gone: false}
	}

	return &RawPost{Title: title, Content: content, Font: font, Created: created, Updated: updated, IsRTL: isRTL, Language: lang, OwnerID: ownerID.Int64, Found: true, Gone: content == ""}

}

// TODO; return a Post!
func getRawCollectionPost(app *App, slug, collAlias string) *RawPost {
	var id, title, content, font string
	var isRTL sql.NullBool
	var lang sql.NullString
	var created, updated time.Time
	var ownerID null.Int
	var views int64
	var err error

	if app.cfg.App.SingleUser {
		err = app.db.QueryRow("SELECT id, title, content, text_appearance, language, rtl, view_count, created, updated, owner_id FROM posts WHERE slug = ? AND collection_id = 1", slug).Scan(&id, &title, &content, &font, &lang, &isRTL, &views, &created, &updated, &ownerID)
	} else {
		err = app.db.QueryRow("SELECT id, title, content, text_appearance, language, rtl, view_count, created, updated, owner_id FROM posts WHERE slug = ? AND collection_id = (SELECT id FROM collections WHERE alias = ?)", slug, collAlias).Scan(&id, &title, &content, &font, &lang, &isRTL, &views, &created, &updated, &ownerID)
	}
	switch {
	case err == sql.ErrNoRows:
		return &RawPost{Content: "", Found: false, Gone: false}
	case err != nil:
		return &RawPost{Content: "", Found: true, Gone: false}
	}

	return &RawPost{
		Id:       id,
		Slug:     slug,
		Title:    title,
		Content:  content,
		Font:     font,
		Created:  created,
		Updated:  updated,
		IsRTL:    isRTL,
		Language: lang,
		OwnerID:  ownerID.Int64,
		Found:    true,
		Gone:     content == "",
		Views:    views,
	}
}

func isRaw(r *http.Request) bool {
	vars := mux.Vars(r)
	slug := vars["slug"]

	// NOTE: until this is done better, be sure to keep this in parity with
	// isRaw in viewCollectionPost() and handleViewPost()
	isJSON := strings.HasSuffix(slug, ".json")
	isXML := strings.HasSuffix(slug, ".xml")
	isMarkdown := strings.HasSuffix(slug, ".md")
	return strings.HasSuffix(slug, ".txt") || isJSON || isXML || isMarkdown
}

func viewCollectionPost(app *App, w http.ResponseWriter, r *http.Request) error {
	vars := mux.Vars(r)
	slug := vars["slug"]

	// NOTE: until this is done better, be sure to keep this in parity with
	// isRaw() and handleViewPost()
	isJSON := strings.HasSuffix(slug, ".json")
	isXML := strings.HasSuffix(slug, ".xml")
	isMarkdown := strings.HasSuffix(slug, ".md")
	isRaw := strings.HasSuffix(slug, ".txt") || isJSON || isXML || isMarkdown

	cr := &collectionReq{}
	err := processCollectionRequest(cr, vars, w, r)
	if err != nil {
		return err
	}

	// Check for hellbanned users
	u, err := checkUserForCollection(app, cr, r, true)
	if err != nil {
		return err
	}

	// Normalize the URL, redirecting user to consistent post URL
	if slug != strings.ToLower(slug) {
		loc := fmt.Sprintf("/%s", strings.ToLower(slug))
		if !app.cfg.App.SingleUser {
			loc = "/" + cr.alias + loc
		}
		return impart.HTTPError{http.StatusMovedPermanently, loc}
	}

	// Display collection if this is a collection
	var c *Collection
	if app.cfg.App.SingleUser {
		c, err = app.db.GetCollectionByID(1)
	} else {
		c, err = app.db.GetCollection(cr.alias)
	}
	if err != nil {
		if err, ok := err.(impart.HTTPError); ok {
			if err.Status == http.StatusNotFound {
				// Redirect if necessary
				newAlias := app.db.GetCollectionRedirect(cr.alias)
				if newAlias != "" {
					return impart.HTTPError{http.StatusFound, "/" + newAlias + "/" + slug}
				}
			}
		}
		return err
	}
	c.hostName = app.cfg.App.Host

	silenced, err := app.db.IsUserSilenced(c.OwnerID)
	if err != nil {
		log.Error("view collection post: %v", err)
	}

	// Check collection permissions
	if c.IsPrivate() && (u == nil || u.ID != c.OwnerID) {
		return ErrPostNotFound
	}
	if c.IsProtected() && (u == nil || u.ID != c.OwnerID) {
		if silenced {
			return ErrPostNotFound
		} else if !isAuthorizedForCollection(app, c.Alias, r) {
			return impart.HTTPError{http.StatusFound, c.CanonicalURL() + "/?g=" + slug}
		}
	}

	cr.isCollOwner = u != nil && c.OwnerID == u.ID

	if isRaw {
		slug = strings.Split(slug, ".")[0]
	}

	// Fetch extra data about the Collection
	// TODO: refactor out this logic, shared in collection.go:fetchCollection()
	coll := NewCollectionObj(c)
	owner, err := app.db.GetUserByID(coll.OwnerID)
	if err != nil {
		// Log the error and just continue
		log.Error("Error getting user for collection: %v", err)
	} else {
		coll.Owner = owner
	}

	postFound := true
	p, err := app.db.GetPost(slug, coll.ID)
	if err != nil {
		if err == ErrCollectionPageNotFound {
			postFound = false

			if slug == "feed" {
				// User tried to access blog feed without a trailing slash, and
				// there's no post with a slug "feed"
				return impart.HTTPError{http.StatusFound, c.CanonicalURL() + "/feed/"}
			}

			po := &Post{
				Slug:     null.NewString(slug, true),
				Font:     "norm",
				Language: zero.NewString("en", true),
				RTL:      zero.NewBool(false, true),
				Content: `<p class="msg">This page is missing.</p>

Are you sure it was ever here?`,
			}
			pp := po.processPost()
			p = &pp
		} else {
			return err
		}
	}
	p.IsOwner = owner != nil && p.OwnerID.Valid && owner.ID == p.OwnerID.Int64
	p.Collection = coll
	p.IsTopLevel = app.cfg.App.SingleUser

	if !p.IsOwner && silenced {
		return ErrPostNotFound
	}
	// Check if post has been unpublished
	if p.Content == "" && p.Title.String == "" {
		return impart.HTTPError{http.StatusGone, "Post was unpublished."}
	}

	p.augmentContent()

	// Serve collection post
	if isRaw {
		contentType := "text/plain"
		if isJSON {
			contentType = "application/json"
		} else if isXML {
			contentType = "application/xml"
		} else if isMarkdown {
			contentType = "text/markdown"
		}
		w.Header().Set("Content-Type", fmt.Sprintf("%s; charset=utf-8", contentType))
		if !postFound {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(w, "Post not found.")
			// TODO: return error instead, so status is correctly reflected in logs
			return nil
		}
		if isMarkdown && p.Title.String != "" {
			fmt.Fprintf(w, "# %s\n\n", p.Title.String)
		}
		fmt.Fprint(w, p.Content)
	} else if strings.Contains(r.Header.Get("Accept"), "application/activity+json") {
		if !postFound {
			return ErrCollectionPageNotFound
		}
		p.extractData()
		ap := p.ActivityObject(app)
		ap.Context = []interface{}{activitystreams.Namespace}
		setCacheControl(w, apCacheTime)
		return impart.RenderActivityJSON(w, ap, http.StatusOK)
	} else {
		p.extractData()
		p.Content = strings.Replace(p.Content, "<!--more-->", "", 1)
		// TODO: move this to function
		p.formatContent(app.cfg, cr.isCollOwner)
		tp := struct {
			*PublicPost
			page.StaticPage
			IsOwner        bool
			IsPinned       bool
			IsCustomDomain bool
			PinnedPosts    *[]PublicPost
			IsFound        bool
			IsAdmin        bool
			CanInvite      bool
			Silenced       bool
		}{
			PublicPost:     p,
			StaticPage:     pageForReq(app, r),
			IsOwner:        cr.isCollOwner,
			IsCustomDomain: cr.isCustomDomain,
			IsFound:        postFound,
			Silenced:       silenced,
		}
		tp.IsAdmin = u != nil && u.IsAdmin()
		tp.CanInvite = canUserInvite(app.cfg, tp.IsAdmin)
		tp.PinnedPosts, _ = app.db.GetPinnedPosts(coll, p.IsOwner)
		tp.IsPinned = len(*tp.PinnedPosts) > 0 && PostsContains(tp.PinnedPosts, p)

		if !postFound {
			w.WriteHeader(http.StatusNotFound)
		}
		postTmpl := "collection-post"
		if app.cfg.App.Chorus {
			postTmpl = "chorus-collection-post"
		}
		if err := templates[postTmpl].ExecuteTemplate(w, "post", tp); err != nil {
			log.Error("Error in collection-post template: %v", err)
		}
	}

	go func() {
		if p.OwnerID.Valid {
			// Post is owned by someone. Don't update stats if owner is viewing the post.
			if u != nil && p.OwnerID.Int64 == u.ID {
				return
			}
		}
		// Update stats for non-raw post views
		if !isRaw && r.Method != "HEAD" && !bots.IsBot(r.UserAgent()) {
			_, err := app.db.Exec("UPDATE posts SET view_count = view_count + 1 WHERE slug = ? AND collection_id = ?", slug, coll.ID)
			if err != nil {
				log.Error("Unable to update posts count: %v", err)
			}
		}
	}()

	return nil
}

// TODO: move this to utils after making it more generic
func PostsContains(sl *[]PublicPost, s *PublicPost) bool {
	for _, e := range *sl {
		if e.ID == s.ID {
			return true
		}
	}
	return false
}

func (p *Post) extractData() {
	p.Tags = tags.Extract(p.Content)
	p.extractImages()
}

func (rp *RawPost) UserFacingCreated() string {
	return rp.Created.Format(postMetaDateFormat)
}

func (rp *RawPost) Created8601() string {
	return rp.Created.Format("2006-01-02T15:04:05Z")
}

func (rp *RawPost) Updated8601() string {
	if rp.Updated.IsZero() {
		return ""
	}
	return rp.Updated.Format("2006-01-02T15:04:05Z")
}

var imageURLRegex = regexp.MustCompile(`(?i)[^ ]+\.(gif|png|jpg|jpeg|image)$`)

func (p *Post) extractImages() {
	p.Images = extractImages(p.Content)
}

func extractImages(content string) []string {
	matches := extract.ExtractUrls(content)
	urls := map[string]bool{}
	for i := range matches {
		uRaw := matches[i].Text
		// Parse the extracted text so we can examine the path
		u, err := url.Parse(uRaw)
		if err != nil {
			continue
		}
		// Ensure the path looks like it leads to an image file
		if !imageURLRegex.MatchString(u.Path) {
			continue
		}
		urls[uRaw] = true
	}

	resURLs := make([]string, 0)
	for k := range urls {
		resURLs = append(resURLs, k)
	}
	return resURLs
}
