import { Weekday, type RecurrenceRule } from "@/gen/reminders/v1/recurring_item_pb";

// The selectable weekdays, in Monday-first display order (matches how most
// calendar UIs lay out a week) - WEEKDAY_UNSPECIFIED is deliberately
// excluded, same convention as AREA_OPTIONS in area.ts.
export const WEEKDAY_OPTIONS: Weekday[] = [
  Weekday.MONDAY,
  Weekday.TUESDAY,
  Weekday.WEDNESDAY,
  Weekday.THURSDAY,
  Weekday.FRIDAY,
  Weekday.SATURDAY,
  Weekday.SUNDAY,
];

export const WEEKDAY_SHORT_LABELS: Record<Weekday, string> = {
  [Weekday.UNSPECIFIED]: "",
  [Weekday.SUNDAY]: "Sun",
  [Weekday.MONDAY]: "Mon",
  [Weekday.TUESDAY]: "Tue",
  [Weekday.WEDNESDAY]: "Wed",
  [Weekday.THURSDAY]: "Thu",
  [Weekday.FRIDAY]: "Fri",
  [Weekday.SATURDAY]: "Sat",
};

const MONTH_SHORT_LABELS = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];

// Monday(2)-first sort order for a set of Weekday values, wrapping Sunday(1)
// to the end.
function weekdayOrder(w: Weekday): number {
  return (w - Weekday.MONDAY + 7) % 7;
}

function ordinal(n: number): string {
  if (n % 100 >= 11 && n % 100 <= 13) return `${n}th`;
  switch (n % 10) {
    case 1:
      return `${n}st`;
    case 2:
      return `${n}nd`;
    case 3:
      return `${n}rd`;
    default:
      return `${n}th`;
  }
}

// Formats a RecurrenceRule for display, e.g. "Every Mon, Wed, Fri",
// "Monthly on the 1st & 15th", "Every year on Jan 1, Jul 4", "Every 10
// days". Purely presentational - the next occurrence itself is always the
// server-computed next_due_date (see time.ts's formatChipDate), never
// recomputed client-side.
export function formatRule(rule: RecurrenceRule | undefined): string {
  switch (rule?.rule.case) {
    case "weekly": {
      const days = rule.rule.value.daysOfWeek
        .slice()
        .sort((a, b) => weekdayOrder(a) - weekdayOrder(b))
        .map((d) => WEEKDAY_SHORT_LABELS[d]);
      return `Every ${days.join(", ")}`;
    }
    case "monthly": {
      const days = rule.rule.value.daysOfMonth
        .slice()
        .sort((a, b) => a - b)
        .map(ordinal);
      return `Monthly on the ${days.join(" & ")}`;
    }
    case "yearly": {
      const dates = rule.rule.value.dates
        .slice()
        .sort((a, b) => a.month - b.month || a.day - b.day)
        .map((d) => `${MONTH_SHORT_LABELS[d.month - 1]} ${d.day}`);
      return `Every year on ${dates.join(", ")}`;
    }
    case "interval":
      return rule.rule.value.intervalDays === 1 ? "Every day" : `Every ${rule.rule.value.intervalDays} days`;
    default:
      return "No schedule set";
  }
}
