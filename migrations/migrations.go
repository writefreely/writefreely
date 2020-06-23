/*
 * Copyright © 2019 A Bunch Tell LLC.
 *
 * This file is part of WriteFreely.
 *
 * WriteFreely is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License, included
 * in the LICENSE file in this source code package.
 */

// Package migrations contains database migrations for WriteFreely
package migrations

import (
	"database/sql"

	"github.com/writeas/web-core/log"
)

// TODO: refactor to use the datastore struct from writefreely pkg
type datastore struct {
	*sql.DB
	driverName string
}

func NewDatastore(db *sql.DB, dn string) *datastore {
	return &datastore{db, dn}
}

// TODO: use these consts from writefreely pkg
const (
	driverMySQL  = "mysql"
	driverSQLite = "sqlite3"
)

type Migration interface {
	Description() string
	Migrate(db *datastore) error
}

type migration struct {
	description string
	migrate     func(db *datastore) error
}

func New(d string, fn func(db *datastore) error) Migration {
	return &migration{d, fn}
}

func (m *migration) Description() string {
	return m.description
}

func (m *migration) Migrate(db *datastore) error {
	return m.migrate(db)
}

var migrations = []Migration{
	New("support user invites", supportUserInvites),                 // -> V1 (v0.8.0)
	New("support dynamic instance pages", supportInstancePages),     // V1 -> V2 (v0.9.0)
	New("support users suspension", supportUserStatus),              // V2 -> V3 (v0.11.0)
	New("support oauth", oauth),                                     // V3 -> V4
	New("support slack oauth", oauthSlack),                          // V4 -> v5
	New("support ActivityPub mentions", supportActivityPubMentions), // V5 -> V6
	New("support oauth attach", oauthAttach),                        // V6 -> V7
	New("support oauth via invite", oauthInvites),                   // V7 -> V8 (v0.12.0)
	New("optimize drafts retrieval", optimizeDrafts),                // V8 -> V9
	New("support post signatures", supportPostSignatures),           // V9 -> V10
}

// CurrentVer returns the current migration version the application is on
func CurrentVer() int {
	return len(migrations)
}

func SetInitialMigrations(db *datastore) error {
	// Included schema files represent changes up to V1, so note that in the database
	_, err := db.Exec("INSERT INTO appmigrations (version, migrated, result) VALUES (?, "+db.now()+", ?)", 1, "")
	if err != nil {
		return err
	}
	return nil
}

func Migrate(db *datastore) error {
	var version int
	var err error
	if db.tableExists("appmigrations") {
		err = db.QueryRow("SELECT MAX(version) FROM appmigrations").Scan(&version)
	} else {
		log.Info("Initializing appmigrations table...")
		version = 0
		_, err = db.Exec(`CREATE TABLE appmigrations (
			version ` + db.typeInt() + ` NOT NULL,
			migrated ` + db.typeDateTime() + ` NOT NULL,
			result ` + db.typeText() + ` NOT NULL
		) ` + db.engine() + `;`)
		if err != nil {
			return err
		}
	}

	if len(migrations[version:]) > 0 {
		for i, m := range migrations[version:] {
			curVer := version + i + 1
			log.Info("Migrating to V%d: %s", curVer, m.Description())
			err = m.Migrate(db)
			if err != nil {
				return err
			}

			// Update migrations table
			_, err = db.Exec("INSERT INTO appmigrations (version, migrated, result) VALUES (?, "+db.now()+", ?)", curVer, "")
			if err != nil {
				return err
			}
		}
	} else {
		log.Info("Database up-to-date. No migrations to run.")
	}

	return nil
}

func (db *datastore) tableExists(t string) bool {
	var dummy string
	var err error
	if db.driverName == driverSQLite {
		err = db.QueryRow("SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?", t).Scan(&dummy)
	} else {
		err = db.QueryRow("SHOW TABLES LIKE '" + t + "'").Scan(&dummy)
	}
	switch {
	case err == sql.ErrNoRows:
		return false
	case err != nil:
		log.Error("Couldn't SHOW TABLES: %v", err)
		return false
	}

	return true
}
