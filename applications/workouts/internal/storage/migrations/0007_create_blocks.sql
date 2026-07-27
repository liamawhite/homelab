-- Block is one time period within a Cycle (the user's replacement term for
-- "week" - a block isn't necessarily 7 days). Append-only, ordered by
-- position; there's no reordering RPC for blocks.
CREATE TABLE blocks (
    id         TEXT PRIMARY KEY,
    cycle_id   TEXT NOT NULL REFERENCES cycles(id) ON DELETE CASCADE,
    name       TEXT NOT NULL,
    position   INTEGER NOT NULL,
    created_at TEXT NOT NULL
);

-- GetCycle lists every block for a cycle ordered by position.
CREATE INDEX idx_blocks_cycle_position ON blocks (cycle_id, position);
