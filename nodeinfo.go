package writefreely

import (
	"github.com/writeas/go-nodeinfo"
	"github.com/writeas/web-core/log"
	"github.com/writeas/writefreely/config"
)

type nodeInfoResolver struct {
	cfg *config.Config
	db  *datastore
}

func nodeInfoConfig(cfg *config.Config) *nodeinfo.Config {
	name := cfg.App.SiteName
	return &nodeinfo.Config{
		BaseURL: cfg.App.Host,
		InfoURL: "/api/nodeinfo",

		Metadata: nodeinfo.Metadata{
			NodeName:        name,
			NodeDescription: "Minimal, federated blogging platform.",
			Private:         cfg.App.Private,
			Software: nodeinfo.SoftwareMeta{
				HomePage: softwareURL,
				GitHub:   "https://github.com/writeas/writefreely",
				Follow:   "https://writing.exchange/@write_as",
			},
		},
		Protocols: []nodeinfo.NodeProtocol{
			nodeinfo.ProtocolActivityPub,
		},
		Services: nodeinfo.Services{
			Inbound:  []nodeinfo.NodeService{},
			Outbound: []nodeinfo.NodeService{},
		},
		Software: nodeinfo.SoftwareInfo{
			Name:    serverSoftware,
			Version: softwareVer,
		},
	}
}

func (r nodeInfoResolver) IsOpenRegistration() (bool, error) {
	return r.cfg.App.OpenRegistration, nil
}

func (r nodeInfoResolver) Usage() (nodeinfo.Usage, error) {
	var collCount, postCount, activeHalfYear, activeMonth int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM collections`).Scan(&collCount)
	if err != nil {
		collCount = 0
	}
	err = r.db.QueryRow(`SELECT COUNT(*) FROM posts`).Scan(&postCount)
	if err != nil {
		log.Error("Unable to fetch post counts: %v", err)
	}

	if r.cfg.App.PublicStats {
		// Display bi-yearly / monthly stats
		err = r.db.QueryRow(`SELECT COUNT(*) FROM (
SELECT DISTINCT collection_id
FROM posts
INNER JOIN collections c
ON collection_id = c.id
WHERE collection_id IS NOT NULL
	AND updated > DATE_SUB(NOW(), INTERVAL 6 MONTH)) co`).Scan(&activeHalfYear)

		err = r.db.QueryRow(`SELECT COUNT(*) FROM (
SELECT DISTINCT collection_id
FROM posts
INNER JOIN FROM collections c
ON collection_id = c.id
WHERE collection_id IS NOT NULL
	AND updated > DATE_SUB(NOW(), INTERVAL 1 MONTH)) co`).Scan(&activeMonth)
	}

	return nodeinfo.Usage{
		Users: nodeinfo.UsageUsers{
			Total:          collCount,
			ActiveHalfYear: activeHalfYear,
			ActiveMonth:    activeMonth,
		},
		LocalPosts: postCount,
	}, nil
}
