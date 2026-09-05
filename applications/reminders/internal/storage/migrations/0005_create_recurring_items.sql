-- recurring_items stores one row per recurring reminder; rule_type selects
-- which of the rule-specific columns below are populated (NULL otherwise) -
-- see internal/recurrence for the Go-side Rule type and next-due-date math,
-- and internal/storage/recurring_items.go for the row<->Rule conversion.
CREATE TABLE recurring_items (
    id            TEXT PRIMARY KEY,
    title         TEXT NOT NULL,
    notes         TEXT NOT NULL DEFAULT '',
    created_at    TEXT NOT NULL,
    next_due_date TEXT NOT NULL,
    area          TEXT NOT NULL,
    rule_type     TEXT NOT NULL,      -- 'weekly' | 'monthly' | 'yearly' | 'interval'
    days_of_week  TEXT,               -- weekly only: sorted, comma-separated time.Weekday ints, e.g. "1,3,5"
    days_of_month TEXT,               -- monthly only: sorted, comma-separated days, e.g. "1,15"
    yearly_dates  TEXT,               -- yearly only: sorted, comma-separated "M-D" pairs, e.g. "1-1,7-4"
    interval_days INTEGER             -- interval only: >= 1
);
