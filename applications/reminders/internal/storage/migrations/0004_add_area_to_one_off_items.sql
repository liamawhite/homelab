-- area is a TEXT enum (SQLite has no native enum type) - add new values
-- here and to storage.Area's constants together. The DEFAULT only backfills
-- rows that predate this column; CreateOneOffItem always requires an
-- explicit area going forward (see oneoffitemservice.fromProtoArea).
ALTER TABLE one_off_items ADD COLUMN area TEXT NOT NULL DEFAULT 'personal';
