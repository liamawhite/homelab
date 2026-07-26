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
