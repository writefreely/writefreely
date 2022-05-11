/*
 * Copyright © 2019-2021 A Bunch Tell LLC.
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
	dialect := wf_db.DialectMySQL
	if db.driverName == driverSQLite {
		dialect = wf_db.DialectSQLite
	}
	return wf_db.RunTransactionWithOptions(context.Background(), db.DB, &sql.TxOptions{}, func(ctx context.Context, tx *sql.Tx) error {
		createTableUsersOauth, err := dialect.
			Table("oauth_users").
			SetIfNotExists(false).
			Column(wf_db.NonNullableColumn("user_id", wf_db.ColumnTypeInt{MaxBytes: 4})).
			Column(wf_db.NonNullableColumn("remote_user_id", wf_db.ColumnTypeInt{MaxBytes: 4})).
			ToSQL()
		if err != nil {
			return err
		}
		createTableOauthClientState, err := dialect.
			Table("oauth_client_states").
			SetIfNotExists(false).
			Column(wf_db.NonNullableColumn("state", wf_db.ColumnTypeString{MaxChars: 255})).
			Column(wf_db.NonNullableColumn("used", wf_db.ColumnTypeBool{})).
			Column(wf_db.NonNullableColumn("created_at", wf_db.ColumnTypeDateTime{DefaultVal: wf_db.DefaultNow})).
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
