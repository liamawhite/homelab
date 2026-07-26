-- unit is a TEXT enum (SQLite has no native enum type) - add new values
-- here and to storage.WeightUnit's CHECK together.
--
-- Every update to a user's training max for an exercise INSERTs a new row
-- rather than overwriting one, so history is never lost. There's no
-- separate "is_current" flag (which could drift out of sync on a failed
-- write) - "current" is derived purely from whichever row for a given
-- (user_id, exercise_id) has the latest effective_at, see queries.sql's
-- ListCurrentTrainingMaxes.
CREATE TABLE training_maxes (
    id           TEXT PRIMARY KEY,
    user_id      TEXT NOT NULL REFERENCES users(id),
    exercise_id  TEXT NOT NULL REFERENCES exercises(id),
    weight       REAL NOT NULL CHECK (weight > 0),
    unit         TEXT NOT NULL CHECK (unit IN ('kg', 'lb')),
    effective_at TEXT NOT NULL,
    created_at   TEXT NOT NULL
);

-- Every read pattern (current-TM lookup, history-for-one-exercise) filters
-- on user_id (+ optionally exercise_id) and orders by effective_at, so this
-- index covers both without a second one.
CREATE INDEX idx_training_maxes_user_exercise_effective
    ON training_maxes (user_id, exercise_id, effective_at);
