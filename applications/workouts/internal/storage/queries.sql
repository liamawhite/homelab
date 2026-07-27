-- name: ListUsers :many
SELECT id, name, created_at FROM users ORDER BY created_at ASC;

-- name: CreateUser :one
INSERT INTO users (id, name, created_at) VALUES (?, ?, ?) RETURNING id, name, created_at;

-- name: DeleteUser :execrows
DELETE FROM users WHERE id = ?;

-- name: GetUser :one
SELECT id, name, created_at FROM users WHERE id = ?;

-- name: ListExercises :many
SELECT id, name, category, equipment, archived, created_at FROM exercises ORDER BY created_at ASC;

-- name: GetExercise :one
SELECT id, name, category, equipment, archived, created_at FROM exercises WHERE id = ?;

-- name: CreateExercise :one
INSERT INTO exercises (id, name, category, equipment, archived, created_at) VALUES (?, ?, ?, ?, 0, ?)
RETURNING id, name, category, equipment, archived, created_at;

-- name: ArchiveExercise :execrows
UPDATE exercises SET archived = 1 WHERE id = ?;

-- name: RestoreExercise :execrows
UPDATE exercises SET archived = 0 WHERE id = ?;

-- name: ListCurrentTrainingMaxes :many
-- One row per exercise: whichever training_maxes row for (user_id,
-- exercise_id) has the latest effective_at, ties broken by rowid (i.e.
-- most-recently-inserted wins) - see 0003_create_training_maxes.sql.
SELECT tm.id, tm.user_id, tm.exercise_id, tm.weight, tm.unit, tm.effective_at, tm.created_at, ex.name AS exercise_name
FROM training_maxes tm
JOIN exercises ex ON ex.id = tm.exercise_id
WHERE tm.user_id = ?
  AND tm.id = (
    SELECT latest.id FROM training_maxes AS latest
    WHERE latest.user_id = tm.user_id AND latest.exercise_id = tm.exercise_id
    ORDER BY latest.effective_at DESC, latest.rowid DESC
    LIMIT 1
  )
ORDER BY ex.name ASC;

-- name: ListTrainingMaxHistory :many
SELECT tm.id, tm.user_id, tm.exercise_id, tm.weight, tm.unit, tm.effective_at, tm.created_at, ex.name AS exercise_name
FROM training_maxes tm
JOIN exercises ex ON ex.id = tm.exercise_id
WHERE tm.user_id = ? AND tm.exercise_id = ?
ORDER BY tm.effective_at DESC;

