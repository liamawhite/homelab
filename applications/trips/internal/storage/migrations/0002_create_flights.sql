CREATE TABLE flights (
    id                  TEXT PRIMARY KEY,
    trip_id             TEXT NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
    flight_number       TEXT NOT NULL,
    flight_date         TEXT NOT NULL,
    departure_airport   TEXT NOT NULL DEFAULT '',
    arrival_airport     TEXT NOT NULL DEFAULT '',
    scheduled_departure TEXT,
    scheduled_arrival   TEXT,
    actual_departure    TEXT,
    actual_arrival      TEXT,
    status              TEXT NOT NULL DEFAULT 'UNKNOWN',
    last_synced_at      TEXT,
    sync_error          TEXT NOT NULL DEFAULT '',
    created_at          TEXT NOT NULL
);

CREATE INDEX idx_flights_trip_id ON flights(trip_id);
