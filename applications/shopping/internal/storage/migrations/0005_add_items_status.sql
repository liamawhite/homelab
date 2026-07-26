-- status is a TEXT enum (SQLite has no native enum type) constrained to a
-- fixed set of values, matching storage.ItemStatus - add new values there
-- and to this CHECK constraint together.
--
-- completed_at must be set if and only if status = 'done' - enforced by a
-- table-level CHECK below, not just application code, so a future bug can
-- never write a done item with no timestamp or a todo item with one. SQLite
-- can't add a multi-column CHECK via ALTER TABLE ADD COLUMN, so this
-- rebuilds the table (safe: production has zero real items at this point,
-- copying over id/name/label_id/created_at from any that exist and
-- defaulting the new columns to todo/NULL, which trivially satisfies the
-- constraint).
CREATE TABLE items_new (
    id           TEXT PRIMARY KEY,
    name         TEXT NOT NULL,
    label_id     TEXT NOT NULL REFERENCES labels(id),
    status       TEXT NOT NULL DEFAULT 'todo' CHECK (status IN ('todo', 'done')),
    completed_at TEXT,
    created_at   TEXT NOT NULL,
    CHECK (
        (status = 'done' AND completed_at IS NOT NULL) OR
        (status = 'todo' AND completed_at IS NULL)
    )
);

INSERT INTO items_new (id, name, label_id, status, completed_at, created_at)
SELECT id, name, label_id, 'todo', NULL, created_at FROM items;

DROP TABLE items;
ALTER TABLE items_new RENAME TO items;
