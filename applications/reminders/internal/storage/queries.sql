-- name: ListOneOffItems :many
SELECT id, title, created_at FROM one_off_items ORDER BY created_at ASC;

-- name: CreateOneOffItem :one
INSERT INTO one_off_items (id, title, created_at) VALUES (?, ?, ?) RETURNING id, title, created_at;

-- name: DeleteOneOffItem :execrows
DELETE FROM one_off_items WHERE id = ?;
