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

// Formats a Date as the "YYYY-MM-DD" value an <input type="date"> expects,
// in UTC (due dates are stored/created as UTC midnight - see
// RemindersPage.tsx).
export function toDateInputValue(date: Date): string {
  return date.toISOString().slice(0, 10);
}

// Parses an <input type="date"> value ("YYYY-MM-DD") back to a Date at UTC
// midnight - the inverse of toDateInputValue.
export function fromDateInputValue(value: string): Date {
  return new Date(`${value}T00:00:00.000Z`);
}

// A due date is overdue only once its calendar day has fully passed -
// today's due date is not overdue just because midnight UTC is behind us.
export function isOverdue(date: Date): boolean {
  return date.getTime() < startOfTodayUTC().getTime();
}

export function startOfTodayUTC(now: Date = new Date()): Date {
  return new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate()));
}

export function addDays(date: Date, days: number): Date {
  const result = new Date(date);
  result.setUTCDate(result.getUTCDate() + days);
  return result;
}

// Formats a UTC-midnight due date the way Todoist does: "Today"/"Tomorrow"/
// "Yesterday" near the present, otherwise a short "Aug 14" date.
export function formatChipDate(date: Date, now: Date = new Date()): string {
  const today = startOfTodayUTC(now);
  const diffDays = Math.round((date.getTime() - today.getTime()) / 86_400_000);
  if (diffDays === 0) return "Today";
  if (diffDays === 1) return "Tomorrow";
  if (diffDays === -1) return "Yesterday";
  return date.toLocaleDateString("en-US", { timeZone: "UTC", month: "short", day: "numeric" });
}
