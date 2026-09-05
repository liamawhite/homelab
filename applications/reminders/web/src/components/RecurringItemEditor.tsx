import { useState } from "react";
import type { MessageInitShape } from "@bufbuild/protobuf";
import { X } from "lucide-react";

import { Area } from "@/gen/reminders/v1/one_off_item_pb";
import { Weekday, RecurrenceRuleSchema, type RecurrenceRule } from "@/gen/reminders/v1/recurring_item_pb";
import { AREA_OPTIONS, AREA_LABELS, AREA_TEXT_CLASSES, AREA_BORDER_CLASSES } from "@/lib/area";
import { WEEKDAY_OPTIONS, WEEKDAY_SHORT_LABELS } from "@/lib/recurrenceDisplay";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";

export interface RecurringItemEditorValues {
  title: string;
  notes: string;
  area: Area;
  rule: MessageInitShape<typeof RecurrenceRuleSchema>;
}

type RuleKind = "weekly" | "monthly" | "yearly" | "interval";

const RULE_KIND_OPTIONS: RuleKind[] = ["weekly", "monthly", "yearly", "interval"];

const RULE_KIND_LABELS: Record<RuleKind, string> = {
  weekly: "Weekly",
  monthly: "Monthly",
  yearly: "Yearly",
  interval: "Every N days",
};

const MONTH_DAYS = Array.from({ length: 31 }, (_, i) => i + 1);

// A leap year, so Feb 29 is selectable via a native date input - only the
// month/day are ever read back out (see monthDayFromInputValue).
const DUMMY_YEAR = 2000;

interface RuleEditorState {
  kind: RuleKind;
  weeklyDays: Set<Weekday>;
  monthlyDays: Set<number>;
  yearlyDates: { month: number; day: number }[];
  intervalDays: number;
}

function emptyRuleEditorState(): RuleEditorState {
  return { kind: "weekly", weeklyDays: new Set(), monthlyDays: new Set(), yearlyDates: [], intervalDays: 1 };
}

function ruleToEditorState(rule: RecurrenceRule | undefined): RuleEditorState {
  switch (rule?.rule.case) {
    case "weekly":
      return { ...emptyRuleEditorState(), kind: "weekly", weeklyDays: new Set(rule.rule.value.daysOfWeek) };
    case "monthly":
      return { ...emptyRuleEditorState(), kind: "monthly", monthlyDays: new Set(rule.rule.value.daysOfMonth) };
    case "yearly":
      return {
        ...emptyRuleEditorState(),
        kind: "yearly",
        yearlyDates: rule.rule.value.dates.map((d) => ({ month: d.month, day: d.day })),
      };
    case "interval":
      return { ...emptyRuleEditorState(), kind: "interval", intervalDays: rule.rule.value.intervalDays };
    default:
      return emptyRuleEditorState();
  }
}

// Builds the wire RecurrenceRule from editor state, or null while the
// current selection isn't yet a valid rule (e.g. no weekdays picked yet).
function editorStateToRule(state: RuleEditorState): MessageInitShape<typeof RecurrenceRuleSchema> | null {
  switch (state.kind) {
    case "weekly":
      if (state.weeklyDays.size === 0) return null;
      return { rule: { case: "weekly", value: { daysOfWeek: [...state.weeklyDays] } } };
    case "monthly":
      if (state.monthlyDays.size === 0) return null;
      return { rule: { case: "monthly", value: { daysOfMonth: [...state.monthlyDays] } } };
    case "yearly":
      if (state.yearlyDates.length === 0) return null;
      return { rule: { case: "yearly", value: { dates: state.yearlyDates } } };
    case "interval":
      if (!Number.isFinite(state.intervalDays) || state.intervalDays < 1) return null;
      return { rule: { case: "interval", value: { intervalDays: state.intervalDays } } };
  }
}

function monthDayToInputValue(d: { month: number; day: number }): string {
  return `${DUMMY_YEAR}-${String(d.month).padStart(2, "0")}-${String(d.day).padStart(2, "0")}`;
}

function monthDayFromInputValue(value: string): { month: number; day: number } {
  const [, month, day] = value.split("-").map(Number);
  return { month, day };
}

