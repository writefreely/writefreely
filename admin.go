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
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/writeas/impart"
	"github.com/writeas/web-core/auth"
	"github.com/writeas/web-core/log"
	"github.com/writeas/web-core/passgen"
	"github.com/writeas/writefreely/appstats"
	"github.com/writeas/writefreely/config"
)

var (
	appStartTime = time.Now()
	sysStatus    systemStatus
)

const adminUsersPerPage = 30

type systemStatus struct {
	Uptime       string
	NumGoroutine int

	// General statistics.
	MemAllocated string // bytes allocated and still in use
	MemTotal     string // bytes allocated (even if freed)
	MemSys       string // bytes obtained from system (sum of XxxSys below)
	Lookups      uint64 // number of pointer lookups
	MemMallocs   uint64 // number of mallocs
	MemFrees     uint64 // number of frees

	// Main allocation heap statistics.
	HeapAlloc    string // bytes allocated and still in use
	HeapSys      string // bytes obtained from system
	HeapIdle     string // bytes in idle spans
	HeapInuse    string // bytes in non-idle span
	HeapReleased string // bytes released to the OS
	HeapObjects  uint64 // total number of allocated objects

	// Low-level fixed-size structure allocator statistics.
	//	Inuse is bytes used now.
	//	Sys is bytes obtained from system.
	StackInuse  string // bootstrap stacks
	StackSys    string
	MSpanInuse  string // mspan structures
	MSpanSys    string
	MCacheInuse string // mcache structures
	MCacheSys   string
	BuckHashSys string // profiling bucket hash table
	GCSys       string // GC metadata
	OtherSys    string // other system allocations

	// Garbage collector statistics.
	NextGC       string // next run in HeapAlloc time (bytes)
	LastGC       string // last run in absolute time (ns)
	PauseTotalNs string
	PauseNs      string // circular buffer of recent GC pause times, most recent at [(NumGC+255)%256]
	NumGC        uint32
}

type inspectedCollection struct {
	CollectionObj
	Followers int
	LastPost  string
}

type instanceContent struct {
	ID      string
	Type    string
	Title   sql.NullString
	Content string
	Updated time.Time
}

type AdminPage struct {
	UpdateAvailable bool
}

func NewAdminPage(app *App) *AdminPage {
	ap := &AdminPage{}
	if app.updates != nil {
		ap.UpdateAvailable = app.updates.AreAvailableNoCheck()
	}
	return ap
}

func (c instanceContent) UpdatedFriendly() string {
	/*
		// TODO: accept a locale in this method and use that for the format
		var loc monday.Locale = monday.LocaleEnUS
		return monday.Format(u.Created, monday.DateTimeFormatsByLocale[loc], loc)
	*/
	return c.Updated.Format("January 2, 2006, 3:04 PM")
}

func handleViewAdminDash(app *App, u *User, w http.ResponseWriter, r *http.Request) error {
	p := struct {
		*UserPage
		*AdminPage
		Message string

		UsersCount, CollectionsCount, PostsCount int64
	}{
		UserPage:  NewUserPage(app, r, u, "Admin", nil),
		AdminPage: NewAdminPage(app),
		Message:   r.FormValue("m"),
	}

	// Get user stats
	p.UsersCount = app.db.GetAllUsersCount()
	var err error
	p.CollectionsCount, err = app.db.GetTotalCollections()
	if err != nil {
		return err
	}
	p.PostsCount, err = app.db.GetTotalPosts()
	if err != nil {
		return err
	}

	showUserPage(w, "admin", p)
	return nil
}

func handleViewAdminMonitor(app *App, u *User, w http.ResponseWriter, r *http.Request) error {
	updateAppStats()
	p := struct {
		*UserPage
		*AdminPage
		SysStatus systemStatus
		Config    config.AppCfg

		Message, ConfigMessage string
	}{
		UserPage:  NewUserPage(app, r, u, "Admin", nil),
		AdminPage: NewAdminPage(app),
		SysStatus: sysStatus,
		Config:    app.cfg.App,

		Message:       r.FormValue("m"),
		ConfigMessage: r.FormValue("cm"),
	}

	showUserPage(w, "monitor", p)
	return nil
}

