-- equipment is a TEXT enum (SQLite has no native enum type) - add new
-- values here and to storage.ExerciseEquipment's CHECK together. Existing
-- rows default to 'barbell' since SQLite's ALTER TABLE ADD COLUMN requires
-- a default for a NOT NULL column with pre-existing rows (same pattern as
-- shopping's 0006_add_labels_color.sql).
ALTER TABLE exercises ADD COLUMN equipment TEXT NOT NULL DEFAULT 'barbell' CHECK (equipment IN (
    'barbell', 'dumbbell', 'bodyweight'
));
