import * as chrono from "chrono-node";

import { Area } from "@/gen/reminders/v1/one_off_item_pb";
import { AREA_BY_LABEL } from "@/lib/area";
import { startOfTodayUTC } from "@/lib/time";

// Todoist-style quick-entry: typing "@work" or "friday" anywhere in the
// title is parsed out on submit and turned into structured fields, rather
// than staying part of the title text.

const AREA_TOKEN_RE = /(^|\s)@(personal|family|work|professional)\b/i;

export function extractArea(text: string): { area: Area; rest: string } | null {
  const match = text.match(AREA_TOKEN_RE);
  if (!match || match.index === undefined) return null;

  const area = AREA_BY_LABEL[match[2].toLowerCase()];
  const rest = (text.slice(0, match.index) + text.slice(match.index + match[0].length))
    .replace(/\s+/g, " ")
    .trim();
  return { area, rest };
}

// Delegates to chrono-node for anything beyond the literal "today"/
// "tomorrow"/weekday cases - it understands "in 3 days", "next week", "Aug
// 20", etc.
//
// Parses naively first: chrono's default resolution already treats naming
// today's own weekday as today (not next week), which forwardDate:true
// would get wrong (it forces the *next* occurrence even when today
// matches). But naive resolution can also land an unqualified month/day
// ("March 3") in the past if that date already passed this year. So: parse
// naively, and only if the result is before today, reparse the same text
// with forwardDate:true to push it forward (to next year, in that case).
export function extractDueDate(text: string, now: Date = new Date()): { date: Date; rest: string } | null {
  let [result] = chrono.parse(text, now);
  if (result && toUTCMidnight(result.date()).getTime() < startOfTodayUTC(now).getTime()) {
    [result] = chrono.parse(text, now, { forwardDate: true });
  }
  if (!result) return null;

  // chrono resolves relative to local time; normalize to UTC midnight of
  // that same calendar day so it lines up with how due dates are
  // stored/compared everywhere else in this app (see time.ts).
  const date = toUTCMidnight(result.date());

  const rest = (text.slice(0, result.index) + text.slice(result.index + result.text.length))
    .replace(/\s+/g, " ")
    .trim();
  return { date, rest };
}

function toUTCMidnight(date: Date): Date {
  return new Date(Date.UTC(date.getFullYear(), date.getMonth(), date.getDate()));
}
