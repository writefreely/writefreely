package db

import (
	"context"
	"database/sql"
)

// TransactionScopedWork describes code executed within a database transaction.
type TransactionScopedWork func(ctx context.Context, db *sql.Tx) error

// RunTransactionWithOptions executes a block of code within a database transaction.
func RunTransactionWithOptions(ctx context.Context, db *sql.DB, txOpts *sql.TxOptions, txWork TransactionScopedWork) error {
	tx, err := db.BeginTx(ctx, txOpts)
	if err != nil {
		return err
	}

	if err = txWork(ctx, tx); err != nil {
		if txErr := tx.Rollback(); txErr != nil {
			return txErr
		}
		return err
	}
	return tx.Commit()
}