function RuleEditor({ state, onChange }: { state: RuleEditorState; onChange: (state: RuleEditorState) => void }) {
  return (
    <div className="flex flex-col gap-2">
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button variant="outline" size="sm" type="button" className="w-fit">
            {RULE_KIND_LABELS[state.kind]}
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start">
          <DropdownMenuRadioGroup value={state.kind} onValueChange={(v) => onChange({ ...state, kind: v as RuleKind })}>
            {RULE_KIND_OPTIONS.map((kind) => (
              <DropdownMenuRadioItem key={kind} value={kind}>
                {RULE_KIND_LABELS[kind]}
              </DropdownMenuRadioItem>
            ))}
          </DropdownMenuRadioGroup>
        </DropdownMenuContent>
      </DropdownMenu>

      {state.kind === "weekly" && (
        <div className="flex flex-wrap gap-1">
          {WEEKDAY_OPTIONS.map((day) => (
            <Button
              key={day}
              type="button"
              size="sm"
              variant={state.weeklyDays.has(day) ? "default" : "outline"}
              onClick={() => {
                const next = new Set(state.weeklyDays);
                if (next.has(day)) next.delete(day);
                else next.add(day);
                onChange({ ...state, weeklyDays: next });
              }}
            >
              {WEEKDAY_SHORT_LABELS[day]}
            </Button>
          ))}
        </div>
      )}

      {state.kind === "monthly" && (
        <div className="flex max-w-xs flex-wrap gap-1">
          {MONTH_DAYS.map((day) => (
            <Button
              key={day}
              type="button"
              size="icon-sm"
              variant={state.monthlyDays.has(day) ? "default" : "outline"}
              onClick={() => {
                const next = new Set(state.monthlyDays);
                if (next.has(day)) next.delete(day);
                else next.add(day);
                onChange({ ...state, monthlyDays: next });
              }}
            >
              {day}
            </Button>
          ))}
        </div>
      )}

      {state.kind === "yearly" && (
        <div className="flex flex-col gap-1">
          {state.yearlyDates.map((date, i) => (
            <div key={i} className="flex items-center gap-1">
              <input
                type="date"
                className="h-8 rounded-md border border-input bg-transparent px-2 text-sm text-foreground outline-none focus-visible:border-ring"
                value={monthDayToInputValue(date)}
                onChange={(e) => {
                  const next = state.yearlyDates.slice();
                  next[i] = monthDayFromInputValue(e.target.value);
                  onChange({ ...state, yearlyDates: next });
                }}
              />
              <Button
                type="button"
                variant="ghost"
                size="icon-sm"
                aria-label="Remove date"
                onClick={() => onChange({ ...state, yearlyDates: state.yearlyDates.filter((_, idx) => idx !== i) })}
              >
                <X />
              </Button>
            </div>
          ))}
          <Button
            type="button"
            variant="secondary"
            size="sm"
            className="w-fit"
            onClick={() => onChange({ ...state, yearlyDates: [...state.yearlyDates, { month: 1, day: 1 }] })}
          >
            Add date
          </Button>
        </div>
      )}

      {state.kind === "interval" && (
        <div className="flex items-center gap-2">
          <span className="text-sm text-muted-foreground">Every</span>
          <Input
            type="number"
            min={1}
            className="w-20"
            value={state.intervalDays}
            onChange={(e) => onChange({ ...state, intervalDays: Number(e.target.value) })}
          />
          <span className="text-sm text-muted-foreground">days</span>
        </div>
      )}
    </div>
  );
}

function AreaChip({
  value,
  onChange,
  disabled,
}: {
  value: Area;
  onChange: (area: Area) => void;
  disabled?: boolean;
}) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          variant="outline"
          size="sm"
          type="button"
          disabled={disabled}
          className={cn("w-fit rounded-md", AREA_TEXT_CLASSES[value], AREA_BORDER_CLASSES[value])}
        >
          @{AREA_LABELS[value]}
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent>
        <DropdownMenuRadioGroup value={String(value)} onValueChange={(v) => onChange(Number(v) as Area)}>
          {AREA_OPTIONS.map((area) => (
            <DropdownMenuRadioItem key={area} value={String(area)} className={AREA_TEXT_CLASSES[area]}>
              @{AREA_LABELS[area]}
            </DropdownMenuRadioItem>
          ))}
        </DropdownMenuRadioGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

// The recurring-item creation/editing form - shared by RecurringItemsPage's
// "Add recurring reminder" flow and its click-to-edit flow, same convention
// as TaskEditor for one-off items.
export function RecurringItemEditor({
  initial,
  submitLabel,
  onSubmit,
  onCancel,
  pending,
  error,
}: {
  initial: { title: string; notes: string; area: Area; rule: RecurrenceRule | undefined };
  submitLabel: string;
  onSubmit: (values: RecurringItemEditorValues) => void;
  onCancel: () => void;
  pending?: boolean;
  error?: Error | null;
}) {
  const [title, setTitle] = useState(initial.title);
  const [notes, setNotes] = useState(initial.notes);
  const [area, setArea] = useState<Area>(initial.area);
  const [ruleState, setRuleState] = useState<RuleEditorState>(() => ruleToEditorState(initial.rule));

  const finalTitle = title.trim();
  const rule = editorStateToRule(ruleState);

  const handleSubmit = () => {
    if (!finalTitle || !rule) return;
    onSubmit({ title: finalTitle, notes: notes.trim(), area, rule });
  };

  return (
    <div className="flex flex-col gap-3 rounded-lg border border-input p-3">
      <Input autoFocus placeholder="Task name" value={title} onChange={(e) => setTitle(e.target.value)} />
      <Textarea placeholder="Notes" rows={1} value={notes} onChange={(e) => setNotes(e.target.value)} />
      <div className="flex flex-wrap items-center gap-2">
        <AreaChip value={area} onChange={setArea} />
      </div>
      <RuleEditor state={ruleState} onChange={setRuleState} />
      <div className="flex items-center justify-end gap-2 border-t border-input pt-2">
        {error && <p className="mr-auto text-sm text-destructive">{error.message}</p>}
        <Button variant="secondary" onClick={onCancel}>
          Cancel
        </Button>
        <Button onClick={handleSubmit} disabled={pending || !finalTitle || !rule}>
          {submitLabel}
        </Button>
      </div>
    </div>
  );
}
