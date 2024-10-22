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
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/gorilla/csrf"
	"github.com/gorilla/mux"
	"github.com/writeas/go-webfinger"
	"github.com/writeas/web-core/log"
	"github.com/writefreely/go-nodeinfo"
)

// InitStaticRoutes adds routes for serving static files.
// TODO: this should just be a func, not method
func (app *App) InitStaticRoutes(r *mux.Router) {
	// Handle static files
	fs := http.FileServer(http.Dir(filepath.Join(app.cfg.Server.StaticParentDir, staticDir)))
	fs = cacheControl(fs)
	app.shttp = http.NewServeMux()
	app.shttp.Handle("/", fs)
	r.PathPrefix("/").Handler(fs)
}

// InitRoutes adds dynamic routes for the given mux.Router.
func InitRoutes(apper Apper, r *mux.Router) *mux.Router {
	// Create handler
	handler := NewWFHandler(apper)

	// Set up routes
	hostSubroute := apper.App().cfg.App.Host[strings.Index(apper.App().cfg.App.Host, "://")+3:]
	if apper.App().cfg.App.SingleUser {
		hostSubroute = "{domain}"
	} else {
		if strings.HasPrefix(hostSubroute, "localhost") {
			hostSubroute = "localhost"
		}
	}

	if apper.App().cfg.App.SingleUser {
		log.Info("Adding %s routes (single user)...", hostSubroute)
	} else {
		log.Info("Adding %s routes (multi-user)...", hostSubroute)
	}

	// Primary app routes
	write := r.PathPrefix("/").Subrouter()

	// Federation endpoint configurations
	wf := webfinger.Default(wfResolver{apper.App().db, apper.App().cfg})
	wf.NoTLSHandler = nil

	// Federation endpoints
	// host-meta
	write.HandleFunc("/.well-known/host-meta", handler.Web(handleViewHostMeta, UserLevelReader))
	// webfinger
	write.HandleFunc(webfinger.WebFingerPath, handler.LogHandlerFunc(http.HandlerFunc(wf.Webfinger)))
	// nodeinfo
	niCfg := nodeInfoConfig(apper.App().db, apper.App().cfg)
	ni := nodeinfo.NewService(*niCfg, nodeInfoResolver{apper.App().cfg, apper.App().db})
	write.HandleFunc(nodeinfo.NodeInfoPath, handler.LogHandlerFunc(http.HandlerFunc(ni.NodeInfoDiscover)))
	write.HandleFunc(niCfg.InfoURL, handler.LogHandlerFunc(http.HandlerFunc(ni.NodeInfo)))

	// handle mentions
	write.HandleFunc("/@/{handle}", handler.Web(handleViewMention, UserLevelReader))

	configureSlackOauth(handler, write, apper.App())
	configureWriteAsOauth(handler, write, apper.App())
	configureGitlabOauth(handler, write, apper.App())
	configureGenericOauth(handler, write, apper.App())
	configureGiteaOauth(handler, write, apper.App())

	// Set up dynamic page handlers
	// Handle auth
	auth := write.PathPrefix("/api/auth/").Subrouter()
	if apper.App().cfg.App.OpenRegistration {
		auth.HandleFunc("/signup", handler.All(apiSignup)).Methods("POST")
	}
	auth.HandleFunc("/login", handler.All(login)).Methods("POST")
	auth.HandleFunc("/read", handler.WebErrors(handleWebCollectionUnlock, UserLevelNone)).Methods("POST")
	auth.HandleFunc("/me", handler.All(handleAPILogout)).Methods("DELETE")

	// Handle logged in user sections
	me := write.PathPrefix("/me").Subrouter()
	me.HandleFunc("/", handler.Redirect("/me", UserLevelUser))
	me.HandleFunc("/c", handler.Redirect("/me/c/", UserLevelUser)).Methods("GET")
	me.HandleFunc("/c/", handler.User(viewCollections)).Methods("GET")
	me.HandleFunc("/c/{collection}", handler.User(viewEditCollection)).Methods("GET")
	me.HandleFunc("/c/{collection}/stats", handler.User(viewStats)).Methods("GET")
	me.HandleFunc("/c/{collection}/subscribers", handler.User(handleViewSubscribers)).Methods("GET")
	me.Path("/delete").Handler(csrf.Protect(apper.App().keys.CSRFKey)(handler.User(handleUserDelete))).Methods("POST")
	me.HandleFunc("/posts", handler.Redirect("/me/posts/", UserLevelUser)).Methods("GET")
	me.HandleFunc("/posts/", handler.User(viewArticles)).Methods("GET")
	me.HandleFunc("/posts/export.csv", handler.Download(viewExportPosts, UserLevelUser)).Methods("GET")
	me.HandleFunc("/posts/export.zip", handler.Download(viewExportPosts, UserLevelUser)).Methods("GET")
	me.HandleFunc("/posts/export.json", handler.Download(viewExportPosts, UserLevelUser)).Methods("GET")
	me.HandleFunc("/export", handler.User(viewExportOptions)).Methods("GET")
	me.HandleFunc("/export.json", handler.Download(viewExportFull, UserLevelUser)).Methods("GET")
	me.HandleFunc("/import", handler.User(viewImport)).Methods("GET")
	me.Path("/settings").Handler(csrf.Protect(apper.App().keys.CSRFKey)(handler.User(viewSettings))).Methods("GET")
	me.HandleFunc("/invites", handler.User(handleViewUserInvites)).Methods("GET")
	me.HandleFunc("/logout", handler.Web(viewLogout, UserLevelNone)).Methods("GET")

	write.HandleFunc("/api/me", handler.All(viewMeAPI)).Methods("GET")
	apiMe := write.PathPrefix("/api/me/").Subrouter()
	apiMe.HandleFunc("/", handler.All(viewMeAPI)).Methods("GET")
	apiMe.HandleFunc("/posts", handler.UserWebAPI(viewMyPostsAPI)).Methods("GET")
	apiMe.HandleFunc("/collections", handler.UserAPI(viewMyCollectionsAPI)).Methods("GET")
	apiMe.HandleFunc("/password", handler.All(updatePassphrase)).Methods("POST")
	apiMe.HandleFunc("/self", handler.All(updateSettings)).Methods("POST")
	apiMe.HandleFunc("/invites", handler.User(handleCreateUserInvite)).Methods("POST")
	apiMe.HandleFunc("/import", handler.User(handleImport)).Methods("POST")
	apiMe.HandleFunc("/oauth/remove", handler.User(removeOauth)).Methods("POST")

	// Sign up validation
	write.HandleFunc("/api/alias", handler.All(handleUsernameCheck)).Methods("POST")

	write.HandleFunc("/api/markdown", handler.All(handleRenderMarkdown)).Methods("POST")

	instanceURL, _ := url.Parse(apper.App().Config().App.Host)
	host := instanceURL.Host

	// Handle collections
	write.HandleFunc("/api/collections", handler.All(newCollection)).Methods("POST")
	apiColls := write.PathPrefix("/api/collections/").Subrouter()
	apiColls.HandleFunc("/monetization-pointer", handler.PlainTextAPI(handleSPSPEndpoint)).Methods("GET")
	apiColls.HandleFunc("/"+host, handler.AllReader(fetchCollection)).Methods("GET")
	apiColls.HandleFunc("/{alias:[0-9a-zA-Z\\-]+}", handler.AllReader(fetchCollection)).Methods("GET")
	apiColls.HandleFunc("/{alias:[0-9a-zA-Z\\-]+}", handler.All(existingCollection)).Methods("POST", "DELETE")
	apiColls.HandleFunc("/{alias}/posts", handler.AllReader(fetchCollectionPosts)).Methods("GET")
	apiColls.HandleFunc("/{alias}/posts", handler.All(newPost)).Methods("POST")
	apiColls.HandleFunc("/{alias}/posts/{post}", handler.AllReader(fetchPost)).Methods("GET")
	apiColls.HandleFunc("/{alias}/posts/{post:[a-zA-Z0-9]{10}}", handler.All(existingPost)).Methods("POST")
	apiColls.HandleFunc("/{alias}/posts/{post}/splitcontent", handler.AllReader(handleGetSplitContent)).Methods("GET", "POST")
	apiColls.HandleFunc("/{alias}/posts/{post}/{property}", handler.AllReader(fetchPostProperty)).Methods("GET")
	apiColls.HandleFunc("/{alias}/collect", handler.All(addPost)).Methods("POST")
	apiColls.HandleFunc("/{alias}/pin", handler.All(pinPost)).Methods("POST")
	apiColls.HandleFunc("/{alias}/unpin", handler.All(pinPost)).Methods("POST")
	apiColls.HandleFunc("/{alias}/email/subscribe", handler.All(handleCreateEmailSubscription)).Methods("POST")
	apiColls.HandleFunc("/{alias}/email/subscribe", handler.All(handleDeleteEmailSubscription)).Methods("DELETE")
	apiColls.HandleFunc("/{collection}/email/unsubscribe", handler.All(handleDeleteEmailSubscription)).Methods("GET")
	apiColls.HandleFunc("/{alias}/inbox", handler.All(handleFetchCollectionInbox)).Methods("POST")
	apiColls.HandleFunc("/{alias}/outbox", handler.AllReader(handleFetchCollectionOutbox)).Methods("GET")
	apiColls.HandleFunc("/{alias}/following", handler.AllReader(handleFetchCollectionFollowing)).Methods("GET")
	apiColls.HandleFunc("/{alias}/followers", handler.AllReader(handleFetchCollectionFollowers)).Methods("GET")

	// Handle posts
	write.HandleFunc("/api/posts", handler.All(newPost)).Methods("POST")
	posts := write.PathPrefix("/api/posts/").Subrouter()
	posts.HandleFunc("/{post:[a-zA-Z0-9]+}", handler.AllReader(fetchPost)).Methods("GET")
	posts.HandleFunc("/{post:[a-zA-Z0-9]+}", handler.All(existingPost)).Methods("POST", "PUT")
	posts.HandleFunc("/{post:[a-zA-Z0-9]+}", handler.All(deletePost)).Methods("DELETE")
	posts.HandleFunc("/{post:[a-zA-Z0-9]+}/{property}", handler.AllReader(fetchPostProperty)).Methods("GET")
	posts.HandleFunc("/claim", handler.All(addPost)).Methods("POST")
	posts.HandleFunc("/disperse", handler.All(dispersePost)).Methods("POST")

	write.HandleFunc("/auth/signup", handler.Web(handleWebSignup, UserLevelNoneRequired)).Methods("POST")
	write.HandleFunc("/auth/login", handler.Web(webLogin, UserLevelNoneRequired)).Methods("POST")

	write.HandleFunc("/admin", handler.Admin(handleViewAdminDash)).Methods("GET")
	write.HandleFunc("/admin/monitor", handler.Admin(handleViewAdminMonitor)).Methods("GET")
	write.HandleFunc("/admin/settings", handler.Admin(handleViewAdminSettings)).Methods("GET")
	write.HandleFunc("/admin/users", handler.Admin(handleViewAdminUsers)).Methods("GET")
	write.HandleFunc("/admin/user/{username}", handler.Admin(handleViewAdminUser)).Methods("GET")
	write.HandleFunc("/admin/user/{username}/delete", handler.Admin(handleAdminDeleteUser)).Methods("POST")
	write.HandleFunc("/admin/user/{username}/status", handler.Admin(handleAdminToggleUserStatus)).Methods("POST")
	write.HandleFunc("/admin/user/{username}/passphrase", handler.Admin(handleAdminResetUserPass)).Methods("POST")
	write.HandleFunc("/admin/pages", handler.Admin(handleViewAdminPages)).Methods("GET")
	write.HandleFunc("/admin/page/{slug}", handler.Admin(handleViewAdminPage)).Methods("GET")
	write.HandleFunc("/admin/update/config", handler.AdminApper(handleAdminUpdateConfig)).Methods("POST")
	write.HandleFunc("/admin/update/{page}", handler.Admin(handleAdminUpdateSite)).Methods("POST")
	write.HandleFunc("/admin/updates", handler.Admin(handleViewAdminUpdates)).Methods("GET")

	// Handle special pages first
	write.Path("/reset").Handler(csrf.Protect(apper.App().keys.CSRFKey)(handler.Web(viewResetPassword, UserLevelNoneRequired)))
	write.HandleFunc("/login", handler.Web(viewLogin, UserLevelNoneRequired))
	write.HandleFunc("/signup", handler.Web(handleViewLanding, UserLevelNoneRequired))
	write.HandleFunc("/invite/{code:[a-zA-Z0-9]+}", handler.Web(handleViewInvite, UserLevelOptional)).Methods("GET")
	// TODO: show a reader-specific 404 page if the function is disabled
	write.HandleFunc("/read", handler.Web(viewLocalTimeline, UserLevelReader))
	RouteRead(handler, UserLevelReader, write.PathPrefix("/read").Subrouter())

	draftEditPrefix := ""
	if apper.App().cfg.App.SingleUser {
		draftEditPrefix = "/d"
		write.HandleFunc("/me/new", handler.Web(handleViewPad, UserLevelUser)).Methods("GET")
	} else {
		write.HandleFunc("/new", handler.Web(handleViewPad, UserLevelUser)).Methods("GET")
	}

	// All the existing stuff
	write.HandleFunc(draftEditPrefix+"/{action}/edit", handler.Web(handleViewPad, UserLevelUser)).Methods("GET")
	write.HandleFunc(draftEditPrefix+"/{action}/meta", handler.Web(handleViewMeta, UserLevelUser)).Methods("GET")
	// Collections
	if apper.App().cfg.App.SingleUser {
		RouteCollections(handler, write.PathPrefix("/").Subrouter())
	} else {
		write.HandleFunc("/{prefix:[@~$!\\-+]}{collection}", handler.Web(handleViewCollection, UserLevelReader))
		write.HandleFunc("/{collection}/", handler.Web(handleViewCollection, UserLevelReader))
		RouteCollections(handler, write.PathPrefix("/{prefix:[@~$!\\-+]?}{collection}").Subrouter())
		// Posts
	}
	write.HandleFunc(draftEditPrefix+"/{post}", handler.Web(handleViewPost, UserLevelOptional))
	write.HandleFunc("/", handler.Web(handleViewHome, UserLevelOptional))

	return r
}

