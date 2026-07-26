-- category is a TEXT enum (SQLite has no native enum type) - add new
-- values here and to storage.ExerciseCategory's CHECK together.
CREATE TABLE exercises (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL UNIQUE,
    category   TEXT NOT NULL CHECK (category IN ('main_lift', 'accessory')),
    archived   INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL
);
