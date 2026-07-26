-- The "Uncategorized" fallback label (seeded by 0002_create_labels.sql) is
-- being removed: every item must now carry a real, user-created label - no
-- automatic catch-all. Safe to delete outright (not just archive) since it
-- was a fixed seed row, not user data, and the items.label_id foreign key
-- (enforced by the `_pragma=foreign_keys(ON)` DSN option) would reject this
-- if any item still referenced it.
DELETE FROM labels WHERE id = '00000000-0000-0000-0000-000000000001';
