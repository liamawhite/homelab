package storage

import (
	"context"
	"database/sql"
)

// withTx runs fn inside a transaction against db, committing on success and
// rolling back otherwise. Assets.Create/Update use this because each writes
// several tables (assets, asset_owners, asset_regions, and for Create,
// asset_valuations) that must land together or not at all.
func withTx(ctx context.Context, db *sql.DB, fn func(*sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}
