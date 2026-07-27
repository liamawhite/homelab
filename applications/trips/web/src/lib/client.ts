import { createClient } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";

import { TripService } from "../gen/trips/v1/trip_pb";
import { FlightService } from "../gen/trips/v1/flight_pb";
import { AccommodationService } from "../gen/trips/v1/accommodation_pb";

// Same-origin: the Go binary serves both this app and the Connect API on
// one port (see internal/server), so no CORS setup is needed. The Vite dev
// proxy (vite.config.ts) makes this true in development too.
const transport = createConnectTransport({ baseUrl: "/" });

export const tripClient = createClient(TripService, transport);
export const flightClient = createClient(FlightService, transport);
export const accommodationClient = createClient(AccommodationService, transport);
