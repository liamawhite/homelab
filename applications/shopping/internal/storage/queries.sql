-- name: ListItems :many
-- No ORDER BY - display order is a client concern, not the server's (see
-- web/src/lib/items.ts's useItems for the actual sort).
SELECT items.id, items.name, items.created_at, items.label_id, items.status, items.completed_at, labels.name AS label_name
FROM items
JOIN labels ON labels.id = items.label_id;

-- name: CreateItem :one
-- New items always start todo/uncompleted - status and completed_at are
-- never caller-supplied.
INSERT INTO items (id, name, label_id, status, completed_at, created_at)
VALUES (?, ?, ?, 'todo', NULL, ?)
RETURNING id, name, label_id, status, completed_at, created_at;

-- name: SetItemStatus :one
UPDATE items SET status = ?, completed_at = ? WHERE id = ?
RETURNING id, name, label_id, status, completed_at, created_at;

-- name: DeleteItem :execrows
DELETE FROM items WHERE id = ?;

-- name: ListLabels :many
-- No ORDER BY - display order is a client concern, not the server's (see
-- web/src/lib/labels.ts's useLabels for the actual sort).
SELECT id, name, archived, created_at, color FROM labels;

-- name: GetLabel :one
SELECT id, name, archived, created_at, color FROM labels WHERE id = ?;

-- name: CountLabels :one
SELECT COUNT(*) FROM labels;

-- name: CreateLabel :one
INSERT INTO labels (id, name, archived, created_at, color) VALUES (?, ?, 0, ?, ?) RETURNING id, name, archived, created_at, color;

-- name: ArchiveLabel :execrows
UPDATE labels SET archived = 1 WHERE id = ?;

-- name: RestoreLabel :execrows
UPDATE labels SET archived = 0 WHERE id = ?;

-- name: SetLabelColor :one
UPDATE labels SET color = ? WHERE id = ? RETURNING id, name, archived, created_at, color;
