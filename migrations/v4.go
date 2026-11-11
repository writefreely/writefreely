/*
 * Copyright © 2019-2021 Musing Studio LLC.
 *
 * This file is part of WriteFreely.
 *
 * WriteFreely is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License, included
 * in the LICENSE file in this source code package.
 */

package migrations

import (
	"context"
	"database/sql"

	wf_db "github.com/writefreely/writefreely/db"
)

func oauth(db *datastore) error {
	var dialect wf_db.DialectType

	switch db.driverName {
	case driverSQLite:
		dialect = wf_db.DialectSQLite
	case driverMySQL:
		dialect = wf_db.DialectMySQL
	case driverPostgres:
		dialect = wf_db.DialectPostgres
	}

	return wf_db.RunTransactionWithOptions(context.Background(), db.DB, &sql.TxOptions{}, func(ctx context.Context, tx *sql.Tx) error {
		createTableUsersOauth, err := dialect.
			Table("oauth_users").
			SetIfNotExists(false).
			Column(dialect.Column("user_id", wf_db.ColumnTypeInteger, wf_db.UnsetSize)).
			Column(dialect.Column("remote_user_id", wf_db.ColumnTypeInteger, wf_db.UnsetSize)).
			ToSQL()
		if err != nil {
			return err
		}
		createTableOauthClientState, err := dialect.
			Table("oauth_client_states").
			SetIfNotExists(false).
			Column(dialect.Column("state", wf_db.ColumnTypeVarChar, wf_db.OptionalInt{Set: true, Value: 255})).
			Column(dialect.Column("used", wf_db.ColumnTypeBool, wf_db.UnsetSize)).
			Column(dialect.Column("created_at", wf_db.ColumnTypeDateTime, wf_db.UnsetSize).SetDefaultCurrentTimestamp()).
			UniqueConstraint("state").
			ToSQL()
		if err != nil {
			return err
		}

		for _, table := range []string{createTableUsersOauth, createTableOauthClientState} {
			if _, err := tx.ExecContext(ctx, table); err != nil {
				return err
			}
		}
		return nil
	})
}
