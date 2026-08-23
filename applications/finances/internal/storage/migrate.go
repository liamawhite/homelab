package storage

import (
	"database/sql"
	"embed"
	"fmt"
	"sort"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Migrate applies every embedded migration not yet recorded in
// schema_migrations, in filename order, each in its own transaction. This is
// deliberately the smallest thing that works for one table - adding
// 0003_create_transactions.sql later is just dropping in a new file, no
// migration framework needed.
func Migrate(db *sql.DB) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (version TEXT PRIMARY KEY)`); err != nil {
		return fmt.Errorf("creating schema_migrations table: %w", err)
	}

	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("reading migrations directory: %w", err)
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	sort.Strings(names)

	for _, name := range names {
		if err := applyMigration(db, name); err != nil {
			return err
		}
	}

	return nil
}

func applyMigration(db *sql.DB, name string) error {
	var applied bool
	if err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = ?)`, name).Scan(&applied); err != nil {
		return fmt.Errorf("checking migration %s: %w", name, err)
	}
	if applied {
		return nil
	}

	statement, err := migrationsFS.ReadFile("migrations/" + name)
	if err != nil {
		return fmt.Errorf("reading migration %s: %w", name, err)
	}

	return applyMigrationSQL(db, name, string(statement))
}

// applyMigrationSQL is applyMigration's FK-safety-and-transaction core,
// split out so it can be exercised directly in tests against synthetic SQL
// without needing a real embedded migration file.
func applyMigrationSQL(db *sql.DB, name, statement string) error {
	// foreign_keys must be toggled outside any transaction - SQLite makes
	// it a no-op otherwise (Open's connection pool is capped at 1, so this
	// is guaranteed to affect the same connection tx.Begin() below grabs).
	// Without this, a CREATE-copy-DROP-RENAME rebuild migration (the
	// technique this app uses for CHECK-constraint/column changes, e.g.
	// 0011) cascade-deletes every row in any table with an ON DELETE
	// CASCADE FK to the table being rebuilt the moment DROP TABLE runs -
	// even though the *_new table already has the data copied across. This
	// is SQLite's own documented procedure for schema changes on tables
	// with incoming foreign keys (https://www.sqlite.org/lang_altertable.html
	// section 7); 0011 hit it for real against live data before this fix.
	if _, err := db.Exec(`PRAGMA foreign_keys = OFF`); err != nil {
		return fmt.Errorf("disabling foreign keys for migration %s: %w", name, err)
	}
	defer db.Exec(`PRAGMA foreign_keys = ON`)

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("beginning transaction for migration %s: %w", name, err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(statement); err != nil {
		return fmt.Errorf("applying migration %s: %w", name, err)
	}
	if _, err := tx.Exec(`INSERT INTO schema_migrations (version) VALUES (?)`, name); err != nil {
		return fmt.Errorf("recording migration %s: %w", name, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing migration %s: %w", name, err)
	}

	return checkForeignKeys(db, name)
}

// checkForeignKeys runs PRAGMA foreign_key_check after a migration commits -
// it reports any row that no longer satisfies its foreign key regardless of
// whether enforcement is currently on, so it catches a rebuild migration
// that silently orphaned or (as with the cascade bug above) wiped rows,
// before that damage ships any further than this one migration.
func checkForeignKeys(db *sql.DB, name string) error {
	rows, err := db.Query(`PRAGMA foreign_key_check`)
	if err != nil {
		return fmt.Errorf("running foreign_key_check after migration %s: %w", name, err)
	}
	defer rows.Close()

	if rows.Next() {
		return fmt.Errorf("migration %s left foreign key violations behind", name)
	}
	return rows.Err()
}
