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

	// Handle posts
	write.HandleFunc("/api/posts", handler.All(newPost)).Methods("POST")
	posts := write.PathPrefix("/api/posts/").Subrouter()
	posts.HandleFunc("/{post:[a-zA-Z0-9]{10}}", handler.All(fetchPost)).Methods("GET")
	posts.HandleFunc("/{post:[a-zA-Z0-9]{10}}", handler.All(existingPost)).Methods("POST", "PUT")
	posts.HandleFunc("/{post:[a-zA-Z0-9]{10}}", handler.All(deletePost)).Methods("DELETE")
	posts.HandleFunc("/{post:[a-zA-Z0-9]{10}}/{property}", handler.All(fetchPostProperty)).Methods("GET")
	posts.HandleFunc("/claim", handler.All(addPost)).Methods("POST")
	posts.HandleFunc("/disperse", handler.All(dispersePost)).Methods("POST")

	// All the existing stuff
	write.HandleFunc("/{action}/edit", handler.Web(handleViewPad, UserLevelOptional)).Methods("GET")
	write.HandleFunc("/{action}/meta", handler.Web(handleViewMeta, UserLevelOptional)).Methods("GET")
	// Collections
	if cfg.App.SingleUser {
	} else {
		// Posts
		write.HandleFunc("/{post}", handler.Web(handleViewPost, UserLevelOptional))
	}
}
