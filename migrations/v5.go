package migrations

import (
	"context"
	"database/sql"

	wf_db "github.com/writeas/writefreely/db"
)

func oauthSlack(db *datastore) error {
	dialect := wf_db.DialectMySQL
	if db.driverName == driverSQLite {
		dialect = wf_db.DialectSQLite
	}
	return wf_db.RunTransactionWithOptions(context.Background(), db.DB, &sql.TxOptions{}, func(ctx context.Context, tx *sql.Tx) error {
		builders := []wf_db.SQLBuilder{
			dialect.
				AlterTable("oauth_client_state").
				AddColumn(dialect.
					Column(
						"provider",
						wf_db.ColumnTypeVarChar,
						wf_db.OptionalInt{Set: true, Value: 24,})).
				AddColumn(dialect.
					Column(
						"client_id",
						wf_db.ColumnTypeVarChar,
						wf_db.OptionalInt{Set: true, Value: 128,})),
			dialect.
				AlterTable("users_oauth").
				ChangeColumn("remote_user_id",
					dialect.
						Column(
							"remote_user_id",
							wf_db.ColumnTypeVarChar,
							wf_db.OptionalInt{Set: true, Value: 128,})).
				AddColumn(dialect.
					Column(
						"provider",
						wf_db.ColumnTypeVarChar,
						wf_db.OptionalInt{Set: true, Value: 24,})).
				AddColumn(dialect.
					Column(
						"client_id",
						wf_db.ColumnTypeVarChar,
						wf_db.OptionalInt{Set: true, Value: 128,})).
				AddColumn(dialect.
					Column(
						"access_token",
						wf_db.ColumnTypeVarChar,
						wf_db.OptionalInt{Set: true, Value: 512,})),
			dialect.DropIndex("remote_user_id", "users_oauth"),
			dialect.DropIndex("user_id", "users_oauth"),
			dialect.CreateUniqueIndex("users_oauth", "users_oauth", "user_id", "provider", "client_id"),
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
