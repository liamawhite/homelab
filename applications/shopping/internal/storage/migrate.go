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
// 0002_....sql later is just dropping in a new file, no migration framework
// needed.
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

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("beginning transaction for migration %s: %w", name, err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(string(statement)); err != nil {
		return fmt.Errorf("applying migration %s: %w", name, err)
	}
	if _, err := tx.Exec(`INSERT INTO schema_migrations (version) VALUES (?)`, name); err != nil {
		return fmt.Errorf("recording migration %s: %w", name, err)
	}

	return tx.Commit()
}
