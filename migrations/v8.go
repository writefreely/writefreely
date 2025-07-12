/*
 * Copyright © 2020-2021 Musing Studio LLC.
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

func oauthInvites(db *datastore) error {
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
		builders := []wf_db.SQLBuilder{
			dialect.
				AlterTable("oauth_client_states").
				AddColumn(dialect.Column("invite_code", wf_db.ColumnTypeChar, wf_db.OptionalInt{
					Set:   true,
					Value: 6,
				}).SetNullable(true)),
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
