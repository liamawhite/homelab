CREATE TABLE labels (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL UNIQUE,
    archived   INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL
);

-- Well-known fallback label every item resolves to when no label is given
-- on create (see storage.UncategorizedLabelID) - seeded with a fixed ID so
-- Go code can reference it without a lookup, and never archived/deleted.
INSERT INTO labels (id, name, archived, created_at)
VALUES ('00000000-0000-0000-0000-000000000001', 'Uncategorized', 0, '1970-01-01T00:00:00Z');
