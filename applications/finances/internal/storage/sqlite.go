// Package storage owns SQLite access: opening the database, applying
// migrations, and the repositories that query it.
package storage

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// Open opens the SQLite database at path. WAL mode and foreign keys are
// enabled via the DSN; the connection pool is capped at one connection so
// SQLite's single-writer constraint is enforced at the pool level instead of
// tuning busy-timeout - fine for a low-traffic, single-replica app.
// temp_store=MEMORY keeps any transient b-trees (e.g. a table-rebuild
// migration's INSERT...SELECT) in RAM instead of SQLite's default of
// spilling to a temp file - the container image is FROM scratch, so there
// is no /tmp for it to spill to (SQLITE_IOERR_GETTEMPPATH otherwise).
func Open(path string) (*sql.DB, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)&_pragma=temp_store(2)", path)

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening sqlite database: %w", err)
	}
	db.SetMaxOpenConns(1)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("pinging sqlite database: %w", err)
	}

	return db, nil
}
