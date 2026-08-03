// Full IANA zone list, sourced from the runtime rather than a hardcoded
// list or extra dependency - supported in all evergreen browsers.
export const IANA_TIMEZONES: string[] = (() => {
  try {
    return Intl.supportedValuesOf("timeZone");
  } catch {
    return ["UTC"];
  }
})();

export function browserTimezone(): string {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone;
  } catch {
    return "UTC";
  }
}

const DEFAULT_TIMEZONE_STORAGE_KEY = "trips:itinerary:defaultTimezone";

// The calendar's "new items in" dropdown persists across reloads via
// localStorage rather than the backend - it's a client-side default for
// new items, not trip data.
export function loadDefaultTimezone(): string {
  try {
    return (
      localStorage.getItem(DEFAULT_TIMEZONE_STORAGE_KEY) || browserTimezone()
    );
  } catch {
    return browserTimezone();
  }
}

export function saveDefaultTimezone(tz: string) {
  try {
    localStorage.setItem(DEFAULT_TIMEZONE_STORAGE_KEY, tz);
  } catch {
    // localStorage unavailable (private browsing, etc.) - fine, this is
    // just a convenience default.
  }
}
