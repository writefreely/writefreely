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

func oauthSlack(db *datastore) error {
	dialect := wf_db.DialectMySQL
	if db.driverName == driverSQLite {
		dialect = wf_db.DialectSQLite
	}
	return wf_db.RunTransactionWithOptions(context.Background(), db.DB, &sql.TxOptions{}, func(ctx context.Context, tx *sql.Tx) error {
		builders := []wf_db.SQLBuilder{
			dialect.
				AlterTable("oauth_client_states").
				AddColumn(wf_db.
					NonNullableColumn(
						"provider",
						wf_db.ColumnTypeString{
							MaxChars:   24,
							HasDefault: true,
							DefaultVal: "",
						})),
			dialect.
				AlterTable("oauth_client_states").
				AddColumn(wf_db.
					NonNullableColumn(
						"client_id",
						wf_db.ColumnTypeString{
							MaxChars:   128,
							HasDefault: true,
							DefaultVal: "",
						},
					)),
			dialect.
				AlterTable("oauth_users").
				AddColumn(wf_db.
					NonNullableColumn(
						"provider",
						wf_db.ColumnTypeString{
							MaxChars:   24,
							HasDefault: true,
							DefaultVal: "",
						})),
			dialect.
				AlterTable("oauth_users").
				AddColumn(wf_db.
					NonNullableColumn(
						"client_id",
						wf_db.ColumnTypeString{
							MaxChars:   128,
							HasDefault: true,
							DefaultVal: "",
						})),
			dialect.
				AlterTable("oauth_users").
				AddColumn(wf_db.
					NonNullableColumn(
						"access_token",
						wf_db.ColumnTypeString{
							MaxChars:   512,
							HasDefault: true,
							DefaultVal: "",
						})),
			dialect.CreateUniqueIndex("oauth_users_uk", "oauth_users", "user_id", "provider", "client_id"),
		}

		if dialect != wf_db.DialectSQLite {
			// This updates the length of the `remote_user_id` column. It isn't needed for SQLite databases.
			builders = append(builders, dialect.
				AlterTable("oauth_users").
				ChangeColumn("remote_user_id",
					wf_db.
						NonNullableColumn(
							"remote_user_id",
							wf_db.ColumnTypeString{
								MaxChars: 128,
							})))
		}

		for _, builder := range builders {
			query, err := builder.ToSQL()
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, query); err != nil {
				return err
			}
		}
		return nil
	})
}
