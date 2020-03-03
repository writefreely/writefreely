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
				AlterTable("oauth_client_states").
				AddColumn(dialect.
					Column(
						"provider",
						wf_db.ColumnTypeVarChar,
						wf_db.OptionalInt{Set: true, Value: 24})),
			dialect.
				AlterTable("oauth_client_states").
				AddColumn(dialect.
					Column(
						"client_id",
						wf_db.ColumnTypeVarChar,
						wf_db.OptionalInt{Set: true, Value: 128})),
			dialect.
				AlterTable("oauth_users").
				AddColumn(dialect.
					Column(
						"provider",
						wf_db.ColumnTypeVarChar,
						wf_db.OptionalInt{Set: true, Value: 24})),
			dialect.
				AlterTable("oauth_users").
				AddColumn(dialect.
					Column(
						"client_id",
						wf_db.ColumnTypeVarChar,
						wf_db.OptionalInt{Set: true, Value: 128})),
			dialect.
				AlterTable("oauth_users").
				AddColumn(dialect.
					Column(
						"access_token",
						wf_db.ColumnTypeVarChar,
						wf_db.OptionalInt{Set: true, Value: 512,})),
			dialect.CreateUniqueIndex("oauth_users", "oauth_users", "user_id", "provider", "client_id"),
		}

		if dialect != wf_db.DialectSQLite {
			// This updates the length of the `remote_user_id` column. It isn't needed for SQLite databases.
			builders = append(builders, dialect.
				AlterTable("oauth_users").
				ChangeColumn("remote_user_id",
					dialect.
						Column(
							"remote_user_id",
							wf_db.ColumnTypeVarChar,
							wf_db.OptionalInt{Set: true, Value: 128})))
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