func handleViewAdminSettings(app *App, u *User, w http.ResponseWriter, r *http.Request) error {
	p := struct {
		*UserPage
		*AdminPage
		Config config.AppCfg

		Message, ConfigMessage string
	}{
		UserPage:  NewUserPage(app, r, u, "Admin", nil),
		AdminPage: NewAdminPage(app),
		Config:    app.cfg.App,

		Message:       r.FormValue("m"),
		ConfigMessage: r.FormValue("cm"),
	}

	showUserPage(w, "app-settings", p)
	return nil
}

func handleViewAdminUsers(app *App, u *User, w http.ResponseWriter, r *http.Request) error {
	p := struct {
		*UserPage
		*AdminPage
		Config  config.AppCfg
		Message string

		Users      *[]User
		CurPage    int
		TotalUsers int64
		TotalPages []int
	}{
		UserPage:  NewUserPage(app, r, u, "Users", nil),
		AdminPage: NewAdminPage(app),
		Config:    app.cfg.App,
		Message:   r.FormValue("m"),
	}

	p.TotalUsers = app.db.GetAllUsersCount()
	ttlPages := p.TotalUsers / adminUsersPerPage
	p.TotalPages = []int{}
	for i := 1; i <= int(ttlPages); i++ {
		p.TotalPages = append(p.TotalPages, i)
	}

	var err error
	p.CurPage, err = strconv.Atoi(r.FormValue("p"))
	if err != nil || p.CurPage < 1 {
		p.CurPage = 1
	} else if p.CurPage > int(ttlPages) {
		p.CurPage = int(ttlPages)
	}

	p.Users, err = app.db.GetAllUsers(uint(p.CurPage))
	if err != nil {
		return impart.HTTPError{http.StatusInternalServerError, fmt.Sprintf("Could not get users: %v", err)}
	}

	showUserPage(w, "users", p)
	return nil
}

func handleViewAdminUser(app *App, u *User, w http.ResponseWriter, r *http.Request) error {
	vars := mux.Vars(r)
	username := vars["username"]
	if username == "" {
		return impart.HTTPError{http.StatusFound, "/admin/users"}
	}

	p := struct {
		*UserPage
		*AdminPage
		Config  config.AppCfg
		Message string

		User        *User
		Colls       []inspectedCollection
		LastPost    string
		NewPassword string
		TotalPosts  int64
		ClearEmail  string
	}{
		AdminPage: NewAdminPage(app),
		Config:    app.cfg.App,
		Message:   r.FormValue("m"),
		Colls:     []inspectedCollection{},
	}

	var err error
	p.User, err = app.db.GetUserForAuth(username)
	if err != nil {
		if err == ErrUserNotFound {
			return err
		}
		log.Error("Could not get user: %v", err)
		return impart.HTTPError{http.StatusInternalServerError, err.Error()}
	}

	flashes, _ := getSessionFlashes(app, w, r, nil)
	for _, flash := range flashes {
		if strings.HasPrefix(flash, "SUCCESS: ") {
			p.NewPassword = strings.TrimPrefix(flash, "SUCCESS: ")
			p.ClearEmail = p.User.EmailClear(app.keys)
		}
	}
	p.UserPage = NewUserPage(app, r, u, p.User.Username, nil)
	p.TotalPosts = app.db.GetUserPostsCount(p.User.ID)
	lp, err := app.db.GetUserLastPostTime(p.User.ID)
	if err != nil {
		return impart.HTTPError{http.StatusInternalServerError, fmt.Sprintf("Could not get user's last post time: %v", err)}
	}
	if lp != nil {
		p.LastPost = lp.Format("January 2, 2006, 3:04 PM")
	}

	colls, err := app.db.GetCollections(p.User, app.cfg.App.Host)
	if err != nil {
		return impart.HTTPError{http.StatusInternalServerError, fmt.Sprintf("Could not get user's collections: %v", err)}
	}
	for _, c := range *colls {
		ic := inspectedCollection{
			CollectionObj: CollectionObj{Collection: c},
		}

		if app.cfg.App.Federation {
			folls, err := app.db.GetAPFollowers(&c)
			if err == nil {
				// TODO: handle error here (at least log it)
				ic.Followers = len(*folls)
			}
		}

		app.db.GetPostsCount(&ic.CollectionObj, true)

		lp, err := app.db.GetCollectionLastPostTime(c.ID)
		if err != nil {
			log.Error("Didn't get last post time for collection %d: %v", c.ID, err)
		}
		if lp != nil {
			ic.LastPost = lp.Format("January 2, 2006, 3:04 PM")
		}

		p.Colls = append(p.Colls, ic)
	}

	showUserPage(w, "view-user", p)
	return nil
}

