-- Finances only ever tracks this household's two people. Seed them so the
-- app is usable immediately; UserService still supports adding/removing
-- more later if that changes.
INSERT INTO users (id, name, created_at) VALUES
    ('1e0bbdf5-a356-47ea-803d-ea1af043059d', 'Liam White', '2026-07-27T00:00:00Z'),
    ('a402564b-0a65-4bae-9e80-90a38b47f712', 'Tia Louden', '2026-07-27T00:00:00Z');
