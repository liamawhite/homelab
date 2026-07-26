-- name: ListItems :many
SELECT id, name, created_at FROM items ORDER BY created_at ASC;

-- name: CreateItem :one
INSERT INTO items (id, name, created_at) VALUES (?, ?, ?) RETURNING id, name, created_at;

-- name: DeleteItem :execrows
DELETE FROM items WHERE id = ?;