func RouteCollections(handler *Handler, r *mux.Router) {
	r.HandleFunc("/logout", handler.Web(handleLogOutCollection, UserLevelOptional))
	r.HandleFunc("/page/{page:[0-9]+}", handler.Web(handleViewCollection, UserLevelReader))
	r.HandleFunc("/archive/", handler.Web(handleViewCollection, UserLevelReader))
	r.HandleFunc("/{archive:archive}/page/{page:[0-9]+}", handler.Web(handleViewCollection, UserLevelReader))
	r.HandleFunc("/lang:{lang:[a-z]{2}}", handler.Web(handleViewCollectionLang, UserLevelOptional))
	r.HandleFunc("/lang:{lang:[a-z]{2}}/page/{page:[0-9]+}", handler.Web(handleViewCollectionLang, UserLevelOptional))
	r.HandleFunc("/tag:{tag}", handler.Web(handleViewCollectionTag, UserLevelReader))
	r.HandleFunc("/tag:{tag}/page/{page:[0-9]+}", handler.Web(handleViewCollectionTag, UserLevelReader))
	r.HandleFunc("/tag:{tag}/feed/", handler.Web(ViewFeed, UserLevelReader))
	r.HandleFunc("/sitemap.xml", handler.AllReader(handleViewSitemap))
	r.HandleFunc("/feed/", handler.AllReader(ViewFeed))
	r.HandleFunc("/email/confirm/{subscriber}", handler.All(handleConfirmEmailSubscription)).Methods("GET")
	r.HandleFunc("/email/unsubscribe/{subscriber}", handler.All(handleDeleteEmailSubscription)).Methods("GET")
	r.HandleFunc("/{slug}", handler.CollectionPostOrStatic)
	r.HandleFunc("/{slug}/edit", handler.Web(handleViewPad, UserLevelUser))
	r.HandleFunc("/{slug}/edit/meta", handler.Web(handleViewMeta, UserLevelUser))
	r.HandleFunc("/{slug}/", handler.Web(handleCollectionPostRedirect, UserLevelReader)).Methods("GET")
}

func RouteRead(handler *Handler, readPerm UserLevelFunc, r *mux.Router) {
	r.HandleFunc("/api/posts", handler.Web(viewLocalTimelineAPI, readPerm))
	r.HandleFunc("/p/{page}", handler.Web(viewLocalTimeline, readPerm))
	r.HandleFunc("/feed/", handler.Web(viewLocalTimelineFeed, readPerm))
	r.HandleFunc("/t/{tag}", handler.Web(viewLocalTimeline, readPerm))
	r.HandleFunc("/a/{post}", handler.Web(handlePostIDRedirect, readPerm))
	r.HandleFunc("/{author}", handler.Web(viewLocalTimeline, readPerm))
	r.HandleFunc("/", handler.Web(viewLocalTimeline, readPerm))
}
