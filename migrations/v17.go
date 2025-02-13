/*
 * Copyright © 2019-2024 Musing Studio LLC.
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

func increasePostContentSize(db *datastore) error {
	if db.driverName != driverMySQL {
		// Only MySQL databases need this migration
		return nil
	}

	dialect := wf_db.DialectMySQL
	return wf_db.RunTransactionWithOptions(context.Background(), db.DB, &sql.TxOptions{}, func(ctx context.Context, tx *sql.Tx) error {
		builders := []wf_db.SQLBuilder{
			dialect.AlterTable("posts").
				ChangeColumn("content",
					dialect.Column(
						"column",
						wf_db.ColumnTypeLongText,
						wf_db.OptionalInt{
							Set: false,
							Value: 0,
						}).SetNullable(false)),
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

