const DIVISIONS: [Intl.RelativeTimeFormatUnit, number][] = [
  ["year", 60 * 60 * 24 * 365],
  ["month", 60 * 60 * 24 * 30],
  ["week", 60 * 60 * 24 * 7],
  ["day", 60 * 60 * 24],
  ["hour", 60 * 60],
  ["minute", 60],
];

const rtf = new Intl.RelativeTimeFormat("en", { numeric: "auto" });

// Formats a past Date as "3 days ago", "2 hours ago", etc., falling back to
// "just now" for anything under a minute.
export function relativeTime(date: Date): string {
  const seconds = (date.getTime() - Date.now()) / 1000;

  for (const [unit, secondsInUnit] of DIVISIONS) {
    if (Math.abs(seconds) >= secondsInUnit) {
      return rtf.format(Math.round(seconds / secondsInUnit), unit);
    }
  }

  return "just now";
}

// dateStr is a plain YYYY-MM-DD calendar date (no time/zone) - parsed via
// explicit local-Date components rather than `new Date(dateStr)` (which
// parses as UTC midnight and can display as the previous day in negative
// UTC-offset zones).
export function formatDate(dateStr: string) {
  if (!dateStr) return "—";
  const [year, month, day] = dateStr.split("-").map(Number);
  return new Date(year, month - 1, day).toLocaleDateString(undefined, {
    month: "short",
    day: "numeric",
    year: "numeric",
  });
}

// minutesToClock formats minutes-since-midnight as "1:00 PM" - purely a
// number formatter, no Date/timezone involved. `% 24` on hour24 handles the
// minutes=1440 (midnight/24:00) edge case - the end of a multi-day item's
// first-day segment (see grid.ts's segmentForDay) - as "12:00 AM", not the
// wrong "12:00 PM" a naive `hour24 < 12` check would give for hour24 === 24.
export function minutesToClock(minutes: number) {
  const hour24 = Math.floor(minutes / 60) % 24;
  const minute = minutes % 60;
  const period = hour24 < 12 ? "AM" : "PM";
  const hour12 = hour24 % 12 === 0 ? 12 : hour24 % 12;
  return `${hour12}:${String(minute).padStart(2, "0")} ${period}`;
}

// localDateKey resolves the YYYY-MM-DD calendar date `date` falls on within
// `tz` - used to place a flight (a UTC instant) into the correct day column
// of the itinerary calendar, which is keyed by local calendar date. Falls
// back to the browser's own zone if tz is empty (matches formatTime's
// convention elsewhere for an unsynced flight).
export function localDateKey(date: Date, tz?: string): string {
  const parts = new Intl.DateTimeFormat("en-CA", {
    timeZone: tz || undefined,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  }).formatToParts(date);
  const get = (type: string) => parts.find((p) => p.type === type)?.value ?? "";
  return `${get("year")}-${get("month")}-${get("day")}`;
}

// localMinuteOfDay resolves the minutes-since-midnight `date` falls on
// within `tz` - paired with localDateKey to place a flight (a UTC instant)
// at the right position within its local day column in the itinerary
// calendar.
export function localMinuteOfDay(date: Date, tz?: string): number {
  const parts = new Intl.DateTimeFormat("en-US", {
    timeZone: tz || undefined,
    hour: "2-digit",
    minute: "2-digit",
    hourCycle: "h23",
  }).formatToParts(date);
  const hour = Number(parts.find((p) => p.type === "hour")?.value ?? "0");
  const minute = Number(parts.find((p) => p.type === "minute")?.value ?? "0");
  return hour * 60 + minute;
}

// zoneAbbreviation resolves the short abbreviation (e.g. "BST", "PDT") an
// IANA zone uses on dateStr - rendering-only, this never converts the
// item's own date/time, it only labels what zone those values are in.
// Anchored at noon UTC on dateStr (well clear of any DST-transition hour)
// so the result reflects that calendar date's DST status in tz regardless
// of the item's own clock time.
export function zoneAbbreviation(dateStr: string, tz: string): string {
  if (!dateStr || !tz) return "";
  try {
    const anchor = new Date(`${dateStr}T12:00:00Z`);
    const parts = new Intl.DateTimeFormat(undefined, {
      timeZone: tz,
      timeZoneName: "short",
    }).formatToParts(anchor);
    return parts.find((p) => p.type === "timeZoneName")?.value ?? tz;
  } catch {
    return tz;
  }
}
