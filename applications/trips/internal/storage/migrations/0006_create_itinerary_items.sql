CREATE TABLE itinerary_items (
    id           TEXT PRIMARY KEY,
    trip_id      TEXT NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
    title        TEXT NOT NULL,
    location     TEXT NOT NULL DEFAULT '',
    latitude     REAL NOT NULL DEFAULT 0,
    longitude    REAL NOT NULL DEFAULT 0,
    place_id     TEXT NOT NULL DEFAULT '',
    state        TEXT NOT NULL DEFAULT 'UNSCHEDULED',
    date         TEXT NOT NULL DEFAULT '',
    start_minute INTEGER NOT NULL DEFAULT 0,
    end_minute   INTEGER NOT NULL DEFAULT 0,
    created_at   TEXT NOT NULL
);

CREATE INDEX idx_itinerary_items_trip_id ON itinerary_items(trip_id);
