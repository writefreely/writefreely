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
	"fmt"
	"github.com/gogits/gogs/pkg/tool"
	"github.com/gorilla/mux"
	"github.com/writeas/impart"
	"github.com/writeas/web-core/auth"
	"github.com/writeas/web-core/log"
	"github.com/writeas/writefreely/config"
	"net/http"
	"runtime"
	"strconv"
	"time"
)

var (
	appStartTime = time.Now()
	sysStatus    systemStatus
)

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

func handleViewAdminDash(app *app, u *User, w http.ResponseWriter, r *http.Request) error {
	updateAppStats()
	p := struct {
		*UserPage
		SysStatus systemStatus
		Config    config.AppCfg

		Message, ConfigMessage string

		AboutPage, PrivacyPage string
	}{
		UserPage:  NewUserPage(app, r, u, "Admin", nil),
		SysStatus: sysStatus,
		Config:    app.cfg.App,

		Message:       r.FormValue("m"),
		ConfigMessage: r.FormValue("cm"),
	}

	var err error
	p.AboutPage, err = getAboutPage(app)
	if err != nil {
		return err
	}

	p.PrivacyPage, _, err = getPrivacyPage(app)
	if err != nil {
		return err
	}

	showUserPage(w, "admin", p)
	return nil
}

func handleViewAdminUsers(app *app, u *User, w http.ResponseWriter, r *http.Request) error {
	p := struct {
		*UserPage
		Config  config.AppCfg
		Message string

		Users *[]User
	}{
		UserPage: NewUserPage(app, r, u, "Users", nil),
		Config:   app.cfg.App,
		Message:  r.FormValue("m"),
	}

	var err error
	p.Users, err = app.db.GetAllUsers(1)
	if err != nil {
		return impart.HTTPError{http.StatusInternalServerError, fmt.Sprintf("Could not get users: %v", err)}
	}

	showUserPage(w, "users", p)
	return nil
}

func handleViewAdminUser(app *app, u *User, w http.ResponseWriter, r *http.Request) error {
	vars := mux.Vars(r)
	username := vars["username"]
	if username == "" {
		return impart.HTTPError{http.StatusFound, "/admin/users"}
	}

	p := struct {
		*UserPage
		Config  config.AppCfg
		Message string

		User     *User
		Colls    []inspectedCollection
		LastPost string

		TotalPosts int64
	}{
		Config:  app.cfg.App,
		Message: r.FormValue("m"),
		Colls:   []inspectedCollection{},
	}

	var err error
	p.User, err = app.db.GetUserForAuth(username)
	if err != nil {
		return impart.HTTPError{http.StatusInternalServerError, fmt.Sprintf("Could not get user: %v", err)}
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

	colls, err := app.db.GetCollections(p.User)
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

func handleAdminUpdateSite(app *app, u *User, w http.ResponseWriter, r *http.Request) error {
	vars := mux.Vars(r)
	id := vars["page"]

	// Validate
	if id != "about" && id != "privacy" {
		return impart.HTTPError{http.StatusNotFound, "No such page."}
	}

	// Update page
	m := ""
	err := app.db.UpdateDynamicContent(id, r.FormValue("content"))
	if err != nil {
		m = "?m=" + err.Error()
	}
	return impart.HTTPError{http.StatusFound, "/admin" + m + "#page-" + id}
}

func handleAdminUpdateConfig(app *app, u *User, w http.ResponseWriter, r *http.Request) error {
	app.cfg.App.SiteName = r.FormValue("site_name")
	app.cfg.App.SiteDesc = r.FormValue("site_desc")
	app.cfg.App.OpenRegistration = r.FormValue("open_registration") == "on"
	mul, err := strconv.Atoi(r.FormValue("min_username_len"))
	if err == nil {
		app.cfg.App.MinUsernameLen = mul
	}
	mb, err := strconv.Atoi(r.FormValue("max_blogs"))
	if err == nil {
		app.cfg.App.MaxBlogs = mb
	}
	app.cfg.App.Federation = r.FormValue("federation") == "on"
	app.cfg.App.PublicStats = r.FormValue("public_stats") == "on"
	app.cfg.App.Private = r.FormValue("private") == "on"
	app.cfg.App.LocalTimeline = r.FormValue("local_timeline") == "on"
	if app.cfg.App.LocalTimeline && app.timeline == nil {
		log.Info("Initializing local timeline...")
		initLocalTimeline(app)
	}

	m := "?cm=Configuration+saved."
	err = config.Save(app.cfg, app.cfgFile)
	if err != nil {
		m = "?cm=" + err.Error()
	}
	return impart.HTTPError{http.StatusFound, "/admin" + m + "#config"}
}

func updateAppStats() {
	sysStatus.Uptime = tool.TimeSincePro(appStartTime)

	m := new(runtime.MemStats)
	runtime.ReadMemStats(m)
	sysStatus.NumGoroutine = runtime.NumGoroutine()

	sysStatus.MemAllocated = tool.FileSize(int64(m.Alloc))
	sysStatus.MemTotal = tool.FileSize(int64(m.TotalAlloc))
	sysStatus.MemSys = tool.FileSize(int64(m.Sys))
	sysStatus.Lookups = m.Lookups
	sysStatus.MemMallocs = m.Mallocs
	sysStatus.MemFrees = m.Frees

	sysStatus.HeapAlloc = tool.FileSize(int64(m.HeapAlloc))
	sysStatus.HeapSys = tool.FileSize(int64(m.HeapSys))
	sysStatus.HeapIdle = tool.FileSize(int64(m.HeapIdle))
	sysStatus.HeapInuse = tool.FileSize(int64(m.HeapInuse))
	sysStatus.HeapReleased = tool.FileSize(int64(m.HeapReleased))
	sysStatus.HeapObjects = m.HeapObjects

	sysStatus.StackInuse = tool.FileSize(int64(m.StackInuse))
	sysStatus.StackSys = tool.FileSize(int64(m.StackSys))
	sysStatus.MSpanInuse = tool.FileSize(int64(m.MSpanInuse))
	sysStatus.MSpanSys = tool.FileSize(int64(m.MSpanSys))
	sysStatus.MCacheInuse = tool.FileSize(int64(m.MCacheInuse))
	sysStatus.MCacheSys = tool.FileSize(int64(m.MCacheSys))
	sysStatus.BuckHashSys = tool.FileSize(int64(m.BuckHashSys))
	sysStatus.GCSys = tool.FileSize(int64(m.GCSys))
	sysStatus.OtherSys = tool.FileSize(int64(m.OtherSys))

	sysStatus.NextGC = tool.FileSize(int64(m.NextGC))
	sysStatus.LastGC = fmt.Sprintf("%.1fs", float64(time.Now().UnixNano()-int64(m.LastGC))/1000/1000/1000)
	sysStatus.PauseTotalNs = fmt.Sprintf("%.1fs", float64(m.PauseTotalNs)/1000/1000/1000)
	sysStatus.PauseNs = fmt.Sprintf("%.3fs", float64(m.PauseNs[(m.NumGC+255)%256])/1000/1000/1000)
	sysStatus.NumGC = m.NumGC
}

func adminResetPassword(app *app, u *User, newPass string) error {
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
