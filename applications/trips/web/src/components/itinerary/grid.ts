// Shared constants/pixel math for laying out the hourly grid.

export const DEFAULT_DURATION_MINUTES = 60;
export const MINUTES_PER_DAY = 24 * 60;
// Matches the --hour-height CSS custom property set on CalendarGrid's
// scrollable container (same "component-scoped custom property" pattern as
// Card's --card-spacing) - kept as a JS constant too since a scheduled
// block's positioning math needs the same value pixel-for-pixel.
export const HOUR_HEIGHT_PX = 48;

// yFromMinutes converts minutes-since-midnight into a Y offset (px) at the
// given hour height - used to position/size a TIMED item's block.
export function yFromMinutes(minutes: number, hourHeightPx: number): number {
  return (minutes / 60) * hourHeightPx;
}

// minutesToTimeInput/timeInputToMinutes convert between minutes-since-
// midnight and the "HH:MM" string an <input type="time"> expects.
export function minutesToTimeInput(minutes: number): string {
  const h = Math.floor(minutes / 60)
    .toString()
    .padStart(2, "0");
  const m = (minutes % 60).toString().padStart(2, "0");
  return `${h}:${m}`;
}

export function timeInputToMinutes(value: string): number {
  const [h, m] = value.split(":").map(Number);
  return h * 60 + m;
}

// segmentForDay computes the portion (if any) of a span running from
// (startDay, startMinute) to (endDay, endMinute) that falls on `day`, for
// positioning one day-column's block. Shared by multi-day itinerary items
// and flights (which almost always span two calendar days in transit) -
// both are just "an event with a start day/minute and an end day/minute"
// once flight instants are resolved to local day/minute-of-day (see
// lib/time.ts's localDateKey/localMinuteOfDay for that resolution step).
export function segmentForDay(
  startDay: string,
  startMinute: number,
  endDay: string,
  endMinute: number,
  day: string,
): { startMinute: number; endMinute: number } | null {
  if (day < startDay || day > endDay) return null;
  return {
    startMinute: day === startDay ? startMinute : 0,
    endMinute: day === endDay ? endMinute : MINUTES_PER_DAY,
  };
}
