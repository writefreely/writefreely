package migrations

import (
	"context"
	"database/sql"

	wf_db "github.com/writeas/writefreely/db"
)

func oauthAttach(db *datastore) error {
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
						"attach_user_id",
						wf_db.ColumnTypeInteger,
						wf_db.OptionalInt{Set: true, Value: 24}).SetNullable(true)),
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
