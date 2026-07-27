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
