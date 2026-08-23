package storage

import (
	"path/filepath"
	"testing"
)

// TestApplyMigrationSQL_RebuildDoesNotCascadeDeleteChildren is a regression
// test for the incident where 0011_rename_pension_category_to_retirement.sql
// wiped every asset's owners/exposures/valuations in production. The
// CREATE-copy-DROP-RENAME rebuild technique this app uses for CHECK/column
// changes drops the original table after copying its rows into a *_new
// table - but SQLite fires ON DELETE CASCADE on that DROP for any table
// still referencing the original by foreign key, deleting the child rows
// even though the new parent table already has the data. This reproduces
// that exact shape (a populated parent with a cascading child) against a
// rebuild migration and asserts the child rows survive.
func TestApplyMigrationSQL_RebuildDoesNotCascadeDeleteChildren(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (version TEXT PRIMARY KEY)`); err != nil {
		t.Fatalf("creating schema_migrations: %v", err)
	}

	setup := `
		CREATE TABLE parent (id TEXT PRIMARY KEY, name TEXT NOT NULL);
		CREATE TABLE child (
			id TEXT PRIMARY KEY,
			parent_id TEXT NOT NULL REFERENCES parent(id) ON DELETE CASCADE
		);
		INSERT INTO parent (id, name) VALUES ('p1', 'original');
		INSERT INTO child (id, parent_id) VALUES ('c1', 'p1');
	`
	if err := applyMigrationSQL(db, "0001_setup.sql", setup); err != nil {
		t.Fatalf("applyMigrationSQL(setup) error = %v", err)
	}

	rebuild := `
		CREATE TABLE parent_new (id TEXT PRIMARY KEY, name TEXT NOT NULL);
		INSERT INTO parent_new (id, name) SELECT id, name FROM parent;
		DROP TABLE parent;
		ALTER TABLE parent_new RENAME TO parent;
	`
	if err := applyMigrationSQL(db, "0002_rebuild.sql", rebuild); err != nil {
		t.Fatalf("applyMigrationSQL(rebuild) error = %v", err)
	}

	var parentCount, childCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM parent`).Scan(&parentCount); err != nil {
		t.Fatalf("counting parent: %v", err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM child`).Scan(&childCount); err != nil {
		t.Fatalf("counting child: %v", err)
	}

	if parentCount != 1 {
		t.Fatalf("parent count = %d, want 1", parentCount)
	}
	if childCount != 1 {
		t.Fatalf("child count = %d, want 1 (rebuild must not cascade-delete children of the rebuilt table)", childCount)
	}
}

// TestMigration0003_PreservesPopulatedData is a second, migration-specific
// regression test on top of the general fix above: 0003 rebuilds both
// assets (widening the category CHECK) and asset_valuations (dropping the
// amount CHECK) against a database that - unlike a fresh install - already
// has real rows in every child table by the time 0003 runs, which is
// exactly the shape that caused the original incident. Applies 0001 and
// 0002 directly (not the full embedded set, so 0003 hasn't run yet),
// inserts one asset with an owner, an exposure, and two valuations, then
// applies 0003 and asserts everything survived intact.
func TestMigration0003_PreservesPopulatedData(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (version TEXT PRIMARY KEY)`); err != nil {
		t.Fatalf("creating schema_migrations: %v", err)
	}
	if err := applyMigration(db, "0001_init.sql"); err != nil {
		t.Fatalf("applying 0001_init.sql: %v", err)
	}
	if err := applyMigration(db, "0002_create_exchange_rates.sql"); err != nil {
		t.Fatalf("applying 0002_create_exchange_rates.sql: %v", err)
	}

	seed := `
		INSERT INTO assets (id, category, currency, etf_name, security_type, created_at)
		VALUES ('a1', 'etf', 'usd', 'FXAIX', 'stock', '2026-01-01T00:00:00Z');
		INSERT INTO asset_owners (id, asset_id, user_id, percentage, created_at)
		VALUES ('o1', 'a1', '1e0bbdf5-a356-47ea-803d-ea1af043059d', 100, '2026-01-01T00:00:00Z');
		INSERT INTO asset_exposures (id, asset_id, region, percentage, created_at)
		VALUES ('e1', 'a1', 'us', 100, '2026-01-01T00:00:00Z');
		INSERT INTO asset_valuations (id, asset_id, date, amount, created_at)
		VALUES
			('v1', 'a1', '2026-01-01', 1000, '2026-01-01T00:00:00Z'),
			('v2', 'a1', '2026-02-01', 1100, '2026-02-01T00:00:00Z');
	`
	if _, err := db.Exec(seed); err != nil {
		t.Fatalf("seeding pre-0003 data: %v", err)
	}

	if err := applyMigration(db, "0003_add_capital_balance_category.sql"); err != nil {
		t.Fatalf("applying 0003_add_capital_balance_category.sql: %v", err)
	}

	counts := map[string]int{}
	for table := range map[string]struct{}{"assets": {}, "asset_owners": {}, "asset_exposures": {}, "asset_valuations": {}} {
		var count int
		if err := db.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&count); err != nil {
			t.Fatalf("counting %s: %v", table, err)
		}
		counts[table] = count
	}

	if counts["assets"] != 1 {
		t.Errorf("assets count = %d, want 1", counts["assets"])
	}
	if counts["asset_owners"] != 1 {
		t.Errorf("asset_owners count = %d, want 1 (must survive the assets rebuild)", counts["asset_owners"])
	}
	if counts["asset_exposures"] != 1 {
		t.Errorf("asset_exposures count = %d, want 1 (must survive the assets rebuild)", counts["asset_exposures"])
	}
	if counts["asset_valuations"] != 2 {
		t.Errorf("asset_valuations count = %d, want 2 (must survive both the assets and asset_valuations rebuilds)", counts["asset_valuations"])
	}

	// The new capabilities 0003 adds must both work post-migration: a
	// 'capital_balance' category and a negative valuation amount.
	if _, err := db.Exec(`
		INSERT INTO assets (id, category, currency, account_name, created_at)
		VALUES ('a2', 'capital_balance', 'usd', 'Shared Capital', '2026-01-01T00:00:00Z');
	`); err != nil {
		t.Fatalf("inserting capital_balance asset after migration: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO asset_valuations (id, asset_id, date, amount, created_at)
		VALUES ('v3', 'a2', '2026-01-01', -13000, '2026-01-01T00:00:00Z');
	`); err != nil {
		t.Fatalf("inserting negative valuation after migration: %v", err)
	}
}
