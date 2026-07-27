-- cycle_exercises is a Cycle's shared, ordered exercise lineup - every
-- Block implicitly has each of these exercises available (there is
-- deliberately no block_exercises join table; exercise_sets rows are
-- simply absent until prescribed for a given (cycle_exercise, block)
-- pair). position is a single cycle-scoped ordering int; MoveCycleExercise
-- swaps a row's position with its nearest same-category neighbor
-- (category read from the joined exercises row) so category-grouped
-- display stays consistent cycle-wide. UNIQUE prevents adding the same
-- exercise twice.
CREATE TABLE cycle_exercises (
    id          TEXT PRIMARY KEY,
    cycle_id    TEXT NOT NULL REFERENCES cycles(id) ON DELETE CASCADE,
    exercise_id TEXT NOT NULL REFERENCES exercises(id),
    position    INTEGER NOT NULL,
    created_at  TEXT NOT NULL,
    UNIQUE (cycle_id, exercise_id)
);

-- GetCycle lists every cycle_exercise for a cycle ordered by position;
-- MoveCycleExercise's neighbor lookup filters the same way.
CREATE INDEX idx_cycle_exercises_cycle_position ON cycle_exercises (cycle_id, position);