func handleAdminToggleUserStatus(app *App, u *User, w http.ResponseWriter, r *http.Request) error {
	vars := mux.Vars(r)
	username := vars["username"]
	if username == "" {
		return impart.HTTPError{http.StatusFound, "/admin/users"}
	}

	user, err := app.db.GetUserForAuth(username)
	if err != nil {
		log.Error("failed to get user: %v", err)
		return impart.HTTPError{http.StatusInternalServerError, fmt.Sprintf("Could not get user from username: %v", err)}
	}
	if user.IsSilenced() {
		err = app.db.SetUserStatus(user.ID, UserActive)
	} else {
		err = app.db.SetUserStatus(user.ID, UserSilenced)
	}
	if err != nil {
		log.Error("toggle user silenced: %v", err)
		return impart.HTTPError{http.StatusInternalServerError, fmt.Sprintf("Could not toggle user status: %v", err)}
	}
	return impart.HTTPError{http.StatusFound, fmt.Sprintf("/admin/user/%s#status", username)}
}

func handleAdminResetUserPass(app *App, u *User, w http.ResponseWriter, r *http.Request) error {
	vars := mux.Vars(r)
	username := vars["username"]
	if username == "" {
		return impart.HTTPError{http.StatusFound, "/admin/users"}
	}

	// Generate new random password since none supplied
	pass := passgen.NewWordish()
	hashedPass, err := auth.HashPass([]byte(pass))
	if err != nil {
		return impart.HTTPError{http.StatusInternalServerError, fmt.Sprintf("Could not create password hash: %v", err)}
	}

	userIDVal := r.FormValue("user")
	log.Info("ADMIN: Changing user %s password", userIDVal)
	id, err := strconv.Atoi(userIDVal)
	if err != nil {
		return impart.HTTPError{http.StatusBadRequest, fmt.Sprintf("Invalid user ID: %v", err)}
	}

	err = app.db.ChangePassphrase(int64(id), true, "", hashedPass)
	if err != nil {
		return impart.HTTPError{http.StatusInternalServerError, fmt.Sprintf("Could not update passphrase: %v", err)}
	}
	log.Info("ADMIN: Successfully changed.")

	addSessionFlash(app, w, r, fmt.Sprintf("SUCCESS: %s", pass), nil)

	return impart.HTTPError{http.StatusFound, fmt.Sprintf("/admin/user/%s", username)}
}

func handleViewAdminPages(app *App, u *User, w http.ResponseWriter, r *http.Request) error {
	p := struct {
		*UserPage
		*AdminPage
		Config  config.AppCfg
		Message string

		Pages []*instanceContent
	}{
		UserPage:  NewUserPage(app, r, u, "Pages", nil),
		AdminPage: NewAdminPage(app),
		Config:    app.cfg.App,
		Message:   r.FormValue("m"),
	}

	var err error
	p.Pages, err = app.db.GetInstancePages()
	if err != nil {
		return impart.HTTPError{http.StatusInternalServerError, fmt.Sprintf("Could not get pages: %v", err)}
	}

	// Add in default pages
	var hasAbout, hasPrivacy bool
	for i, c := range p.Pages {
		if hasAbout && hasPrivacy {
			break
		}
		if c.ID == "about" {
			hasAbout = true
			if !c.Title.Valid {
				p.Pages[i].Title = defaultAboutTitle(app.cfg)
			}
		} else if c.ID == "privacy" {
			hasPrivacy = true
			if !c.Title.Valid {
				p.Pages[i].Title = defaultPrivacyTitle()
			}
		}
	}
	if !hasAbout {
		p.Pages = append(p.Pages, &instanceContent{
			ID:      "about",
			Title:   defaultAboutTitle(app.cfg),
			Content: defaultAboutPage(app.cfg),
			Updated: defaultPageUpdatedTime,
		})
	}
	if !hasPrivacy {
		p.Pages = append(p.Pages, &instanceContent{
			ID:      "privacy",
			Title:   defaultPrivacyTitle(),
			Content: defaultPrivacyPolicy(app.cfg),
			Updated: defaultPageUpdatedTime,
		})
	}

	showUserPage(w, "pages", p)
	return nil
}

