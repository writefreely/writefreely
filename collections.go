/*
 * Copyright © 2018-2021 Musing Studio LLC.
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
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/gorilla/mux"
	"github.com/writeas/impart"
	"github.com/writeas/web-core/activitystreams"
	"github.com/writeas/web-core/auth"
	"github.com/writeas/web-core/bots"
	"github.com/writeas/web-core/i18n"
	"github.com/writeas/web-core/log"
	waposts "github.com/writeas/web-core/posts"
	"github.com/writefreely/writefreely/author"
	"github.com/writefreely/writefreely/config"
	"github.com/writefreely/writefreely/page"
	"github.com/writefreely/writefreely/spam"
	"golang.org/x/net/idna"
)

const (
	collAttrLetterReplyTo = "letter_reply_to"

	collMaxLengthTitle       = 255
	collMaxLengthDescription = 160
)

type (
	// TODO: add Direction to db
	// TODO: add Language to db
	Collection struct {
		ID          int64          `datastore:"id" json:"-"`
		Alias       string         `datastore:"alias" schema:"alias" json:"alias"`
		Title       string         `datastore:"title" schema:"title" json:"title"`
		Description string         `datastore:"description" schema:"description" json:"description"`
		Direction   string         `schema:"dir" json:"dir,omitempty"`
		Language    string         `schema:"lang" json:"lang,omitempty"`
		StyleSheet  string         `datastore:"style_sheet" schema:"style_sheet" json:"style_sheet"`
		Script      string         `datastore:"script" schema:"script" json:"script,omitempty"`
		Signature   string         `datastore:"post_signature" schema:"signature" json:"-"`
		Public      bool           `datastore:"public" json:"public"`
		Visibility  collVisibility `datastore:"private" json:"-"`
		Format      string         `datastore:"format" json:"format,omitempty"`
		Views       int64          `json:"views"`
		OwnerID     int64          `datastore:"owner_id" json:"-"`
		PublicOwner bool           `datastore:"public_owner" json:"-"`
		URL         string         `json:"url,omitempty"`

		Monetization string `json:"monetization_pointer,omitempty"`
		Verification string `json:"verification_link"`

		db       *datastore
		hostName string
	}
	CollectionObj struct {
		Collection
		TotalPosts int           `json:"total_posts"`
		Owner      *User         `json:"owner,omitempty"`
		Posts      *[]PublicPost `json:"posts,omitempty"`
		Format     *CollectionFormat
	}
	DisplayCollection struct {
		*CollectionObj
		Prefix      string
		NavSuffix   string
		IsTopLevel  bool
		CurrentPage int
		TotalPages  int
		Silenced    bool
	}

	CollectionNav struct {
		*Collection
		Path       string
		SingleUser bool
		CanPost    bool
	}

	SubmittedCollection struct {
		// Data used for updating a given collection
		ID      int64
		OwnerID uint64

		// Form helpers
		PreferURL string `schema:"prefer_url" json:"prefer_url"`
		Privacy   int    `schema:"privacy" json:"privacy"`
		Pass      string `schema:"password" json:"password"`
		MathJax   bool   `schema:"mathjax" json:"mathjax"`
		EmailSubs bool   `schema:"email_subs" json:"email_subs"`
		Handle    string `schema:"handle" json:"handle"`

		// Actual collection values updated in the DB
		Alias        *string         `schema:"alias" json:"alias"`
		Title        *string         `schema:"title" json:"title"`
		Description  *string         `schema:"description" json:"description"`
		StyleSheet   *string         `schema:"style_sheet" json:"style_sheet"`
		Script       *string         `schema:"script" json:"script"`
		Signature    *string         `schema:"signature" json:"signature"`
		Monetization *string         `schema:"monetization_pointer" json:"monetization_pointer"`
		Verification *string         `schema:"verification_link" json:"verification_link"`
		LetterReply  *string         `schema:"letter_reply" json:"letter_reply"`
		Visibility   *int            `schema:"visibility" json:"public"`
		Format       *sql.NullString `schema:"format" json:"format"`
	}
	CollectionFormat struct {
		Format string
	}

	collectionReq struct {
		// Information about the collection request itself
		prefix, alias, domain string
		isCustomDomain        bool

		// User-related fields
		isCollOwner bool

		isAuthorized bool
	}
)

func (sc *SubmittedCollection) FediverseHandle() string {
	if sc.Handle == "" {
		return apCustomHandleDefault
	}
	return getSlug(sc.Handle, "")
}

// collVisibility represents the visibility level for the collection.
type collVisibility int

// Visibility levels. Values are bitmasks, stored in the database as
// decimal numbers. If adding types, append them to this list. If removing,
// replace the desired visibility with a new value.
const CollUnlisted collVisibility = 0
const (
	CollPublic collVisibility = 1 << iota
	CollPrivate
	CollProtected
)

var collVisibilityStrings = map[string]collVisibility{
	"unlisted":  CollUnlisted,
	"public":    CollPublic,
	"private":   CollPrivate,
	"protected": CollProtected,
}

func defaultVisibility(cfg *config.Config) collVisibility {
	vis, ok := collVisibilityStrings[cfg.App.DefaultVisibility]
	if !ok {
		vis = CollUnlisted
	}
	return vis
}

func (cf *CollectionFormat) Ascending() bool {
	return cf.Format == "novel"
}
func (cf *CollectionFormat) ShowDates() bool {
	return cf.Format == "blog"
}
func (cf *CollectionFormat) PostsPerPage() int {
	if cf.Format == "novel" {
		return postsPerPage
	}
	return postsPerPage
}

// Valid returns whether or not a format value is valid.
func (cf *CollectionFormat) Valid() bool {
	return cf.Format == "blog" ||
		cf.Format == "novel" ||
		cf.Format == "notebook"
}

// NewFormat creates a new CollectionFormat object from the Collection.
func (c *Collection) NewFormat() *CollectionFormat {
	cf := &CollectionFormat{Format: c.Format}

	// Fill in default format
	if cf.Format == "" {
		cf.Format = "blog"
	}

	return cf
}

func (c *Collection) IsInstanceColl() bool {
	ur, _ := url.Parse(c.hostName)
	return c.Alias == ur.Host
}

func (c *Collection) IsUnlisted() bool {
	return c.Visibility == 0
}

func (c *Collection) IsPrivate() bool {
	return c.Visibility&CollPrivate != 0
}

func (c *Collection) IsProtected() bool {
	return c.Visibility&CollProtected != 0
}

func (c *Collection) IsPublic() bool {
	return c.Visibility&CollPublic != 0
}

func (c *Collection) FriendlyVisibility() string {
	if c.IsPrivate() {
		return "Private"
	}
	if c.IsPublic() {
		return "Public"
	}
	if c.IsProtected() {
		return "Password-protected"
	}
	return "Unlisted"
}

func (c *Collection) ShowFooterBranding() bool {
	// TODO: implement this setting
	return true
}

// CanonicalURL returns a fully-qualified URL to the collection.
func (c *Collection) CanonicalURL() string {
	return c.RedirectingCanonicalURL(false)
}

func (c *Collection) DisplayCanonicalURL() string {
	us := c.CanonicalURL()
	u, err := url.Parse(us)
	if err != nil {
		return us
	}
	p := u.Path
	if p == "/" {
		p = ""
	}
	d := u.Hostname()
	d, _ = idna.ToUnicode(d)
	return d + p
}

// RedirectingCanonicalURL returns the fully-qualified canonical URL for the Collection, with a trailing slash. The
// hostName field needs to be populated for this to work correctly.
func (c *Collection) RedirectingCanonicalURL(isRedir bool) string {
	if c.hostName == "" {
		// If this is true, the human programmers screwed up. So ask for a bug report and fail, fail, fail
		log.Error("[PROGRAMMER ERROR] WARNING: Collection.hostName is empty! Federation and many other things will fail! If you're seeing this in the wild, please report this bug and let us know what you were doing just before this: https://github.com/writefreely/writefreely/issues/new?template=bug_report.md")
	}
	if isSingleUser {
		return c.hostName + "/"
	}

	return fmt.Sprintf("%s/%s/", c.hostName, c.Alias)
}

// PrevPageURL provides a full URL for the previous page of collection posts,
// returning a /page/N result for pages >1
func (c *Collection) PrevPageURL(prefix, navSuffix string, n int, tl bool) string {
	u := ""
	if n == 2 {
		// Previous page is 1; no need for /page/ prefix
		if prefix == "" {
			u = navSuffix + "/"
		}
		// Else leave off trailing slash
	} else {
		u = fmt.Sprintf("%s/page/%d", navSuffix, n-1)
	}

	if tl {
		return u
	}
	return "/" + prefix + c.Alias + u
}

// NextPageURL provides a full URL for the next page of collection posts
func (c *Collection) NextPageURL(prefix, navSuffix string, n int, tl bool) string {

	if tl {
		return fmt.Sprintf("%s/page/%d", navSuffix, n+1)
	}
	return fmt.Sprintf("/%s%s%s/page/%d", prefix, c.Alias, navSuffix, n+1)
}

func (c *Collection) DisplayTitle() string {
	if c.Title != "" {
		return c.Title
	}
	return c.Alias
}

func (c *Collection) StyleSheetDisplay() template.CSS {
	return template.CSS(c.StyleSheet)
}

// ForPublic modifies the Collection for public consumption, such as via
// the API.
func (c *Collection) ForPublic() {
	c.URL = c.CanonicalURL()
}

var isAvatarChar = regexp.MustCompile("[a-z0-9]").MatchString

func (c *Collection) PersonObject(ids ...int64) *activitystreams.Person {
	accountRoot := c.FederatedAccount()
	p := activitystreams.NewPerson(accountRoot)
	p.URL = c.CanonicalURL()
	uname := c.Alias
	p.PreferredUsername = uname
	p.Name = c.DisplayTitle()
	p.Summary = c.Description
	if p.Name != "" {
		if av := c.AvatarURL(); av != "" {
			p.Icon = activitystreams.Image{
				Type:      "Image",
				MediaType: "image/png",
				URL:       av,
			}
		}
	}

	collID := c.ID
	if len(ids) > 0 {
		collID = ids[0]
	}
	pub, priv := c.db.GetAPActorKeys(collID)
	if pub != nil {
		p.AddPubKey(pub)
		p.SetPrivKey(priv)
	}

	return p
}

func (c *Collection) AvatarURL() string {
	fl := string(unicode.ToLower([]rune(c.DisplayTitle())[0]))
	if !isAvatarChar(fl) {
		return ""
	}
	return c.hostName + "/img/avatars/" + fl + ".png"
}

func (c *Collection) FederatedAPIBase() string {
	return c.hostName + "/"
}

func (c *Collection) FederatedAccount() string {
	accountUser := c.Alias
	return c.FederatedAPIBase() + "api/collections/" + accountUser
}

func (c *Collection) RenderMathJax() bool {
	return c.db.CollectionHasAttribute(c.ID, "render_mathjax")
}

func (c *Collection) EmailSubsEnabled() bool {
	return c.db.CollectionHasAttribute(c.ID, "email_subs")
}

func (c *Collection) MonetizationURL() string {
	if c.Monetization == "" {
		return ""
	}
	return strings.Replace(c.Monetization, "$", "https://", 1)
}

func (c CollectionPage) DisplayMonetization() string {
	return displayMonetization(c.Monetization, c.Alias)
}

func (c *DisplayCollection) Direction() string {
	if c.Language == "" {
		return "auto"
	}
	if i18n.LangIsRTL(c.Language) {
		return "rtl"
	}
	return "ltr"
}

func newCollection(app *App, w http.ResponseWriter, r *http.Request) error {
	reqJSON := IsJSON(r)
	alias := r.FormValue("alias")
	title := r.FormValue("title")

	var missingParams, accessToken string
	var u *User
	c := struct {
		Alias string `json:"alias" schema:"alias"`
		Title string `json:"title" schema:"title"`
		Web   bool   `json:"web" schema:"web"`
	}{}
	if reqJSON {
		// Decode JSON request
		decoder := json.NewDecoder(r.Body)
		err := decoder.Decode(&c)
		if err != nil {
			log.Error("Couldn't parse post update JSON request: %v\n", err)
			return ErrBadJSON
		}
	} else {
		// TODO: move form parsing to formDecoder
		c.Alias = alias
		c.Title = title
	}

	if c.Alias == "" {
		if c.Title != "" {
			// If only a title was given, just use it to generate the alias.
			c.Alias = getSlug(c.Title, "")
		} else {
			missingParams += "`alias` "
		}
	}
	if c.Title == "" {
		missingParams += "`title` "
	}
	if missingParams != "" {
		return impart.HTTPError{http.StatusBadRequest, fmt.Sprintf("Parameter(s) %srequired.", missingParams)}
	}

	var userID int64
	var err error
	if reqJSON && !c.Web {
		accessToken = r.Header.Get("Authorization")
		if accessToken == "" {
			return ErrNoAccessToken
		}
		userID = app.db.GetUserID(accessToken)
		if userID == -1 {
			return ErrBadAccessToken
		}
	} else {
		u = getUserSession(app, r)
		if u == nil {
			return ErrNotLoggedIn
		}
		userID = u.ID
	}
	silenced, err := app.db.IsUserSilenced(userID)
	if err != nil {
		log.Error("new collection: %v", err)
		return ErrInternalGeneral
	}
	if silenced {
		return ErrUserSilenced
	}

	if !author.IsValidUsername(app.cfg, c.Alias) {
		return impart.HTTPError{http.StatusPreconditionFailed, "Collection alias isn't valid."}
	}

	coll, err := app.db.CreateCollection(app.cfg, c.Alias, c.Title, userID)
	if err != nil {
		// TODO: handle this
		return err
	}

	res := &CollectionObj{Collection: *coll}

	if reqJSON {
		return impart.WriteSuccess(w, res, http.StatusCreated)
	}
	redirectTo := "/me/c/"
	// TODO: redirect to pad when necessary
	return impart.HTTPError{http.StatusFound, redirectTo}
}

func apiCheckCollectionPermissions(app *App, r *http.Request, c *Collection) (int64, error) {
	accessToken := r.Header.Get("Authorization")
	var userID int64 = -1
	if accessToken != "" {
		userID = app.db.GetUserID(accessToken)
	}
	isCollOwner := userID == c.OwnerID
	if c.IsPrivate() && !isCollOwner {
		// Collection is private, but user isn't authenticated
		return -1, ErrCollectionNotFound
	}
	if c.IsProtected() {
		// TODO: check access token
		return -1, ErrCollectionUnauthorizedRead
	}

	return userID, nil
}

// fetchCollection handles the API endpoint for retrieving collection data.
func fetchCollection(app *App, w http.ResponseWriter, r *http.Request) error {
	if IsActivityPubRequest(r) {
		return handleFetchCollectionActivities(app, w, r)
	}

	vars := mux.Vars(r)
	alias := vars["alias"]

	// TODO: move this logic into a common getCollection function
	// Get base Collection data
	c, err := app.db.GetCollection(alias)
	if err != nil {
		return err
	}
	c.hostName = app.cfg.App.Host

	// Redirect users who aren't requesting JSON
	reqJSON := IsJSON(r)
	if !reqJSON {
		return impart.HTTPError{http.StatusFound, c.CanonicalURL()}
	}

	// Check permissions
	userID, err := apiCheckCollectionPermissions(app, r, c)
	if err != nil {
		return err
	}
	isCollOwner := userID == c.OwnerID

	// Fetch extra data about the Collection
	res := &CollectionObj{Collection: *c}
	if c.PublicOwner {
		u, err := app.db.GetUserByID(res.OwnerID)
		if err != nil {
			// Log the error and just continue
			log.Error("Error getting user for collection: %v", err)
		} else {
			res.Owner = u
		}
	}
	// TODO: check status for silenced
	app.db.GetPostsCount(res, isCollOwner)
	// Strip non-public information
	res.Collection.ForPublic()

	return impart.WriteSuccess(w, res, http.StatusOK)
}

// fetchCollectionPosts handles an API endpoint for retrieving a collection's
// posts.
func fetchCollectionPosts(app *App, w http.ResponseWriter, r *http.Request) error {
	vars := mux.Vars(r)
	alias := vars["alias"]

	c, err := app.db.GetCollection(alias)
	if err != nil {
		return err
	}
	c.hostName = app.cfg.App.Host

	// Check permissions
	userID, err := apiCheckCollectionPermissions(app, r, c)
	if err != nil {
		return err
	}
	isCollOwner := userID == c.OwnerID

	// Get page
	page := 1
	if p := r.FormValue("page"); p != "" {
		pInt, _ := strconv.Atoi(p)
		if pInt > 0 {
			page = pInt
		}
	}

	posts, err := app.db.GetPosts(app.cfg, c, page, isCollOwner, false, false)
	if err != nil {
		return err
	}
	coll := &CollectionObj{Collection: *c, Posts: posts}
	app.db.GetPostsCount(coll, isCollOwner)
	// Strip non-public information
	coll.Collection.ForPublic()

	// Transform post bodies if needed
	if r.FormValue("body") == "html" {
		for _, p := range *coll.Posts {
			p.Content = waposts.ApplyMarkdown([]byte(p.Content))
		}
	}

	return impart.WriteSuccess(w, coll, http.StatusOK)
}

type CollectionPage struct {
	page.StaticPage
	*DisplayCollection
	IsCustomDomain bool
	IsWelcome      bool
	IsOwner        bool
	IsCollLoggedIn bool
	Honeypot       string
	IsSubscriber   bool
	CanPin         bool
	Username       string
	Monetization   string
	Flash          template.HTML
	Collections    *[]Collection
	PinnedPosts    *[]PublicPost

	IsAdmin   bool
	CanInvite bool

	// Helper field for Chorus mode
	CollAlias string
}

type TagCollectionPage struct {
	CollectionPage
	Tag string
}

func (tcp TagCollectionPage) PrevPageURL(prefix string, n int, tl bool) string {
	u := fmt.Sprintf("/tag:%s", tcp.Tag)
	if n > 2 {
		u += fmt.Sprintf("/page/%d", n-1)
	}
	if tl {
		return u
	}
	return "/" + prefix + tcp.Alias + u

}

func (tcp TagCollectionPage) NextPageURL(prefix string, n int, tl bool) string {
	if tl {
		return fmt.Sprintf("/tag:%s/page/%d", tcp.Tag, n+1)
	}
	return fmt.Sprintf("/%s%s/tag:%s/page/%d", prefix, tcp.Alias, tcp.Tag, n+1)
}

func NewCollectionObj(c *Collection) *CollectionObj {
	return &CollectionObj{
		Collection: *c,
		Format:     c.NewFormat(),
	}
}

func (c *CollectionObj) ScriptDisplay() template.JS {
	return template.JS(c.Script)
}

var jsSourceCommentReg = regexp.MustCompile("(?m)^// src:(.+)$")

func (c *CollectionObj) ExternalScripts() []template.URL {
	scripts := []template.URL{}
	if c.Script == "" {
		return scripts
	}

	matches := jsSourceCommentReg.FindAllStringSubmatch(c.Script, -1)
	for _, m := range matches {
		scripts = append(scripts, template.URL(strings.TrimSpace(m[1])))
	}
	return scripts
}

func (c *CollectionObj) CanShowScript() bool {
	return false
}

func processCollectionRequest(cr *collectionReq, vars map[string]string, w http.ResponseWriter, r *http.Request) error {
	cr.prefix = vars["prefix"]
	cr.alias = vars["collection"]
	// Normalize the URL, redirecting user to consistent post URL
	if cr.alias != strings.ToLower(cr.alias) {
		return impart.HTTPError{http.StatusMovedPermanently, fmt.Sprintf("/%s/", strings.ToLower(cr.alias))}
	}

	return nil
}

// processCollectionPermissions checks the permissions for the given
// collectionReq, returning a Collection if access is granted; otherwise this
// renders any necessary collection pages, for example, if requesting a custom
// domain that doesn't yet have a collection associated, or if a collection
// requires a password. In either case, this will return nil, nil -- thus both
// values should ALWAYS be checked to determine whether or not to continue.
func processCollectionPermissions(app *App, cr *collectionReq, u *User, w http.ResponseWriter, r *http.Request) (*Collection, error) {
	// Display collection if this is a collection
	var c *Collection
	var err error
	if app.cfg.App.SingleUser {
		c, err = app.db.GetCollectionByID(1)
	} else {
		c, err = app.db.GetCollection(cr.alias)
	}
	// TODO: verify we don't reveal the existence of a private collection with redirection
	if err != nil {
		if err, ok := err.(impart.HTTPError); ok {
			if err.Status == http.StatusNotFound {
				if cr.isCustomDomain {
					// User is on the site from a custom domain
					//tErr := pages["404-domain.tmpl"].ExecuteTemplate(w, "base", pageForHost(page.StaticPage{}, r))
					//if tErr != nil {
					//log.Error("Unable to render 404-domain page: %v", err)
					//}
					return nil, nil
				}
				if len(cr.alias) >= minIDLen && len(cr.alias) <= maxIDLen {
					// Alias is within post ID range, so just be sure this isn't a post
					if app.db.PostIDExists(cr.alias) {
						// TODO: use StatusFound for vanity post URLs when we implement them
						return nil, impart.HTTPError{http.StatusMovedPermanently, "/" + cr.alias}
					}
				}
				// Redirect if necessary
				newAlias := app.db.GetCollectionRedirect(cr.alias)
				if newAlias != "" {
					return nil, impart.HTTPError{http.StatusFound, "/" + newAlias + "/"}
				}
			}
		}
		return nil, err
	}
	c.hostName = app.cfg.App.Host

	// Update CollectionRequest to reflect owner status
	cr.isCollOwner = u != nil && u.ID == c.OwnerID

	// Check permissions
	if !cr.isCollOwner {
		if c.IsPrivate() {
			return nil, ErrCollectionNotFound
		} else if c.IsProtected() {
			uname := ""
			if u != nil {
				uname = u.Username
			}

			// TODO: move this to all permission checks?
			suspended, err := app.db.IsUserSilenced(c.OwnerID)
			if err != nil {
				log.Error("process protected collection permissions: %v", err)
				return nil, err
			}
			if suspended {
				return nil, ErrCollectionNotFound
			}

			// See if we've authorized this collection
			cr.isAuthorized = isAuthorizedForCollection(app, c.Alias, r)

			if !cr.isAuthorized {
				p := struct {
					page.StaticPage
					*CollectionObj
					Username string
					Next     string
					Flashes  []template.HTML
				}{
					StaticPage:    pageForReq(app, r),
					CollectionObj: &CollectionObj{Collection: *c},
					Username:      uname,
					Next:          r.FormValue("g"),
					Flashes:       []template.HTML{},
				}
				// Get owner information
				p.CollectionObj.Owner, err = app.db.GetUserByID(c.OwnerID)
				if err != nil {
					// Log the error and just continue
					log.Error("Error getting user for collection: %v", err)
				}

				flashes, _ := getSessionFlashes(app, w, r, nil)
				for _, flash := range flashes {
					p.Flashes = append(p.Flashes, template.HTML(flash))
				}
				err = templates["password-collection"].ExecuteTemplate(w, "password-collection", p)
				if err != nil {
					log.Error("Unable to render password-collection: %v", err)
					return nil, err
				}
				return nil, nil
			}
		}
	}
	return c, nil
}

func checkUserForCollection(app *App, cr *collectionReq, r *http.Request, isPostReq bool) (*User, error) {
	u := getUserSession(app, r)
	return u, nil
}

func newDisplayCollection(c *Collection, cr *collectionReq, page int) *DisplayCollection {
	coll := &DisplayCollection{
		CollectionObj: NewCollectionObj(c),
		CurrentPage:   page,
		Prefix:        cr.prefix,
		IsTopLevel:    isSingleUser,
	}
	c.db.GetPostsCount(coll.CollectionObj, cr.isCollOwner)
	return coll
}

// getCollectionPage returns the collection page as an int. If the parsed page value is not
// greater than 0 then the default value of 1 is returned.
func getCollectionPage(vars map[string]string) int {
	if p, _ := strconv.Atoi(vars["page"]); p > 0 {
		return p
	}

	return 1
}

// handleViewCollection displays the requested Collection
func handleViewCollection(app *App, w http.ResponseWriter, r *http.Request) error {
	vars := mux.Vars(r)
	cr := &collectionReq{}

	err := processCollectionRequest(cr, vars, w, r)
	if err != nil {
		return err
	}

	u, err := checkUserForCollection(app, cr, r, false)
	if err != nil {
		return err
	}

	page := getCollectionPage(vars)

	c, err := processCollectionPermissions(app, cr, u, w, r)
	if c == nil || err != nil {
		return err
	}
	c.hostName = app.cfg.App.Host

	silenced, err := app.db.IsUserSilenced(c.OwnerID)
	if err != nil {
		log.Error("view collection: %v", err)
		return ErrInternalGeneral
	}

	// Serve ActivityStreams data now, if requested
	if IsActivityPubRequest(r) {
		ac := c.PersonObject()
		ac.Context = []interface{}{activitystreams.Namespace}
		setCacheControl(w, apCacheTime)
		return impart.RenderActivityJSON(w, ac, http.StatusOK)
	}

	// Fetch extra data about the Collection
	// TODO: refactor out this logic, shared in collection.go:fetchCollection()
	coll := newDisplayCollection(c, cr, page)

	coll.TotalPages = int(math.Ceil(float64(coll.TotalPosts) / float64(coll.Format.PostsPerPage())))
	if coll.TotalPages > 0 && page > coll.TotalPages {
		redirURL := fmt.Sprintf("/page/%d", coll.TotalPages)
		if !app.cfg.App.SingleUser {
			redirURL = fmt.Sprintf("/%s%s%s", cr.prefix, coll.Alias, redirURL)
		}
		return impart.HTTPError{http.StatusFound, redirURL}
	}

	coll.Posts, _ = app.db.GetPosts(app.cfg, c, page, cr.isCollOwner, false, false)

	// Serve collection
	displayPage := CollectionPage{
		DisplayCollection: coll,
		IsCollLoggedIn:    cr.isAuthorized,
		StaticPage:        pageForReq(app, r),
		IsCustomDomain:    cr.isCustomDomain,
		IsWelcome:         r.FormValue("greeting") != "",
		Honeypot:          spam.HoneypotFieldName(),
		CollAlias:         c.Alias,
	}
	flashes, _ := getSessionFlashes(app, w, r, nil)
	for _, f := range flashes {
		displayPage.Flash = template.HTML(f)
	}
	displayPage.IsAdmin = u != nil && u.IsAdmin()
	displayPage.CanInvite = canUserInvite(app.cfg, displayPage.IsAdmin)
	var owner *User
	if u != nil {
		displayPage.Username = u.Username
		displayPage.IsOwner = u.ID == coll.OwnerID
		displayPage.IsSubscriber = u.IsEmailSubscriber(app, coll.ID)
		if displayPage.IsOwner {
			// Add in needed information for users viewing their own collection
			owner = u
			displayPage.CanPin = true

			pubColls, err := app.db.GetPublishableCollections(owner, app.cfg.App.Host)
			if err != nil {
				log.Error("unable to fetch collections: %v", err)
			}
			displayPage.Collections = pubColls
		}
	}
	isOwner := owner != nil
	if !isOwner {
		// Current user doesn't own collection; retrieve owner information
		owner, err = app.db.GetUserByID(coll.OwnerID)
		if err != nil {
			// Log the error and just continue
			log.Error("Error getting user for collection: %v", err)
		}
	}
	if !isOwner && silenced {
		return ErrCollectionNotFound
	}
	displayPage.Silenced = isOwner && silenced
	displayPage.Owner = owner
	coll.Owner = displayPage.Owner

	// Add more data
	// TODO: fix this mess of collections inside collections
	displayPage.PinnedPosts, _ = app.db.GetPinnedPosts(coll.CollectionObj, isOwner)
	displayPage.Monetization = app.db.GetCollectionAttribute(coll.ID, "monetization_pointer")

	collTmpl := "collection"
	if app.cfg.App.Chorus {
		collTmpl = "chorus-collection"
	}
	err = templates[collTmpl].ExecuteTemplate(w, "collection", displayPage)
	if err != nil {
		log.Error("Unable to render collection index: %v", err)
	}

	// Update collection view count
	go func() {
		// Don't update if owner is viewing the collection.
		if u != nil && u.ID == coll.OwnerID {
			return
		}
		// Only update for human views
		if r.Method == "HEAD" || bots.IsBot(r.UserAgent()) {
			return
		}

		_, err := app.db.Exec("UPDATE collections SET view_count = view_count + 1 WHERE id = ?", coll.ID)
		if err != nil {
			log.Error("Unable to update collections count: %v", err)
		}
	}()

	return err
}

func handleViewMention(app *App, w http.ResponseWriter, r *http.Request) error {
	vars := mux.Vars(r)
	handle := vars["handle"]

	remoteUser, err := app.db.GetProfilePageFromHandle(app, handle)
	if err != nil || remoteUser == "" {
		log.Error("Couldn't find user %s: %v", handle, err)
		return ErrRemoteUserNotFound
	}

	return impart.HTTPError{Status: http.StatusFound, Message: remoteUser}
}

func handleViewCollectionTag(app *App, w http.ResponseWriter, r *http.Request) error {
	vars := mux.Vars(r)
	tag := vars["tag"]

	cr := &collectionReq{}
	err := processCollectionRequest(cr, vars, w, r)
	if err != nil {
		return err
	}

	u, err := checkUserForCollection(app, cr, r, false)
	if err != nil {
		return err
	}

	page := getCollectionPage(vars)

	c, err := processCollectionPermissions(app, cr, u, w, r)
	if c == nil || err != nil {
		return err
	}

	coll := newDisplayCollection(c, cr, page)

	taggedPostIDs, err := app.db.GetAllPostsTaggedIDs(c, tag, cr.isCollOwner)
	if err != nil {
		return err
	}

	ttlPosts := len(taggedPostIDs)
	pagePosts := coll.Format.PostsPerPage()
	coll.TotalPages = int(math.Ceil(float64(ttlPosts) / float64(pagePosts)))
	if coll.TotalPages > 0 && page > coll.TotalPages {
		redirURL := fmt.Sprintf("/page/%d", coll.TotalPages)
		if !app.cfg.App.SingleUser {
			redirURL = fmt.Sprintf("/%s%s%s", cr.prefix, coll.Alias, redirURL)
		}
		return impart.HTTPError{http.StatusFound, redirURL}
	}

	coll.Posts, _ = app.db.GetPostsTagged(app.cfg, c, tag, page, cr.isCollOwner)
	if coll.Posts != nil && len(*coll.Posts) == 0 {
		return ErrCollectionPageNotFound
	}

	// Serve collection
	displayPage := TagCollectionPage{
		CollectionPage: CollectionPage{
			DisplayCollection: coll,
			StaticPage:        pageForReq(app, r),
			IsCustomDomain:    cr.isCustomDomain,
		},
		Tag: tag,
	}
	var owner *User
	if u != nil {
		displayPage.Username = u.Username
		displayPage.IsOwner = u.ID == coll.OwnerID
		if displayPage.IsOwner {
			// Add in needed information for users viewing their own collection
			owner = u
			displayPage.CanPin = true

			pubColls, err := app.db.GetPublishableCollections(owner, app.cfg.App.Host)
			if err != nil {
				log.Error("unable to fetch collections: %v", err)
			}
			displayPage.Collections = pubColls
		}
	}
	isOwner := owner != nil
	if !isOwner {
		// Current user doesn't own collection; retrieve owner information
		owner, err = app.db.GetUserByID(coll.OwnerID)
		if err != nil {
			// Log the error and just continue
			log.Error("Error getting user for collection: %v", err)
		}
		if owner.IsSilenced() {
			return ErrCollectionNotFound
		}
	}
	displayPage.Silenced = owner != nil && owner.IsSilenced()
	displayPage.Owner = owner
	coll.Owner = displayPage.Owner
	// Add more data
	// TODO: fix this mess of collections inside collections
	displayPage.PinnedPosts, _ = app.db.GetPinnedPosts(coll.CollectionObj, isOwner)
	displayPage.Monetization = app.db.GetCollectionAttribute(coll.ID, "monetization_pointer")

	err = templates["collection-tags"].ExecuteTemplate(w, "collection-tags", displayPage)
	if err != nil {
		log.Error("Unable to render collection tag page: %v", err)
	}

	return nil
}

func handleViewCollectionLang(app *App, w http.ResponseWriter, r *http.Request) error {
	vars := mux.Vars(r)
	lang := vars["lang"]

	cr := &collectionReq{}
	err := processCollectionRequest(cr, vars, w, r)
	if err != nil {
		return err
	}

	u, err := checkUserForCollection(app, cr, r, false)
	if err != nil {
		return err
	}

	page := getCollectionPage(vars)

	c, err := processCollectionPermissions(app, cr, u, w, r)
	if c == nil || err != nil {
		return err
	}

	coll := newDisplayCollection(c, cr, page)
	coll.Language = lang
	coll.NavSuffix = fmt.Sprintf("/lang:%s", lang)

	ttlPosts, err := app.db.GetCollLangTotalPosts(coll.ID, lang)
	if err != nil {
		log.Error("Unable to getCollLangTotalPosts: %s", err)
	}
	pagePosts := coll.Format.PostsPerPage()
	coll.TotalPages = int(math.Ceil(float64(ttlPosts) / float64(pagePosts)))
	if coll.TotalPages > 0 && page > coll.TotalPages {
		redirURL := fmt.Sprintf("/lang:%s/page/%d", lang, coll.TotalPages)
		if !app.cfg.App.SingleUser {
			redirURL = fmt.Sprintf("/%s%s%s", cr.prefix, coll.Alias, redirURL)
		}
		return impart.HTTPError{http.StatusFound, redirURL}
	}

	coll.Posts, _ = app.db.GetLangPosts(app.cfg, c, lang, page, cr.isCollOwner)
	if err != nil {
		return ErrCollectionPageNotFound
	}

	// Serve collection
	displayPage := struct {
		CollectionPage
		Tag string
	}{
		CollectionPage: CollectionPage{
			DisplayCollection: coll,
			StaticPage:        pageForReq(app, r),
			IsCustomDomain:    cr.isCustomDomain,
		},
		Tag: lang,
	}
	var owner *User
	if u != nil {
		displayPage.Username = u.Username
		displayPage.IsOwner = u.ID == coll.OwnerID
		if displayPage.IsOwner {
			// Add in needed information for users viewing their own collection
			owner = u
			displayPage.CanPin = true

			pubColls, err := app.db.GetPublishableCollections(owner, app.cfg.App.Host)
			if err != nil {
				log.Error("unable to fetch collections: %v", err)
			}
			displayPage.Collections = pubColls
		}
	}
	isOwner := owner != nil
	if !isOwner {
		// Current user doesn't own collection; retrieve owner information
		owner, err = app.db.GetUserByID(coll.OwnerID)
		if err != nil {
			// Log the error and just continue
			log.Error("Error getting user for collection: %v", err)
		}
		if owner.IsSilenced() {
			return ErrCollectionNotFound
		}
	}
	displayPage.Silenced = owner != nil && owner.IsSilenced()
	displayPage.Owner = owner
	coll.Owner = displayPage.Owner
	// Add more data
	// TODO: fix this mess of collections inside collections
	displayPage.PinnedPosts, _ = app.db.GetPinnedPosts(coll.CollectionObj, isOwner)
	displayPage.Monetization = app.db.GetCollectionAttribute(coll.ID, "monetization_pointer")

	collTmpl := "collection"
	if app.cfg.App.Chorus {
		collTmpl = "chorus-collection"
	}
	err = templates[collTmpl].ExecuteTemplate(w, "collection", displayPage)
	if err != nil {
		log.Error("Unable to render collection lang page: %v", err)
	}

	return nil
}

func handleCollectionPostRedirect(app *App, w http.ResponseWriter, r *http.Request) error {
	vars := mux.Vars(r)
	slug := vars["slug"]

	cr := &collectionReq{}
	err := processCollectionRequest(cr, vars, w, r)
	if err != nil {
		return err
	}

	// Normalize the URL, redirecting user to consistent post URL
	loc := fmt.Sprintf("/%s", slug)
	if !app.cfg.App.SingleUser {
		loc = fmt.Sprintf("/%s/%s", cr.alias, slug)
	}
	return impart.HTTPError{http.StatusFound, loc}
}

func existingCollection(app *App, w http.ResponseWriter, r *http.Request) error {
	reqJSON := IsJSON(r)
	vars := mux.Vars(r)
	collAlias := vars["alias"]
	isWeb := r.FormValue("web") == "1"

	u := &User{}
	if reqJSON && !isWeb {
		// Ensure an access token was given
		accessToken := r.Header.Get("Authorization")
		u.ID = app.db.GetUserID(accessToken)
		if u.ID == -1 {
			return ErrBadAccessToken
		}
	} else {
		u = getUserSession(app, r)
		if u == nil {
			return ErrNotLoggedIn
		}
	}

	silenced, err := app.db.IsUserSilenced(u.ID)
	if err != nil {
		log.Error("existing collection: %v", err)
		return ErrInternalGeneral
	}

	if silenced {
		return ErrUserSilenced
	}

	if r.Method == "DELETE" {
		err := app.db.DeleteCollection(collAlias, u.ID)
		if err != nil {
			// TODO: if not HTTPError, report error to admin
			log.Error("Unable to delete collection: %s", err)
			return err
		}
		addSessionFlash(app, w, r, "Deleted your blog, "+collAlias+".", nil)
		return impart.HTTPError{Status: http.StatusNoContent}
	}

	c := SubmittedCollection{OwnerID: uint64(u.ID)}

	if reqJSON {
		// Decode JSON request
		decoder := json.NewDecoder(r.Body)
		err = decoder.Decode(&c)
		if err != nil {
			log.Error("Couldn't parse collection update JSON request: %v\n", err)
			return ErrBadJSON
		}
	} else {
		err = r.ParseForm()
		if err != nil {
			log.Error("Couldn't parse collection update form request: %v\n", err)
			return ErrBadFormData
		}

		err = app.formDecoder.Decode(&c, r.PostForm)
		if err != nil {
			log.Error("Couldn't decode collection update form request: %v\n", err)
			return ErrBadFormData
		}
	}

	err = app.db.UpdateCollection(app, &c, collAlias)
	if err != nil {
		if err, ok := err.(impart.HTTPError); ok {
			if reqJSON {
				return err
			}
			addSessionFlash(app, w, r, err.Message, nil)
			return impart.HTTPError{http.StatusFound, "/me/c/" + collAlias}
		} else {
			log.Error("Couldn't update collection: %v\n", err)
			return err
		}
	}

	if reqJSON {
		return impart.WriteSuccess(w, struct {
		}{}, http.StatusOK)
	}

	addSessionFlash(app, w, r, "Blog updated!", nil)
	return impart.HTTPError{http.StatusFound, "/me/c/" + collAlias}
}

// collectionAliasFromReq takes a request and returns the collection alias
// if it can be ascertained, as well as whether or not the collection uses a
// custom domain.
func collectionAliasFromReq(r *http.Request) string {
	vars := mux.Vars(r)
	alias := vars["subdomain"]
	isSubdomain := alias != ""
	if !isSubdomain {
		// Fall back to write.as/{collection} since this isn't a custom domain
		alias = vars["collection"]
	}
	return alias
}

func handleWebCollectionUnlock(app *App, w http.ResponseWriter, r *http.Request) error {
	var readReq struct {
		Alias string `schema:"alias" json:"alias"`
		Pass  string `schema:"password" json:"password"`
		Next  string `schema:"to" json:"to"`
	}

	// Get params
	if impart.ReqJSON(r) {
		decoder := json.NewDecoder(r.Body)
		err := decoder.Decode(&readReq)
		if err != nil {
			log.Error("Couldn't parse readReq JSON request: %v\n", err)
			return ErrBadJSON
		}
	} else {
		err := r.ParseForm()
		if err != nil {
			log.Error("Couldn't parse readReq form request: %v\n", err)
			return ErrBadFormData
		}

		err = app.formDecoder.Decode(&readReq, r.PostForm)
		if err != nil {
			log.Error("Couldn't decode readReq form request: %v\n", err)
			return ErrBadFormData
		}
	}

	if readReq.Alias == "" {
		return impart.HTTPError{http.StatusBadRequest, "Need a collection `alias` to read."}
	}
	if readReq.Pass == "" {
		return impart.HTTPError{http.StatusBadRequest, "Please supply a password."}
	}

	var collHashedPass []byte
	err := app.db.QueryRow("SELECT password FROM collectionpasswords INNER JOIN collections ON id = collection_id WHERE alias = ?", readReq.Alias).Scan(&collHashedPass)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Error("No collectionpassword found when trying to read collection %s", readReq.Alias)
			return impart.HTTPError{http.StatusInternalServerError, "Something went very wrong. The humans have been alerted."}
		}
		return err
	}

	if !auth.Authenticated(collHashedPass, []byte(readReq.Pass)) {
		return impart.HTTPError{http.StatusUnauthorized, "Incorrect password."}
	}

	// Success; set cookie
	session, err := app.sessionStore.Get(r, blogPassCookieName)
	if err == nil {
		session.Values[readReq.Alias] = true
		err = session.Save(r, w)
		if err != nil {
			log.Error("Didn't save unlocked blog '%s': %v", readReq.Alias, err)
		}
	}

	next := "/" + readReq.Next
	if !app.cfg.App.SingleUser {
		next = "/" + readReq.Alias + next
	}
	return impart.HTTPError{http.StatusFound, next}
}

func isAuthorizedForCollection(app *App, alias string, r *http.Request) bool {
	authd := false
	session, err := app.sessionStore.Get(r, blogPassCookieName)
	if err == nil {
		_, authd = session.Values[alias]
	}
	return authd
}

func logOutCollection(app *App, alias string, w http.ResponseWriter, r *http.Request) error {
	session, err := app.sessionStore.Get(r, blogPassCookieName)
	if err != nil {
		return err
	}

	// Remove this from map of blogs logged into
	delete(session.Values, alias)

	// If not auth'd with any blog, delete entire cookie
	if len(session.Values) == 0 {
		session.Options.MaxAge = -1
	}
	return session.Save(r, w)
}

func handleLogOutCollection(app *App, w http.ResponseWriter, r *http.Request) error {
	alias := collectionAliasFromReq(r)
	var c *Collection
	var err error
	if app.cfg.App.SingleUser {
		c, err = app.db.GetCollectionByID(1)
	} else {
		c, err = app.db.GetCollection(alias)
	}
	if err != nil {
		return err
	}
	if !c.IsProtected() {
		// Invalid to log out of this collection
		return ErrCollectionPageNotFound
	}

	err = logOutCollection(app, c.Alias, w, r)
	if err != nil {
		addSessionFlash(app, w, r, "Logging out failed. Try clearing cookies for this site, instead.", nil)
	}
	return impart.HTTPError{http.StatusFound, c.CanonicalURL()}
}
