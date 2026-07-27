CREATE TABLE accommodations (
    id         TEXT PRIMARY KEY,
    trip_id    TEXT NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
    name       TEXT NOT NULL,
    location   TEXT NOT NULL DEFAULT '',
    check_in   TEXT NOT NULL,
    check_out  TEXT NOT NULL,
    created_at TEXT NOT NULL
);

CREATE INDEX idx_accommodations_trip_id ON accommodations(trip_id);
