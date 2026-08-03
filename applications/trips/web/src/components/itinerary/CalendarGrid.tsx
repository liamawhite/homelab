import { useEffect, useRef, type ReactNode } from "react";
import { timestampDate } from "@bufbuild/protobuf/wkt";
import { PlaneTakeoff, PlaneLanding } from "lucide-react";

import {
  ItineraryItemState,
  type ItineraryItem,
} from "@/gen/trips/v1/itinerary_pb";
import type { Flight } from "@/gen/trips/v1/flight_pb";
import type { Accommodation } from "@/gen/trips/v1/accommodation_pb";
import { localDateKey, localMinuteOfDay } from "@/lib/time";
import { MapsLink } from "@/components/MapsLink";
import { AllDayBlock, TimedBlock } from "./ScheduledBlock";
import { HOUR_HEIGHT_PX, segmentForDay, yFromMinutes } from "./grid";

const HOURS = Array.from({ length: 24 }, (_, i) => i);
const DEFAULT_SCROLL_HOUR = 7;
const GUTTER_WIDTH = "w-12";

// day is YYYY-MM-DD - parsed via explicit local-Date components, same
// convention as lib/time.ts's formatDate.
function formatDayHeader(day: string) {
  const [year, month, dayOfMonth] = day.split("-").map(Number);
  return new Date(year, month - 1, dayOfMonth).toLocaleDateString(undefined, {
    weekday: "short",
    month: "short",
    day: "numeric",
  });
}

interface SpanningEntry {
  id: string;
  startDay: string;
  endDay: string;
  content: ReactNode;
}

interface SpanningRowProps {
  days: string[];
  entries: SpanningEntry[];
  minHeightClass: string;
}

// Lays entries out as CSS-grid column spans (one real bar from startDay to
// endDay) rather than repeating the same content in every day column it
// touches - a multi-day stay or all-day item should read as one continuous
// thing, not N separate-looking copies of itself. Non-overlapping entries
// pack into the same grid row automatically; overlapping ones wrap to a
// second row (grid's default row-flow placement).
function SpanningRow({ days, entries, minHeightClass }: SpanningRowProps) {
  return (
    <div
      className={`grid flex-1 gap-1 p-1 ${minHeightClass}`}
      style={{ gridTemplateColumns: `repeat(${days.length}, minmax(0, 1fr))` }}
    >
      {entries.map((entry) => {
        const startIdx = days.indexOf(entry.startDay);
        const endIdx = days.indexOf(entry.endDay);
        if (startIdx === -1 && endIdx === -1) return null;
        const clampedStart = Math.max(startIdx, 0);
        const clampedEnd = endIdx === -1 ? days.length - 1 : endIdx;
        return (
          <div
            key={entry.id}
            style={{ gridColumn: `${clampedStart + 1} / ${clampedEnd + 2}` }}
            className="min-w-0"
          >
            {entry.content}
          </div>
        );
      })}
    </div>
  );
}

function AccommodationChip({ stay }: { stay: Accommodation }) {
  return (
    <div className="flex items-center gap-1 truncate rounded-md bg-muted px-1.5 py-0.5 text-xs">
      <span className="truncate">{stay.name}</span>
      {stay.location && <MapsLink query={stay.location} label="" />}
    </div>
  );
}

interface FlightSegment {
  flight: Flight;
  startMinute: number;
  endMinute: number;
}

interface FlightBlockProps {
  segment: FlightSegment;
}

// A flight's block in the hourly grid, positioned like a TimedBlock but
// read-only (flights are edited via the Flights table, not here) and
// visually distinct (dashed border) so it doesn't look editable. Split
// across day columns the same way a multi-day itinerary item is - see
// CalendarGrid's flightSegmentsForDay.
function FlightBlock({ segment }: FlightBlockProps) {
  const { flight, startMinute, endMinute } = segment;
  const top = yFromMinutes(startMinute, HOUR_HEIGHT_PX);
  const height = Math.max(
    yFromMinutes(endMinute - startMinute, HOUR_HEIGHT_PX),
    16,
  );

  return (
    <div
      style={{ top, height }}
      className="absolute inset-x-0 flex flex-col justify-between overflow-hidden rounded-md border border-dashed border-border bg-muted p-1 text-xs"
    >
      <div className="flex items-center gap-1">
        <PlaneTakeoff className="size-3 shrink-0" />
        <span className="truncate font-medium">{flight.flightNumber}</span>
      </div>
      <div className="flex items-center gap-1 text-muted-foreground">
        <span className="truncate">
          {flight.departureAirport} <PlaneLanding className="inline size-3" />{" "}
          {flight.arrivalAirport}
        </span>
      </div>
    </div>
  );
}

interface TimedSegment {
  item: ItineraryItem;
  startMinute: number;
  endMinute: number;
}

interface GridColumnProps {
  segments: TimedSegment[];
  flightSegments: FlightSegment[];
  onItemClick: (item: ItineraryItem) => void;
  onDelete: (id: string) => void;
  deleting: boolean;
}

