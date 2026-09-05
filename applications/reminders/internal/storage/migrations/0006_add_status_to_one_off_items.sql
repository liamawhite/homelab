-- status is a TEXT enum (SQLite has no native enum type) - add new values
-- here and to storage.OneOffItemStatus's constants together. The DEFAULT
-- only backfills rows that predate this column; every existing row becomes
-- open, matching current behavior where nothing was ever "done".
ALTER TABLE one_off_items ADD COLUMN status TEXT NOT NULL DEFAULT 'open';
