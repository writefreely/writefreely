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
	isSingleUser := !cfg.App.MultiUser

	// Write.as router
	hostSubroute := cfg.Server.Host[strings.Index(cfg.Server.Host, "://")+3:]
	if isSingleUser {
		hostSubroute = "{domain}"
	} else {
		if strings.HasPrefix(hostSubroute, "localhost") {
			hostSubroute = "localhost"
		}
	}

	if isSingleUser {
		log.Info("Adding %s routes (single user)...", hostSubroute)

		return
	}

	// Primary app routes
	log.Info("Adding %s routes (multi-user)...", hostSubroute)
	write := r.Host(hostSubroute).Subrouter()

	// Federation endpoints
	// nodeinfo
	niCfg := nodeInfoConfig(cfg)
	ni := nodeinfo.NewService(*niCfg, nodeInfoResolver{cfg, db})
	write.HandleFunc(nodeinfo.NodeInfoPath, handler.LogHandlerFunc(http.HandlerFunc(ni.NodeInfoDiscover)))
	write.HandleFunc(niCfg.InfoURL, handler.LogHandlerFunc(http.HandlerFunc(ni.NodeInfo)))
}
