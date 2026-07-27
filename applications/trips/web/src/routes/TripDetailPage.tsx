import { useState } from "react";
import { timestampDate, type Timestamp } from "@bufbuild/protobuf/wkt";
import { Link, useNavigate, useParams } from "@tanstack/react-router";
import { ArrowLeft, RefreshCw, Trash2 } from "lucide-react";

import { useTrips, useDeleteTrip } from "@/lib/trips";
import { useFlights, useAddFlight, useDeleteFlight, useSyncFlight } from "@/lib/flights";
import {
  useAccommodations,
  useAddAccommodation,
  useDeleteAccommodation,
} from "@/lib/accommodations";
import { FlightStatus, type Flight } from "@/gen/trips/v1/flight_pb";
import { relativeTime } from "@/lib/time";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";

const STATUS_LABEL: Record<FlightStatus, string> = {
  [FlightStatus.UNSPECIFIED]: "Unknown",
  [FlightStatus.SCHEDULED]: "Scheduled",
  [FlightStatus.ACTIVE]: "En route",
  [FlightStatus.LANDED]: "Landed",
  [FlightStatus.CANCELLED]: "Cancelled",
  [FlightStatus.DIVERTED]: "Diverted",
  [FlightStatus.UNKNOWN]: "Unknown",
};

// Renders ts in timeZone (an IANA zone name, e.g. "America/Los_Angeles")
// with a short zone abbreviation appended, so a departure and arrival shown
// side by side are never misread as the same clock when they're actually in
// different zones. Falls back to the browser's local zone (no abbreviation
// shown) if timeZone is empty - happens before a flight's first successful
// sync, since AeroDataBox is the only source of airport zone data here.
function formatTime(ts?: Timestamp, timeZone?: string) {
  if (!ts) return "—";
  return timestampDate(ts).toLocaleString(undefined, {
    month: "short",
    day: "numeric",
    hour: "numeric",
    minute: "2-digit",
    timeZone: timeZone || undefined,
    timeZoneName: timeZone ? "short" : undefined,
  });
}

// dateStr is a plain YYYY-MM-DD calendar date (no time/zone) - parsed via
// explicit local-Date components rather than `new Date(dateStr)` (which
// parses as UTC midnight and can display as the previous day in negative
// UTC-offset zones).
function formatDate(dateStr: string) {
  if (!dateStr) return "—";
  const [year, month, day] = dateStr.split("-").map(Number);
  return new Date(year, month - 1, day).toLocaleDateString(undefined, {
    month: "short",
    day: "numeric",
    year: "numeric",
  });
}

