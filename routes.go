package writefreely

import (
	"github.com/gorilla/mux"
	"github.com/writeas/go-nodeinfo"
	"github.com/writeas/web-core/log"
	"github.com/writeas/writefreely/config"
	"net/http"
	"strings"
)

func initRoutes(handler *Handler, r *mux.Router, cfg *config.Config, db *datastore) {
	hostSubroute := cfg.App.Host[strings.Index(cfg.App.Host, "://")+3:]
	if cfg.App.SingleUser {
		hostSubroute = "{domain}"
	} else {
		if strings.HasPrefix(hostSubroute, "localhost") {
			hostSubroute = "localhost"
		}
	}

	if cfg.App.SingleUser {
		log.Info("Adding %s routes (single user)...", hostSubroute)
	} else {
		log.Info("Adding %s routes (multi-user)...", hostSubroute)
	}

	// Primary app routes
	write := r.Host(hostSubroute).Subrouter()

	// Federation endpoints
	// nodeinfo
	niCfg := nodeInfoConfig(cfg)
	ni := nodeinfo.NewService(*niCfg, nodeInfoResolver{cfg, db})
	write.HandleFunc(nodeinfo.NodeInfoPath, handler.LogHandlerFunc(http.HandlerFunc(ni.NodeInfoDiscover)))
	write.HandleFunc(niCfg.InfoURL, handler.LogHandlerFunc(http.HandlerFunc(ni.NodeInfo)))

	// Handle logged in user sections
	me := write.PathPrefix("/me").Subrouter()
	me.HandleFunc("/", handler.Redirect("/me", UserLevelUser))
	me.HandleFunc("/c", handler.Redirect("/me/c/", UserLevelUser)).Methods("GET")
	me.HandleFunc("/c/", handler.User(viewCollections)).Methods("GET")
	me.HandleFunc("/c/{collection}", handler.User(viewEditCollection)).Methods("GET")
	me.HandleFunc("/c/{collection}/stats", handler.User(viewStats)).Methods("GET")
	me.HandleFunc("/posts", handler.Redirect("/me/posts/", UserLevelUser)).Methods("GET")
	me.HandleFunc("/posts/", handler.User(viewArticles)).Methods("GET")
	me.HandleFunc("/posts/export.csv", handler.Download(viewExportPosts, UserLevelUser)).Methods("GET")
	me.HandleFunc("/posts/export.zip", handler.Download(viewExportPosts, UserLevelUser)).Methods("GET")
	me.HandleFunc("/posts/export.json", handler.Download(viewExportPosts, UserLevelUser)).Methods("GET")
	me.HandleFunc("/export", handler.User(viewExportOptions)).Methods("GET")
	me.HandleFunc("/export.json", handler.Download(viewExportFull, UserLevelUser)).Methods("GET")
	me.HandleFunc("/settings", handler.User(viewSettings)).Methods("GET")
	me.HandleFunc("/logout", handler.Web(viewLogout, UserLevelNone)).Methods("GET")

	write.HandleFunc("/api/me", handler.All(viewMeAPI)).Methods("GET")
	apiMe := write.PathPrefix("/api/me/").Subrouter()
	apiMe.HandleFunc("/", handler.All(viewMeAPI)).Methods("GET")
	apiMe.HandleFunc("/posts", handler.UserAPI(viewMyPostsAPI)).Methods("GET")
	apiMe.HandleFunc("/collections", handler.UserAPI(viewMyCollectionsAPI)).Methods("GET")
	apiMe.HandleFunc("/password", handler.All(updatePassphrase)).Methods("POST")
	apiMe.HandleFunc("/self", handler.All(updateSettings)).Methods("POST")

	// Sign up validation
	write.HandleFunc("/api/alias", handler.All(handleUsernameCheck)).Methods("POST")

	// Handle collections
	write.HandleFunc("/api/collections", handler.All(newCollection)).Methods("POST")
	apiColls := write.PathPrefix("/api/collections/").Subrouter()
	apiColls.HandleFunc("/{alias:[0-9a-zA-Z\\-]+}", handler.All(fetchCollection)).Methods("GET")
	apiColls.HandleFunc("/{alias:[0-9a-zA-Z\\-]+}", handler.All(existingCollection)).Methods("POST", "DELETE")
	apiColls.HandleFunc("/{alias}/posts", handler.All(fetchCollectionPosts)).Methods("GET")
	apiColls.HandleFunc("/{alias}/posts", handler.All(newPost)).Methods("POST")
	apiColls.HandleFunc("/{alias}/posts/{post}", handler.All(fetchPost)).Methods("GET")
	apiColls.HandleFunc("/{alias}/posts/{post:[a-zA-Z0-9]{10}}", handler.All(existingPost)).Methods("POST")
	apiColls.HandleFunc("/{alias}/posts/{post}/{property}", handler.All(fetchPostProperty)).Methods("GET")
	apiColls.HandleFunc("/{alias}/collect", handler.All(addPost)).Methods("POST")
	apiColls.HandleFunc("/{alias}/pin", handler.All(pinPost)).Methods("POST")
	apiColls.HandleFunc("/{alias}/unpin", handler.All(pinPost)).Methods("POST")

	// Handle posts
	write.HandleFunc("/api/posts", handler.All(newPost)).Methods("POST")
	posts := write.PathPrefix("/api/posts/").Subrouter()
	posts.HandleFunc("/{post:[a-zA-Z0-9]{10}}", handler.All(fetchPost)).Methods("GET")
	posts.HandleFunc("/{post:[a-zA-Z0-9]{10}}", handler.All(existingPost)).Methods("POST", "PUT")
	posts.HandleFunc("/{post:[a-zA-Z0-9]{10}}", handler.All(deletePost)).Methods("DELETE")
	posts.HandleFunc("/{post:[a-zA-Z0-9]{10}}/{property}", handler.All(fetchPostProperty)).Methods("GET")
	posts.HandleFunc("/claim", handler.All(addPost)).Methods("POST")
	posts.HandleFunc("/disperse", handler.All(dispersePost)).Methods("POST")

	if cfg.App.SingleUser {
		write.HandleFunc("/me/new", handler.Web(handleViewPad, UserLevelOptional)).Methods("GET")
	} else {
		write.HandleFunc("/new", handler.Web(handleViewPad, UserLevelOptional)).Methods("GET")
	}

	// All the existing stuff
	write.HandleFunc("/{action}/edit", handler.Web(handleViewPad, UserLevelOptional)).Methods("GET")
	write.HandleFunc("/{action}/meta", handler.Web(handleViewMeta, UserLevelOptional)).Methods("GET")
	// Collections
	if cfg.App.SingleUser {
		RouteCollections(handler, write.PathPrefix("/").Subrouter())
	} else {
		write.HandleFunc("/{prefix:[@~$!\\-+]}{collection}", handler.Web(handleViewCollection, UserLevelOptional))
		write.HandleFunc("/{collection}/", handler.Web(handleViewCollection, UserLevelOptional))
		RouteCollections(handler, write.PathPrefix("/{prefix:[@~$!\\-+]?}{collection}").Subrouter())
		// Posts
		write.HandleFunc("/{post}", handler.Web(handleViewPost, UserLevelOptional))
	}
	write.HandleFunc("/", handler.Web(handleViewHome, UserLevelOptional))
}

func RouteCollections(handler *Handler, r *mux.Router) {
	r.HandleFunc("/page/{page:[0-9]+}", handler.Web(handleViewCollection, UserLevelOptional))
	r.HandleFunc("/tag:{tag}", handler.Web(handleViewCollectionTag, UserLevelOptional))
	r.HandleFunc("/tag:{tag}/feed/", handler.Web(ViewFeed, UserLevelOptional))
	r.HandleFunc("/tags/{tag}", handler.Web(handleViewCollectionTag, UserLevelOptional))
	r.HandleFunc("/sitemap.xml", handler.All(handleViewSitemap))
	r.HandleFunc("/feed/", handler.All(ViewFeed))
	r.HandleFunc("/{slug}", handler.Web(viewCollectionPost, UserLevelOptional))
	r.HandleFunc("/{slug}/edit", handler.Web(handleViewPad, UserLevelUser))
	r.HandleFunc("/{slug}/edit/meta", handler.Web(handleViewMeta, UserLevelUser))
	r.HandleFunc("/{slug}/", handler.Web(handleCollectionPostRedirect, UserLevelOptional)).Methods("GET")
}