func handleViewAdminPage(app *App, u *User, w http.ResponseWriter, r *http.Request) error {
	vars := mux.Vars(r)
	slug := vars["slug"]
	if slug == "" {
		return impart.HTTPError{http.StatusFound, "/admin/pages"}
	}

	p := struct {
		*UserPage
		*AdminPage
		Config  config.AppCfg
		Message string

		Banner  *instanceContent
		Content *instanceContent
	}{
		AdminPage: NewAdminPage(app),
		Config:    app.cfg.App,
		Message:   r.FormValue("m"),
	}

	var err error
	// Get pre-defined pages, or select slug
	if slug == "about" {
		p.Content, err = getAboutPage(app)
	} else if slug == "privacy" {
		p.Content, err = getPrivacyPage(app)
	} else if slug == "landing" {
		p.Banner, err = getLandingBanner(app)
		if err != nil {
			return impart.HTTPError{http.StatusInternalServerError, fmt.Sprintf("Could not get banner: %v", err)}
		}
		p.Content, err = getLandingBody(app)
		p.Content.ID = "landing"
	} else if slug == "reader" {
		p.Content, err = getReaderSection(app)
	} else {
		p.Content, err = app.db.GetDynamicContent(slug)
	}
	if err != nil {
		return impart.HTTPError{http.StatusInternalServerError, fmt.Sprintf("Could not get page: %v", err)}
	}
	title := "New page"
	if p.Content != nil {
		title = "Edit " + p.Content.ID
	} else {
		p.Content = &instanceContent{}
	}
	p.UserPage = NewUserPage(app, r, u, title, nil)

	showUserPage(w, "view-page", p)
	return nil
}

func handleAdminUpdateSite(app *App, u *User, w http.ResponseWriter, r *http.Request) error {
	vars := mux.Vars(r)
	id := vars["page"]

	// Validate
	if id != "about" && id != "privacy" && id != "landing" && id != "reader" {
		return impart.HTTPError{http.StatusNotFound, "No such page."}
	}

	var err error
	m := ""
	if id == "landing" {
		// Handle special landing page
		err = app.db.UpdateDynamicContent("landing-banner", "", r.FormValue("banner"), "section")
		if err != nil {
			m = "?m=" + err.Error()
			return impart.HTTPError{http.StatusFound, "/admin/page/" + id + m}
		}
		err = app.db.UpdateDynamicContent("landing-body", "", r.FormValue("content"), "section")
	} else if id == "reader" {
		// Update sections with titles
		err = app.db.UpdateDynamicContent(id, r.FormValue("title"), r.FormValue("content"), "section")
	} else {
		// Update page
		err = app.db.UpdateDynamicContent(id, r.FormValue("title"), r.FormValue("content"), "page")
	}
	if err != nil {
		m = "?m=" + err.Error()
	}
	return impart.HTTPError{http.StatusFound, "/admin/page/" + id + m}
}

func handleAdminUpdateConfig(apper Apper, u *User, w http.ResponseWriter, r *http.Request) error {
	apper.App().cfg.App.SiteName = r.FormValue("site_name")
	apper.App().cfg.App.SiteDesc = r.FormValue("site_desc")
	apper.App().cfg.App.Landing = r.FormValue("landing")
	apper.App().cfg.App.OpenRegistration = r.FormValue("open_registration") == "on"
	mul, err := strconv.Atoi(r.FormValue("min_username_len"))
	if err == nil {
		apper.App().cfg.App.MinUsernameLen = mul
	}
	mb, err := strconv.Atoi(r.FormValue("max_blogs"))
	if err == nil {
		apper.App().cfg.App.MaxBlogs = mb
	}
	apper.App().cfg.App.Federation = r.FormValue("federation") == "on"
	apper.App().cfg.App.PublicStats = r.FormValue("public_stats") == "on"
	apper.App().cfg.App.Private = r.FormValue("private") == "on"
	apper.App().cfg.App.LocalTimeline = r.FormValue("local_timeline") == "on"
	if apper.App().cfg.App.LocalTimeline && apper.App().timeline == nil {
		log.Info("Initializing local timeline...")
		initLocalTimeline(apper.App())
	}
	apper.App().cfg.App.UserInvites = r.FormValue("user_invites")
	if apper.App().cfg.App.UserInvites == "none" {
		apper.App().cfg.App.UserInvites = ""
	}
	apper.App().cfg.App.DefaultVisibility = r.FormValue("default_visibility")

	m := "?cm=Configuration+saved."
	err = apper.SaveConfig(apper.App().cfg)
	if err != nil {
		m = "?cm=" + err.Error()
	}
	return impart.HTTPError{http.StatusFound, "/admin/settings" + m + "#config"}
}