-- name: CreateTrainingMax :one
INSERT INTO training_maxes (id, user_id, exercise_id, weight, unit, effective_at, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
RETURNING id, user_id, exercise_id, weight, unit, effective_at, created_at;

-- Cycles

-- name: ListCyclesByUser :many
SELECT id, user_id, name, created_at FROM cycles WHERE user_id = ? ORDER BY created_at ASC;

-- name: GetCycle :one
SELECT id, user_id, name, created_at FROM cycles WHERE id = ?;

-- name: CreateCycle :one
INSERT INTO cycles (id, user_id, name, created_at) VALUES (?, ?, ?, ?)
RETURNING id, user_id, name, created_at;

-- name: DeleteCycle :execrows
DELETE FROM cycles WHERE id = ?;

-- Blocks

-- name: ListBlocksByCycle :many
SELECT id, cycle_id, name, position, created_at FROM blocks WHERE cycle_id = ? ORDER BY position ASC;

-- name: GetBlock :one
SELECT id, cycle_id, name, position, created_at FROM blocks WHERE id = ?;

-- name: MaxBlockPosition :one
SELECT COALESCE(MAX(position), -1) FROM blocks WHERE cycle_id = ?;

-- name: CreateBlock :one
INSERT INTO blocks (id, cycle_id, name, position, created_at) VALUES (?, ?, ?, ?, ?)
RETURNING id, cycle_id, name, position, created_at;

-- name: DeleteBlock :execrows
DELETE FROM blocks WHERE id = ?;

-- Cycle exercises

-- name: ListCycleExercisesByCycle :many
-- Joined with exercises to denormalize name/category/equipment (see
-- v1.CycleExercise), ordered by the cycle-wide position.
SELECT ce.id, ce.cycle_id, ce.exercise_id, ce.position, ce.created_at,
       ex.name AS exercise_name, ex.category AS exercise_category, ex.equipment AS exercise_equipment
FROM cycle_exercises ce
JOIN exercises ex ON ex.id = ce.exercise_id
WHERE ce.cycle_id = ?
ORDER BY ce.position ASC;

-- name: GetCycleExercise :one
SELECT ce.id, ce.cycle_id, ce.exercise_id, ce.position, ce.created_at,
       ex.name AS exercise_name, ex.category AS exercise_category, ex.equipment AS exercise_equipment
FROM cycle_exercises ce
JOIN exercises ex ON ex.id = ce.exercise_id
WHERE ce.id = ?;

-- name: MaxCycleExercisePosition :one
SELECT COALESCE(MAX(position), -1) FROM cycle_exercises WHERE cycle_id = ?;

-- name: CreateCycleExercise :one
INSERT INTO cycle_exercises (id, cycle_id, exercise_id, position, created_at) VALUES (?, ?, ?, ?, ?)
RETURNING id, cycle_id, exercise_id, position, created_at;

-- name: DeleteCycleExercise :execrows
DELETE FROM cycle_exercises WHERE id = ?;

-- name: FindCycleExerciseAbove :one
-- Nearest same-category neighbor with a smaller position - MoveCycleExercise's "up" swap.
SELECT ce.id, ce.position
FROM cycle_exercises ce
JOIN exercises ex ON ex.id = ce.exercise_id
WHERE ce.cycle_id = ? AND ex.category = ? AND ce.position < ?
ORDER BY ce.position DESC
LIMIT 1;

-- name: FindCycleExerciseBelow :one
SELECT ce.id, ce.position
FROM cycle_exercises ce
JOIN exercises ex ON ex.id = ce.exercise_id
WHERE ce.cycle_id = ? AND ex.category = ? AND ce.position > ?
ORDER BY ce.position ASC
LIMIT 1;

-- name: SetCycleExercisePosition :exec
UPDATE cycle_exercises SET position = ? WHERE id = ?;

-- Exercise sets

-- name: ListExerciseSetsByCycle :many
SELECT es.id, es.cycle_exercise_id, es.block_id, es.position, es.reps, es.percentage_of_tm, es.created_at
FROM exercise_sets es
JOIN cycle_exercises ce ON ce.id = es.cycle_exercise_id
WHERE ce.cycle_id = ?
ORDER BY es.cycle_exercise_id ASC, es.block_id ASC, es.position ASC;

-- name: GetExerciseSet :one
SELECT id, cycle_exercise_id, block_id, position, reps, percentage_of_tm, created_at FROM exercise_sets WHERE id = ?;

-- name: MaxExerciseSetPosition :one
SELECT COALESCE(MAX(position), -1) FROM exercise_sets WHERE cycle_exercise_id = ? AND block_id = ?;

-- name: CreateExerciseSet :one
INSERT INTO exercise_sets (id, cycle_exercise_id, block_id, position, reps, percentage_of_tm, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
RETURNING id, cycle_exercise_id, block_id, position, reps, percentage_of_tm, created_at;

-- name: UpdateExerciseSet :one
UPDATE exercise_sets SET reps = ?, percentage_of_tm = ? WHERE id = ?
RETURNING id, cycle_exercise_id, block_id, position, reps, percentage_of_tm, created_at;

-- name: DeleteExerciseSet :execrows
DELETE FROM exercise_sets WHERE id = ?;

-- name: FindExerciseSetAbove :one
SELECT id, position FROM exercise_sets
WHERE cycle_exercise_id = ? AND block_id = ? AND position < ?
ORDER BY position DESC
LIMIT 1;

-- name: FindExerciseSetBelow :one
SELECT id, position FROM exercise_sets
WHERE cycle_exercise_id = ? AND block_id = ? AND position > ?
ORDER BY position ASC
LIMIT 1;

-- name: SetExerciseSetPosition :exec
UPDATE exercise_sets SET position = ? WHERE id = ?;
