package writefreely

import (
	"github.com/gorilla/mux"
	"github.com/writeas/web-core/log"
	"github.com/writeas/writefreely/config"
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
}
