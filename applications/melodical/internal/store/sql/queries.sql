-- name: PutQuestion :exec
INSERT INTO questions(id, payload, created_at) VALUES (?, ?, ?);
-- name: GetQuestion :one
SELECT payload FROM questions WHERE id = ?;
-- name: GetAttempt :one
SELECT feedback FROM attempts WHERE question_id = ?;
-- name: PutAttempt :exec
INSERT INTO attempts(question_id, skill, instrument, topic, key_name, correct, assisted, interrupted, elapsed_ms, level, feedback, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
-- name: GetReview :one
SELECT * FROM reviews WHERE skill = ?;
-- name: PutReview :exec
INSERT INTO reviews(skill, instrument, topic, key_name, attempts, correct, unaided, speed_total, speed_count, streak, due_at, last_seen)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(skill) DO UPDATE SET attempts=excluded.attempts, correct=excluded.correct,
unaided=excluded.unaided, speed_total=excluded.speed_total, speed_count=excluded.speed_count,
streak=excluded.streak, due_at=excluded.due_at, last_seen=excluded.last_seen;
-- name: ListReviews :many
SELECT * FROM reviews ORDER BY instrument, topic, key_name;
-- name: RecentAttempts :many
SELECT instrument, topic, key_name, correct, assisted, interrupted, elapsed_ms, created_at FROM attempts ORDER BY attempts.created_at DESC, attempts.rowid DESC LIMIT 12;
-- name: GetSettings :one
SELECT payload FROM settings WHERE id = 1;
-- name: PutSettings :exec
INSERT INTO settings(id, payload) VALUES (1, ?) ON CONFLICT(id) DO UPDATE SET payload=excluded.payload;
-- name: PruneQuestions :exec
DELETE FROM questions WHERE questions.created_at < ? AND questions.id NOT IN (SELECT question_id FROM attempts);
