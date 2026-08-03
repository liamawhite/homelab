-- name: ListTrips :many
SELECT id, name, created_at FROM trips ORDER BY created_at ASC;

-- name: CreateTrip :one
INSERT INTO trips (id, name, created_at) VALUES (?, ?, ?) RETURNING id, name, created_at;

-- name: DeleteTrip :execrows
DELETE FROM trips WHERE id = ?;

-- Column order in every SELECT/RETURNING list below matches the flights
-- table's physical column order (i.e. sqlcgen.Flight's field order), not
-- SQL-readability order - sqlc generates a distinct row struct per query,
-- and storage/flights.go converts those to sqlcgen.Flight via a plain Go
-- struct conversion, which requires matching field order.

-- name: ListFlightsByTrip :many
SELECT id, trip_id, flight_number, flight_date, departure_airport, arrival_airport,
       scheduled_departure, scheduled_arrival, actual_departure, actual_arrival,
       status, last_synced_at, sync_error, created_at, departure_timezone, arrival_timezone,
       confirmation_link, booking_number
FROM flights
WHERE trip_id = ?
ORDER BY flight_date ASC, created_at ASC;

-- name: GetFlight :one
SELECT id, trip_id, flight_number, flight_date, departure_airport, arrival_airport,
       scheduled_departure, scheduled_arrival, actual_departure, actual_arrival,
       status, last_synced_at, sync_error, created_at, departure_timezone, arrival_timezone,
       confirmation_link, booking_number
FROM flights
WHERE id = ?;

-- name: CreateFlight :one
INSERT INTO flights (id, trip_id, flight_number, flight_date, created_at, confirmation_link, booking_number)
VALUES (?, ?, ?, ?, ?, ?, ?)
RETURNING id, trip_id, flight_number, flight_date, departure_airport, arrival_airport,
          scheduled_departure, scheduled_arrival, actual_departure, actual_arrival,
          status, last_synced_at, sync_error, created_at, departure_timezone, arrival_timezone,
          confirmation_link, booking_number;

-- name: DeleteFlight :execrows
DELETE FROM flights WHERE id = ?;

-- name: UpdateFlightSyncResult :one
UPDATE flights
SET departure_airport   = ?,
    arrival_airport     = ?,
    departure_timezone  = ?,
    arrival_timezone    = ?,
    scheduled_departure = ?,
    scheduled_arrival   = ?,
    actual_departure    = ?,
    actual_arrival      = ?,
    status              = ?,
    last_synced_at      = ?,
    sync_error          = ?
WHERE id = ?
RETURNING id, trip_id, flight_number, flight_date, departure_airport, arrival_airport,
          scheduled_departure, scheduled_arrival, actual_departure, actual_arrival,
          status, last_synced_at, sync_error, created_at, departure_timezone, arrival_timezone,
          confirmation_link, booking_number;

-- name: UpdateFlightSyncError :one
UPDATE flights
SET last_synced_at = ?,
    sync_error      = ?
WHERE id = ?
RETURNING id, trip_id, flight_number, flight_date, departure_airport, arrival_airport,
          scheduled_departure, scheduled_arrival, actual_departure, actual_arrival,
          status, last_synced_at, sync_error, created_at, departure_timezone, arrival_timezone,
          confirmation_link, booking_number;

-- name: UpdateFlightConfirmation :one
UPDATE flights
SET confirmation_link = ?,
    booking_number     = ?
WHERE id = ?
RETURNING id, trip_id, flight_number, flight_date, departure_airport, arrival_airport,
          scheduled_departure, scheduled_arrival, actual_departure, actual_arrival,
          status, last_synced_at, sync_error, created_at, departure_timezone, arrival_timezone,
          confirmation_link, booking_number;

-- name: ListAccommodationsByTrip :many
SELECT id, trip_id, name, location, check_in, check_out, created_at, confirmation_link, booking_number
FROM accommodations
WHERE trip_id = ?
ORDER BY check_in ASC, created_at ASC;

-- name: CreateAccommodation :one
INSERT INTO accommodations (id, trip_id, name, location, check_in, check_out, created_at, confirmation_link, booking_number)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id, trip_id, name, location, check_in, check_out, created_at, confirmation_link, booking_number;

-- name: DeleteAccommodation :execrows
DELETE FROM accommodations WHERE id = ?;

-- name: UpdateAccommodationConfirmation :one
UPDATE accommodations
SET confirmation_link = ?,
    booking_number     = ?
WHERE id = ?
RETURNING id, trip_id, name, location, check_in, check_out, created_at, confirmation_link, booking_number;

-- Column order in every SELECT/RETURNING list below matches the
-- itinerary_items table's physical column order (i.e. sqlcgen.ItineraryItem's
-- field order) - see the note above the flights queries for why.

-- name: ListItineraryItemsByTrip :many
SELECT id, trip_id, title, location, latitude, longitude, place_id, state, date, start_minute, end_minute, created_at, timezone, end_date
FROM itinerary_items
WHERE trip_id = ?
ORDER BY created_at ASC;

-- name: CreateItineraryItem :one
INSERT INTO itinerary_items (id, trip_id, title, location, latitude, longitude, place_id, state, date, start_minute, end_minute, created_at, timezone, end_date)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id, trip_id, title, location, latitude, longitude, place_id, state, date, start_minute, end_minute, created_at, timezone, end_date;

-- name: ScheduleItineraryItem :one
UPDATE itinerary_items
SET state        = ?,
    date         = ?,
    start_minute = ?,
    end_minute   = ?,
    timezone     = ?,
    end_date     = ?
WHERE id = ?
RETURNING id, trip_id, title, location, latitude, longitude, place_id, state, date, start_minute, end_minute, created_at, timezone, end_date;

-- name: DeleteItineraryItem :execrows
DELETE FROM itinerary_items WHERE id = ?;
