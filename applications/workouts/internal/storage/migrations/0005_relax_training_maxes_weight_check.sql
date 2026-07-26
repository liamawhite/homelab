-- SQLite has no ALTER TABLE for changing a CHECK constraint, so relaxing
-- weight's bound from `> 0` to `>= 0` requires rebuilding the table:
-- create the new shape, copy rows across, drop the old table, rename the
-- new one into place (same technique shopping's history uses for CHECK
-- constraint changes). 0 is a legitimate training max for a bodyweight
-- exercise - it means bodyweight only, no added load (e.g. an unweighted
-- pull-up), not "no value recorded".
CREATE TABLE training_maxes_new (
    id           TEXT PRIMARY KEY,
    user_id      TEXT NOT NULL REFERENCES users(id),
    exercise_id  TEXT NOT NULL REFERENCES exercises(id),
    weight       REAL NOT NULL CHECK (weight >= 0),
    unit         TEXT NOT NULL CHECK (unit IN ('kg', 'lb')),
    effective_at TEXT NOT NULL,
    created_at   TEXT NOT NULL
);

INSERT INTO training_maxes_new (id, user_id, exercise_id, weight, unit, effective_at, created_at)
SELECT id, user_id, exercise_id, weight, unit, effective_at, created_at FROM training_maxes;

DROP TABLE training_maxes;

ALTER TABLE training_maxes_new RENAME TO training_maxes;

CREATE INDEX idx_training_maxes_user_exercise_effective
    ON training_maxes (user_id, exercise_id, effective_at);
