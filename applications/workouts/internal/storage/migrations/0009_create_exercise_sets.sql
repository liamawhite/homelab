-- exercise_sets is the actual prescription: for a given (cycle_exercise,
-- block) pair, an ordered list of sets - the "cell" in a grid of
-- rows=cycle_exercises (grouped by category), columns=blocks. reps is
-- always required; percentage_of_tm is nullable (no NOT NULL, and its
-- CHECK explicitly allows NULL) so bodyweight/accessory work can be
-- reps-only.
CREATE TABLE exercise_sets (
    id                TEXT PRIMARY KEY,
    cycle_exercise_id TEXT NOT NULL REFERENCES cycle_exercises(id) ON DELETE CASCADE,
    block_id          TEXT NOT NULL REFERENCES blocks(id) ON DELETE CASCADE,
    position          INTEGER NOT NULL,
    reps              INTEGER NOT NULL CHECK (reps > 0),
    percentage_of_tm  REAL CHECK (percentage_of_tm IS NULL OR percentage_of_tm > 0),
    created_at        TEXT NOT NULL
);

-- GetCycle lists every set for a cycle_exercise+block cell ordered by
-- position; MoveSet's neighbor lookup filters the same way.
CREATE INDEX idx_exercise_sets_cell_position ON exercise_sets (cycle_exercise_id, block_id, position);
