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
	"github.com/writeas/web-core/log"
	"github.com/writeas/writefreely/config"
	"github.com/writefreely/go-nodeinfo"
	"strings"
)

type nodeInfoResolver struct {
	cfg *config.Config
	db  *datastore
}

func nodeInfoConfig(db *datastore, cfg *config.Config) *nodeinfo.Config {
	name := cfg.App.SiteName
	desc := cfg.App.SiteDesc
	if desc == "" {
		desc = "Minimal, federated blogging platform."
	}
	if cfg.App.SingleUser {
		// Fetch blog information, instead
		coll, err := db.GetCollectionByID(1)
		if err == nil {
			desc = coll.Description
		}
	}
	return &nodeinfo.Config{
		BaseURL: cfg.App.Host,
		InfoURL: "/api/nodeinfo",

		Metadata: nodeinfo.Metadata{
			NodeName:        name,
			NodeDescription: desc,
			Private:         cfg.App.Private,
			Software: nodeinfo.SoftwareMeta{
				HomePage: softwareURL,
				GitHub:   "https://github.com/writeas/writefreely",
				Follow:   "https://writing.exchange/@write_as",
			},
			MaxBlogs:     cfg.App.MaxBlogs,
			PublicReader: cfg.App.LocalTimeline,
			Invites:      cfg.App.UserInvites != "",
		},
		Protocols: []nodeinfo.NodeProtocol{
			nodeinfo.ProtocolActivityPub,
		},
		Services: nodeinfo.Services{
			Inbound: []nodeinfo.NodeService{},
			Outbound: []nodeinfo.NodeService{
				nodeinfo.ServiceRSS,
			},
		},
		Software: nodeinfo.SoftwareInfo{
			Name:    strings.ToLower(serverSoftware),
			Version: softwareVer,
		},
	}
}

func (r nodeInfoResolver) IsOpenRegistration() (bool, error) {
	return r.cfg.App.OpenRegistration, nil
}

func (r nodeInfoResolver) Usage() (nodeinfo.Usage, error) {
	var collCount, postCount int64
	var activeHalfYear, activeMonth int
	var err error
	collCount, err = r.db.GetTotalCollections()
	if err != nil {
		collCount = 0
	}
	postCount, err = r.db.GetTotalPosts()
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
			Total:          int(collCount),
			ActiveHalfYear: activeHalfYear,
			ActiveMonth:    activeMonth,
		},
		LocalPosts: int(postCount),
	}, nil
}
