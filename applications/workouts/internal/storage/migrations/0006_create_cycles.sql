-- Cycle is the top-level, single-user training program - see 0007-0009 for
-- the rest of its tree (blocks, cycle_exercises, exercise_sets). Unlike
-- training_maxes, every child table in this domain cascades on delete so
-- deleting a Cycle tears down its whole tree in one shot - foreign_keys is
-- already ON via storage.Open's DSN.
CREATE TABLE cycles (
    id         TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users(id),
    name       TEXT NOT NULL,
    created_at TEXT NOT NULL
);

-- ListCycles filters on user_id and orders by created_at.
CREATE INDEX idx_cycles_user_created ON cycles (user_id, created_at);
