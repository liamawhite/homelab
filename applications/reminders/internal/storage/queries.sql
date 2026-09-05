-- name: ListOneOffItems :many
SELECT id, title, created_at, notes, due_date, area, status FROM one_off_items ORDER BY created_at ASC;

-- name: CreateOneOffItem :one
INSERT INTO one_off_items (id, title, notes, created_at, due_date, area, status) VALUES (?, ?, ?, ?, ?, ?, ?) RETURNING id, title, created_at, notes, due_date, area, status;

-- name: UpdateOneOffItemTitle :one
UPDATE one_off_items SET title = ? WHERE id = ? RETURNING id, title, created_at, notes, due_date, area, status;

-- name: UpdateOneOffItemNotes :one
UPDATE one_off_items SET notes = ? WHERE id = ? RETURNING id, title, created_at, notes, due_date, area, status;

-- name: UpdateOneOffItemDueDate :one
UPDATE one_off_items SET due_date = ? WHERE id = ? RETURNING id, title, created_at, notes, due_date, area, status;

-- name: UpdateOneOffItemArea :one
UPDATE one_off_items SET area = ? WHERE id = ? RETURNING id, title, created_at, notes, due_date, area, status;

-- name: UpdateOneOffItemStatus :one
UPDATE one_off_items SET status = ? WHERE id = ? RETURNING id, title, created_at, notes, due_date, area, status;

-- name: DeleteOneOffItem :execrows
DELETE FROM one_off_items WHERE id = ?;

-- name: ListRecurringItems :many
SELECT id, title, notes, created_at, next_due_date, area, rule_type, days_of_week, days_of_month, yearly_dates, interval_days FROM recurring_items ORDER BY next_due_date ASC;

-- name: GetRecurringItem :one
SELECT id, title, notes, created_at, next_due_date, area, rule_type, days_of_week, days_of_month, yearly_dates, interval_days FROM recurring_items WHERE id = ?;

-- name: CreateRecurringItem :one
INSERT INTO recurring_items (id, title, notes, created_at, next_due_date, area, rule_type, days_of_week, days_of_month, yearly_dates, interval_days)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id, title, notes, created_at, next_due_date, area, rule_type, days_of_week, days_of_month, yearly_dates, interval_days;

-- name: UpdateRecurringItemTitle :one
UPDATE recurring_items SET title = ? WHERE id = ?
RETURNING id, title, notes, created_at, next_due_date, area, rule_type, days_of_week, days_of_month, yearly_dates, interval_days;

-- name: UpdateRecurringItemNotes :one
UPDATE recurring_items SET notes = ? WHERE id = ?
RETURNING id, title, notes, created_at, next_due_date, area, rule_type, days_of_week, days_of_month, yearly_dates, interval_days;

-- name: UpdateRecurringItemArea :one
UPDATE recurring_items SET area = ? WHERE id = ?
RETURNING id, title, notes, created_at, next_due_date, area, rule_type, days_of_week, days_of_month, yearly_dates, interval_days;

-- name: UpdateRecurringItemRule :one
UPDATE recurring_items
SET rule_type = ?, days_of_week = ?, days_of_month = ?, yearly_dates = ?, interval_days = ?, next_due_date = ?
WHERE id = ?
RETURNING id, title, notes, created_at, next_due_date, area, rule_type, days_of_week, days_of_month, yearly_dates, interval_days;

-- name: UpdateRecurringItemNextDueDate :one
UPDATE recurring_items SET next_due_date = ? WHERE id = ?
RETURNING id, title, notes, created_at, next_due_date, area, rule_type, days_of_week, days_of_month, yearly_dates, interval_days;

-- name: DeleteRecurringItem :execrows
DELETE FROM recurring_items WHERE id = ?;