func updateAppStats() {
	sysStatus.Uptime = appstats.TimeSincePro(appStartTime)

	m := new(runtime.MemStats)
	runtime.ReadMemStats(m)
	sysStatus.NumGoroutine = runtime.NumGoroutine()

	sysStatus.MemAllocated = appstats.FileSize(int64(m.Alloc))
	sysStatus.MemTotal = appstats.FileSize(int64(m.TotalAlloc))
	sysStatus.MemSys = appstats.FileSize(int64(m.Sys))
	sysStatus.Lookups = m.Lookups
	sysStatus.MemMallocs = m.Mallocs
	sysStatus.MemFrees = m.Frees

	sysStatus.HeapAlloc = appstats.FileSize(int64(m.HeapAlloc))
	sysStatus.HeapSys = appstats.FileSize(int64(m.HeapSys))
	sysStatus.HeapIdle = appstats.FileSize(int64(m.HeapIdle))
	sysStatus.HeapInuse = appstats.FileSize(int64(m.HeapInuse))
	sysStatus.HeapReleased = appstats.FileSize(int64(m.HeapReleased))
	sysStatus.HeapObjects = m.HeapObjects

	sysStatus.StackInuse = appstats.FileSize(int64(m.StackInuse))
	sysStatus.StackSys = appstats.FileSize(int64(m.StackSys))
	sysStatus.MSpanInuse = appstats.FileSize(int64(m.MSpanInuse))
	sysStatus.MSpanSys = appstats.FileSize(int64(m.MSpanSys))
	sysStatus.MCacheInuse = appstats.FileSize(int64(m.MCacheInuse))
	sysStatus.MCacheSys = appstats.FileSize(int64(m.MCacheSys))
	sysStatus.BuckHashSys = appstats.FileSize(int64(m.BuckHashSys))
	sysStatus.GCSys = appstats.FileSize(int64(m.GCSys))
	sysStatus.OtherSys = appstats.FileSize(int64(m.OtherSys))

	sysStatus.NextGC = appstats.FileSize(int64(m.NextGC))
	sysStatus.LastGC = fmt.Sprintf("%.1fs", float64(time.Now().UnixNano()-int64(m.LastGC))/1000/1000/1000)
	sysStatus.PauseTotalNs = fmt.Sprintf("%.1fs", float64(m.PauseTotalNs)/1000/1000/1000)
	sysStatus.PauseNs = fmt.Sprintf("%.3fs", float64(m.PauseNs[(m.NumGC+255)%256])/1000/1000/1000)
	sysStatus.NumGC = m.NumGC
}

func adminResetPassword(app *App, u *User, newPass string) error {
	hashedPass, err := auth.HashPass([]byte(newPass))
	if err != nil {
		return impart.HTTPError{http.StatusInternalServerError, fmt.Sprintf("Could not create password hash: %v", err)}
	}

	err = app.db.ChangePassphrase(u.ID, true, "", hashedPass)
	if err != nil {
		return impart.HTTPError{http.StatusInternalServerError, fmt.Sprintf("Could not update passphrase: %v", err)}
	}
	return nil
}

func handleViewAdminUpdates(app *App, u *User, w http.ResponseWriter, r *http.Request) error {
	check := r.URL.Query().Get("check")

	if check == "now" && app.cfg.App.UpdateChecks {
		app.updates.CheckNow()
	}

	p := struct {
		*UserPage
		*AdminPage
		CurReleaseNotesURL    string
		LastChecked           string
		LastChecked8601       string
		LatestVersion         string
		LatestReleaseURL      string
		LatestReleaseNotesURL string
		CheckFailed           bool
	}{
		UserPage:  NewUserPage(app, r, u, "Updates", nil),
		AdminPage: NewAdminPage(app),
	}
	p.CurReleaseNotesURL = wfReleaseNotesURL(p.Version)
	if app.cfg.App.UpdateChecks {
		p.LastChecked = app.updates.lastCheck.Format("January 2, 2006, 3:04 PM")
		p.LastChecked8601 = app.updates.lastCheck.Format("2006-01-02T15:04:05Z")
		p.LatestVersion = app.updates.LatestVersion()
		p.LatestReleaseURL = app.updates.ReleaseURL()
		p.LatestReleaseNotesURL = app.updates.ReleaseNotesURL()
		p.UpdateAvailable = app.updates.AreAvailable()
		p.CheckFailed = app.updates.checkError != nil
	}

	showUserPage(w, "app-updates", p)
	return nil
}
