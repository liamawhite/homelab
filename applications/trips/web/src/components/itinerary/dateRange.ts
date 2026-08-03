import { timestampDate } from "@bufbuild/protobuf/wkt";

import type { Flight } from "@/gen/trips/v1/flight_pb";
import type { Accommodation } from "@/gen/trips/v1/accommodation_pb";
import {
  ItineraryItemState,
  type ItineraryItem,
} from "@/gen/trips/v1/itinerary_pb";
import { localDateKey } from "@/lib/time";

// dateStr is YYYY-MM-DD - parsed via explicit local-Date components, same
// convention as lib/time.ts's formatDate, to avoid `new Date(dateStr)`'s
// UTC-midnight parsing.
function parseLocalDate(dateStr: string): Date {
  const [year, month, day] = dateStr.split("-").map(Number);
  return new Date(year, month - 1, day);
}

function toDateStr(date: Date): string {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}

function addDays(date: Date, days: number): Date {
  const result = new Date(date);
  result.setDate(result.getDate() + days);
  return result;
}

// pushRange pushes every calendar day from start to end (inclusive) onto
// dates - used for anything that can span multiple days (accommodation
// stays, multi-day itinerary items).
function pushRange(dates: Date[], startStr: string, endStr: string) {
  let cursor = parseLocalDate(startStr);
  const end = parseLocalDate(endStr);
  while (cursor <= end) {
    dates.push(cursor);
    cursor = addDays(cursor, 1);
  }
}

// computeDateRange derives the itinerary calendar's day columns: every
// flight's local departure/arrival date, every day between each
// accommodation's check-in/check-out (inclusive), and every non-unscheduled
// item's date through end_date (inclusive), expanded by one day on either
// side. Falls back to just today if nothing is collected, so the calendar
// always has at least one column to render.
export function computeDateRange(
  flights: Flight[],
  accommodations: Accommodation[],
  items: ItineraryItem[],
): string[] {
  const dates: Date[] = [];

  for (const flight of flights) {
    const departure = flight.actualDeparture ?? flight.scheduledDeparture;
    const arrival = flight.actualArrival ?? flight.scheduledArrival;
    if (departure) {
      dates.push(
        parseLocalDate(
          localDateKey(timestampDate(departure), flight.departureTimezone),
        ),
      );
    }
    if (arrival) {
      dates.push(
        parseLocalDate(
          localDateKey(timestampDate(arrival), flight.arrivalTimezone),
        ),
      );
    }
    if (!departure && !arrival && flight.date) {
      dates.push(parseLocalDate(flight.date));
    }
  }

  for (const accommodation of accommodations) {
    if (!accommodation.checkIn || !accommodation.checkOut) continue;
    pushRange(dates, accommodation.checkIn, accommodation.checkOut);
  }

  for (const item of items) {
    if (item.state !== ItineraryItemState.UNSCHEDULED && item.date) {
      pushRange(dates, item.date, item.endDate || item.date);
    }
  }

  if (dates.length === 0) {
    dates.push(new Date());
  }

  const min = addDays(new Date(Math.min(...dates.map((d) => d.getTime()))), -1);
  const max = addDays(new Date(Math.max(...dates.map((d) => d.getTime()))), 1);

  const range: string[] = [];
  for (let cursor = min; cursor <= max; cursor = addDays(cursor, 1)) {
    range.push(toDateStr(cursor));
  }

  return range;
}