export function TripDetailPage() {
  const { tripId } = useParams({ from: "/trips/$tripId" });
  const { data: trips, isLoading } = useTrips();
  const deleteTrip = useDeleteTrip();
  const navigate = useNavigate();

  const { data: flights, isLoading: flightsLoading, isError: flightsError, error: flightsErrorObj } = useFlights(tripId);
  const addFlight = useAddFlight(tripId);
  const deleteFlight = useDeleteFlight(tripId);
  const syncFlight = useSyncFlight(tripId);
  const [flightNumber, setFlightNumber] = useState("");
  const [flightDate, setFlightDate] = useState("");

  const {
    data: accommodations,
    isLoading: accommodationsLoading,
    isError: accommodationsError,
    error: accommodationsErrorObj,
  } = useAccommodations(tripId);
  const addAccommodation = useAddAccommodation(tripId);
  const deleteAccommodation = useDeleteAccommodation(tripId);
  const [accommodationName, setAccommodationName] = useState("");
  const [accommodationLocation, setAccommodationLocation] = useState("");
  const [checkIn, setCheckIn] = useState("");
  const [checkOut, setCheckOut] = useState("");

  // Trips has no GetTrip RPC - the list is already cached by useTrips, so
  // finding the trip client-side avoids a second endpoint for a field set
  // this small (id/name/created_at).
  const trip = trips?.find((t) => t.id === tripId);

  const handleDelete = () => {
    deleteTrip.mutate(tripId, { onSuccess: () => navigate({ to: "/" }) });
  };

  const handleAddFlight = () => {
    const trimmed = flightNumber.trim();
    if (!trimmed || !flightDate) return;
    addFlight.mutate(
      { flightNumber: trimmed, date: flightDate },
      { onSuccess: () => setFlightNumber("") },
    );
  };

  const syncedSubtext = (flight: Flight) => {
    if (flight.syncError) return flight.syncError;
    if (!flight.lastSyncedAt) return "never synced";
    return `synced ${relativeTime(timestampDate(flight.lastSyncedAt))}`;
  };

  const handleAddAccommodation = () => {
    const trimmedName = accommodationName.trim();
    if (!trimmedName || !checkIn || !checkOut) return;
    addAccommodation.mutate(
      { name: trimmedName, location: accommodationLocation.trim(), checkIn, checkOut },
      {
        onSuccess: () => {
          setAccommodationName("");
          setAccommodationLocation("");
        },
      },
    );
  };

  return (
    <div className="mx-auto flex w-full max-w-4xl flex-1 flex-col gap-4 p-4">
      <Link
        to="/"
        className="flex w-fit items-center gap-1 text-sm text-muted-foreground hover:text-foreground"
      >
        <ArrowLeft className="size-4" />
        Back to trips
      </Link>

      {isLoading && <p className="text-sm text-muted-foreground">Loading…</p>}
      {!isLoading && !trip && <p className="text-sm text-muted-foreground">Trip not found.</p>}

      {trip && (
        <div className="flex flex-col gap-6">
          <div className="flex flex-col gap-4">
            <div className="flex items-start justify-between">
              <h1 className="text-xl font-semibold">{trip.name}</h1>
              <Button
                variant="ghost"
                size="icon-sm"
                aria-label={`Delete ${trip.name}`}
                disabled={deleteTrip.isPending}
                onClick={handleDelete}
              >
                <Trash2 />
              </Button>
            </div>
            <p className="text-sm text-muted-foreground">
              Created {trip.createdAt ? relativeTime(timestampDate(trip.createdAt)) : "unknown"}
            </p>
            {deleteTrip.isError && (
              <p className="text-sm text-destructive">{deleteTrip.error.message}</p>
            )}
          </div>

          <div className="flex flex-col gap-4">
            <h2 className="text-sm font-semibold">Flights</h2>

            <div className="flex gap-2">
              <Input
                placeholder="Flight number (e.g. UA523)"
                value={flightNumber}
                onChange={(e) => setFlightNumber(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter") handleAddFlight();
                }}
              />
              <Input
                type="date"
                value={flightDate}
                onChange={(e) => setFlightDate(e.target.value)}
                className="w-40"
              />
              <Button
                onClick={handleAddFlight}
                disabled={addFlight.isPending || !flightNumber.trim() || !flightDate}
              >
                Add flight
              </Button>
            </div>

            {flightsLoading && <p className="text-sm text-muted-foreground">Loading flights…</p>}
            {flightsError && (
              <p className="text-sm text-destructive">{flightsErrorObj.message}</p>
            )}

            {flights && flights.length > 0 && (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Flight</TableHead>
                    <TableHead>Route</TableHead>
                    <TableHead>Departs</TableHead>
                    <TableHead>Arrives</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead className="text-right">Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {flights.map((flight) => (
                    <TableRow key={flight.id}>
                      <TableCell className="font-medium">{flight.flightNumber}</TableCell>
                      <TableCell className="text-muted-foreground">
                        {flight.departureAirport && flight.arrivalAirport
                          ? `${flight.departureAirport} → ${flight.arrivalAirport}`
                          : "—"}
                      </TableCell>
                      <TableCell className="text-muted-foreground">
                        {formatTime(
                          flight.actualDeparture ?? flight.scheduledDeparture,
                          flight.departureTimezone,
                        )}
                      </TableCell>
                      <TableCell className="text-muted-foreground">
                        {formatTime(
                          flight.actualArrival ?? flight.scheduledArrival,
                          flight.arrivalTimezone,
                        )}
                      </TableCell>
                      <TableCell>
                        <div>{STATUS_LABEL[flight.status]}</div>
                        <div
                          className={
                            flight.syncError
                              ? "text-xs text-destructive"
                              : "text-xs text-muted-foreground"
                          }
                        >
                          {syncedSubtext(flight)}
                        </div>
                      </TableCell>
                      <TableCell className="text-right">
                        <Button
                          variant="ghost"
                          size="icon-sm"
                          aria-label={`Sync ${flight.flightNumber}`}
                          disabled={syncFlight.isPending}
                          onClick={() => syncFlight.mutate(flight.id)}
                        >
                          <RefreshCw />
                        </Button>
                        <Button
                          variant="ghost"
                          size="icon-sm"
                          aria-label={`Delete ${flight.flightNumber}`}
                          disabled={deleteFlight.isPending}
                          onClick={() => deleteFlight.mutate(flight.id)}
                        >
                          <Trash2 />
                        </Button>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}
            {!flightsLoading && flights && flights.length === 0 && (
              <p className="text-sm text-muted-foreground">No flights yet.</p>
            )}
            {addFlight.isError && (
              <p className="text-sm text-destructive">{addFlight.error.message}</p>
            )}
            {deleteFlight.isError && (
              <p className="text-sm text-destructive">{deleteFlight.error.message}</p>
            )}
            {syncFlight.isError && (
              <p className="text-sm text-destructive">{syncFlight.error.message}</p>
            )}
          </div>

          <div className="flex flex-col gap-4">
            <h2 className="text-sm font-semibold">Accommodation</h2>

            <div className="flex flex-wrap gap-2">
              <Input
                placeholder="Name (e.g. Premier Inn Farnham)"
                value={accommodationName}
                onChange={(e) => setAccommodationName(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter") handleAddAccommodation();
                }}
                className="min-w-48 flex-1"
              />
              <Input
                placeholder="Location"
                value={accommodationLocation}
                onChange={(e) => setAccommodationLocation(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter") handleAddAccommodation();
                }}
                className="min-w-40 flex-1"
              />
              <Input
                type="date"
                value={checkIn}
                onChange={(e) => setCheckIn(e.target.value)}
                className="w-40"
              />
              <Input
                type="date"
                value={checkOut}
                onChange={(e) => setCheckOut(e.target.value)}
                className="w-40"
              />
              <Button
                onClick={handleAddAccommodation}
                disabled={
                  addAccommodation.isPending ||
                  !accommodationName.trim() ||
                  !checkIn ||
                  !checkOut
                }
              >
                Add stay
              </Button>
            </div>

            {accommodationsLoading && (
              <p className="text-sm text-muted-foreground">Loading accommodation…</p>
            )}
            {accommodationsError && (
              <p className="text-sm text-destructive">{accommodationsErrorObj.message}</p>
            )}

            {accommodations && accommodations.length > 0 && (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Name</TableHead>
                    <TableHead>Location</TableHead>
                    <TableHead>Check-in</TableHead>
                    <TableHead>Check-out</TableHead>
                    <TableHead className="text-right">Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {accommodations.map((accommodation) => (
                    <TableRow key={accommodation.id}>
                      <TableCell className="font-medium">{accommodation.name}</TableCell>
                      <TableCell className="text-muted-foreground">
                        {accommodation.location || "—"}
                      </TableCell>
                      <TableCell className="text-muted-foreground">
                        {formatDate(accommodation.checkIn)}
                      </TableCell>
                      <TableCell className="text-muted-foreground">
                        {formatDate(accommodation.checkOut)}
                      </TableCell>
                      <TableCell className="text-right">
                        <Button
                          variant="ghost"
                          size="icon-sm"
                          aria-label={`Delete ${accommodation.name}`}
                          disabled={deleteAccommodation.isPending}
                          onClick={() => deleteAccommodation.mutate(accommodation.id)}
                        >
                          <Trash2 />
                        </Button>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}
            {!accommodationsLoading && accommodations && accommodations.length === 0 && (
              <p className="text-sm text-muted-foreground">No accommodation yet.</p>
            )}
            {addAccommodation.isError && (
              <p className="text-sm text-destructive">{addAccommodation.error.message}</p>
            )}
            {deleteAccommodation.isError && (
              <p className="text-sm text-destructive">{deleteAccommodation.error.message}</p>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
