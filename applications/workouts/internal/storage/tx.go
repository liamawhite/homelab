package storage

import (
	"context"
	"database/sql"
	"fmt"
)

// withTx runs fn inside a transaction against db, committing on success and
// rolling back otherwise. Every "append" (position = current max + 1) and
// "move" (swap two positions) in this package uses this - storage.Open's
// single-connection pool only serializes individual statements, not a
// read-then-write pair issued as two separate calls, so without a
// transaction two concurrent requests could interleave and corrupt
// ordering (e.g. both read MAX=3, both insert position=4).
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

// asPosition converts a MAX(position)-style COALESCE query's untyped
// result (sqlc can't infer a concrete type across a COALESCE, so these
// queries return `interface{}`) into an int64.
func asPosition(v interface{}) (int64, error) {
	i, ok := v.(int64)
	if !ok {
		return 0, fmt.Errorf("expected int64 position, got %T", v)
	}
	return i, nil
}