function GridColumn({
  segments,
  flightSegments,
  onItemClick,
  onDelete,
  deleting,
}: GridColumnProps) {
  return (
    <div
      className="relative flex-1 border-l border-border"
      style={{ height: HOURS.length * HOUR_HEIGHT_PX }}
    >
      {HOURS.map((h) => (
        <div
          key={h}
          className="border-b border-border/50"
          style={{ height: HOUR_HEIGHT_PX }}
        />
      ))}
      {flightSegments.map((segment) => (
        <FlightBlock key={`${segment.flight.id}`} segment={segment} />
      ))}
      {segments.map(({ item, startMinute, endMinute }) => (
        <TimedBlock
          key={item.id}
          item={item}
          segmentStartMinute={startMinute}
          segmentEndMinute={endMinute}
          onClick={onItemClick}
          onDelete={onDelete}
          deleting={deleting}
        />
      ))}
    </div>
  );
}

interface CalendarGridProps {
  days: string[];
  items: ItineraryItem[];
  flights: Flight[];
  accommodations: Accommodation[];
  onItemClick: (item: ItineraryItem) => void;
  onDelete: (id: string) => void;
  deleting: boolean;
}

// The week/trip view: an Accommodation row summarizing stays, an all-day
// row, and a scrollable hourly grid below carrying both TIMED itinerary
// items and flights (positioned by their actual departure/arrival time,
// split across day columns when they cross midnight) - one column per day
// (see the plan's "Layout" decision: week view over a single-day
// switcher). Clicking an itinerary item (backlog or scheduled) opens the
// schedule-edit modal - see ItinerarySection. Flights/accommodation are
// read-only here - edit them in their own sections above.
export function CalendarGrid({
  days,
  items,
  flights,
  accommodations,
  onItemClick,
  onDelete,
  deleting,
}: CalendarGridProps) {
  const scrollRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = DEFAULT_SCROLL_HOUR * HOUR_HEIGHT_PX;
    }
  }, []);

  const timedSegmentsForDay = (day: string): TimedSegment[] => {
    const segments: TimedSegment[] = [];
    for (const item of items) {
      if (item.state !== ItineraryItemState.TIMED || !item.date) continue;
      const segment = segmentForDay(
        item.date,
        item.startMinute,
        item.endDate || item.date,
        item.endMinute,
        day,
      );
      if (segment) segments.push({ item, ...segment });
    }
    return segments;
  };

  const flightSegmentsForDay = (day: string): FlightSegment[] => {
    const segments: FlightSegment[] = [];
    for (const flight of flights) {
      const departure = flight.actualDeparture ?? flight.scheduledDeparture;
      const arrival = flight.actualArrival ?? flight.scheduledArrival;
      if (!departure || !arrival) continue; // unsynced - no times to position by

      const departureDate = timestampDate(departure);
      const arrivalDate = timestampDate(arrival);
      const segment = segmentForDay(
        localDateKey(departureDate, flight.departureTimezone),
        localMinuteOfDay(departureDate, flight.departureTimezone),
        localDateKey(arrivalDate, flight.arrivalTimezone),
        localMinuteOfDay(arrivalDate, flight.arrivalTimezone),
        day,
      );
      if (segment) segments.push({ flight, ...segment });
    }
    return segments;
  };

  const stayEntries: SpanningEntry[] = accommodations
    .filter((stay) => stay.checkIn && stay.checkOut)
    .map((stay) => ({
      id: stay.id,
      startDay: stay.checkIn,
      endDay: stay.checkOut,
      content: <AccommodationChip stay={stay} />,
    }));

  const allDayEntries: SpanningEntry[] = items
    .filter((item) => item.state === ItineraryItemState.ALL_DAY && item.date)
    .map((item) => ({
      id: item.id,
      startDay: item.date,
      endDay: item.endDate || item.date,
      content: (
        <AllDayBlock
          item={item}
          onClick={onItemClick}
          onDelete={onDelete}
          deleting={deleting}
        />
      ),
    }));

  return (
    <div className="flex flex-1 flex-col overflow-hidden rounded-lg border border-border">
      <div className="flex">
        <div className={GUTTER_WIDTH + " flex-none"} />
        {days.map((day) => (
          <div
            key={day}
            className="flex-1 border-l border-border p-1 text-center text-xs font-medium"
          >
            {formatDayHeader(day)}
          </div>
        ))}
      </div>

      <div className="flex border-t border-border">
        <div
          className={
            GUTTER_WIDTH +
            " flex-none p-1 text-right text-[10px] text-muted-foreground"
          }
        >
          Stay
        </div>
        <SpanningRow
          days={days}
          entries={stayEntries}
          minHeightClass="min-h-8"
        />
      </div>

      <div className="flex border-t border-border">
        <div
          className={
            GUTTER_WIDTH +
            " flex-none p-1 text-right text-[10px] text-muted-foreground"
          }
        >
          All day
        </div>
        <SpanningRow
          days={days}
          entries={allDayEntries}
          minHeightClass="min-h-10"
        />
      </div>

      <div
        ref={scrollRef}
        className="flex overflow-y-auto border-t border-border"
        style={{ maxHeight: HOUR_HEIGHT_PX * 12 }}
      >
        <div className={GUTTER_WIDTH + " flex-none"}>
          {HOURS.map((h) => (
            <div
              key={h}
              className="pr-1 text-right text-[10px] text-muted-foreground"
              style={{ height: HOUR_HEIGHT_PX }}
            >
              {String(h).padStart(2, "0")}:00
            </div>
          ))}
        </div>
        {days.map((day) => (
          <GridColumn
            key={day}
            segments={timedSegmentsForDay(day)}
            flightSegments={flightSegmentsForDay(day)}
            onItemClick={onItemClick}
            onDelete={onDelete}
            deleting={deleting}
          />
        ))}
      </div>
    </div>
  );
}
