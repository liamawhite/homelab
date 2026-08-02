-- name: ListUsers :many
SELECT id, name, created_at FROM users ORDER BY created_at ASC;

-- name: CreateUser :one
INSERT INTO users (id, name, created_at) VALUES (?, ?, ?) RETURNING id, name, created_at;

-- name: DeleteUser :execrows
DELETE FROM users WHERE id = ?;

-- name: GetUser :one
SELECT id, name, created_at FROM users WHERE id = ?;
