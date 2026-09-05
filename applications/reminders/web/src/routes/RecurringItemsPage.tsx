import { useState } from "react";
import type { MessageInitShape } from "@bufbuild/protobuf";
import { timestampDate } from "@bufbuild/protobuf/wkt";
import { MoreHorizontal, NotebookPen, Repeat, Trash2, X } from "lucide-react";

import {
  useRecurringItems,
  useCreateRecurringItem,
  useUpdateRecurringItemNotes,
  useUpdateRecurringItemArea,
  useUpdateRecurringItemRule,
  useCompleteRecurringItem,
  useDeleteRecurringItem,
} from "@/lib/recurringItems";
import { Area } from "@/gen/reminders/v1/one_off_item_pb";
import { Weekday, RecurrenceRuleSchema, type RecurrenceRule, type RecurringItem } from "@/gen/reminders/v1/recurring_item_pb";
import { AREA_OPTIONS, AREA_LABELS, AREA_TEXT_CLASSES, AREA_BORDER_CLASSES } from "@/lib/area";
import { WEEKDAY_OPTIONS, WEEKDAY_SHORT_LABELS, formatRule } from "@/lib/recurrenceDisplay";
import { formatChipDate, isOverdue } from "@/lib/time";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { TaskCheckbox } from "@/components/TaskCheckbox";
import { PageHeader } from "@/components/PageHeader";
import { AddTaskRow } from "@/components/AddTaskRow";

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

function RecurringItemEditPanel({
  item,
  updateNotes,
  updateRule,
}: {
  item: RecurringItem;
  updateNotes: ReturnType<typeof useUpdateRecurringItemNotes>;
  updateRule: ReturnType<typeof useUpdateRecurringItemRule>;
}) {
  const [notes, setNotes] = useState(item.notes);
  const [ruleState, setRuleState] = useState<RuleEditorState>(() => ruleToEditorState(item.rule));
  const notesDirty = notes !== item.notes;
  const rule = editorStateToRule(ruleState);

  return (
    <div className="mt-2 flex flex-col gap-3 rounded-md bg-muted/30 p-2">
      <div className="flex flex-col gap-2">
        <Textarea placeholder="Add notes…" value={notes} onChange={(e) => setNotes(e.target.value)} />
        <div className="flex items-center justify-end gap-2">
          {updateNotes.isError && <p className="text-sm text-destructive">{updateNotes.error.message}</p>}
          <Button
            size="sm"
            disabled={!notesDirty || updateNotes.isPending}
            onClick={() => updateNotes.mutate({ id: item.id, notes })}
          >
            Save notes
          </Button>
        </div>
      </div>
      <div className="flex flex-col gap-2 border-t border-input pt-3">
        <RuleEditor state={ruleState} onChange={setRuleState} />
        <div className="flex items-center justify-end gap-2">
          {updateRule.isError && <p className="text-sm text-destructive">{updateRule.error.message}</p>}
          <Button size="sm" disabled={!rule || updateRule.isPending} onClick={() => rule && updateRule.mutate({ id: item.id, rule })}>
            Save schedule
          </Button>
        </div>
      </div>
    </div>
  );
}

function RecurringItemRow({
  item,
  expanded,
  onToggleExpanded,
  updateArea,
  completeItem,
  deleteItem,
}: {
  item: RecurringItem;
  expanded: boolean;
  onToggleExpanded: () => void;
  updateArea: ReturnType<typeof useUpdateRecurringItemArea>;
  completeItem: ReturnType<typeof useCompleteRecurringItem>;
  deleteItem: ReturnType<typeof useDeleteRecurringItem>;
}) {
  const updateNotes = useUpdateRecurringItemNotes();
  const updateRule = useUpdateRecurringItemRule();
  const nextDueDate = item.nextDueDate ? timestampDate(item.nextDueDate) : undefined;
  const overdue = nextDueDate !== undefined && isOverdue(nextDueDate);

  return (
    <div className="group flex flex-col gap-1.5 border-b border-border py-2.5 first:pt-0">
      <div className="flex items-start gap-2.5">
        <TaskCheckbox
          checked={false}
          colorClassName={AREA_TEXT_CLASSES[item.area]}
          disabled={completeItem.isPending}
          onCheck={() => completeItem.mutate(item.id)}
          aria-label={`Complete ${item.title}`}
        />
        <div className="flex flex-1 flex-col gap-1.5">
          <div className="flex items-center justify-between gap-2">
            <span className="text-sm font-medium">{item.title}</span>
            <div className="flex items-center gap-0.5 opacity-0 transition-opacity group-hover:opacity-100">
              <Button
                variant="ghost"
                size="icon-sm"
                aria-label={`${expanded ? "Hide" : "Show"} details for ${item.title}`}
                aria-expanded={expanded}
                onClick={onToggleExpanded}
              >
                <NotebookPen />
              </Button>
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button variant="ghost" size="icon-sm" aria-label={`More actions for ${item.title}`}>
                    <MoreHorizontal />
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end">
                  <DropdownMenuItem variant="destructive" disabled={deleteItem.isPending} onSelect={() => deleteItem.mutate(item.id)}>
                    <Trash2 /> Delete
                  </DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
            </div>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <span className={cn("inline-flex items-center gap-1 text-sm", overdue ? "text-destructive" : "text-muted-foreground")}>
              <Repeat className="size-3.5" />
              {nextDueDate ? formatChipDate(nextDueDate) : null} · {formatRule(item.rule)}
            </span>
            <AreaChip value={item.area} disabled={updateArea.isPending} onChange={(area) => updateArea.mutate({ id: item.id, area })} />
          </div>
          {deleteItem.isError && <p className="text-sm text-destructive">{deleteItem.error.message}</p>}
        </div>
      </div>
      {expanded && <RecurringItemEditPanel item={item} updateNotes={updateNotes} updateRule={updateRule} />}
    </div>
  );
}

export function RecurringItemsPage() {
  const { data: items, isLoading, isError, error } = useRecurringItems();
  const createItem = useCreateRecurringItem();
  const updateArea = useUpdateRecurringItemArea();
  const completeItem = useCompleteRecurringItem();
  const deleteItem = useDeleteRecurringItem();

  const [isAdding, setIsAdding] = useState(false);
  const [title, setTitle] = useState("");
  const [notes, setNotes] = useState("");
  const [area, setArea] = useState<Area>(Area.PERSONAL);
  const [newRuleState, setNewRuleState] = useState<RuleEditorState>(emptyRuleEditorState);
  const [expanded, setExpanded] = useState<Set<string>>(new Set());

  const finalTitle = title.trim();
  const rule = editorStateToRule(newRuleState);

  const resetForm = () => {
    setTitle("");
    setNotes("");
    setArea(Area.PERSONAL);
    setNewRuleState(emptyRuleEditorState());
  };

  const handleCreate = () => {
    if (!finalTitle || !rule) return;

    createItem.mutate(
      { title: finalTitle, notes: notes.trim() || undefined, area, rule },
      {
        onSuccess: () => {
          resetForm();
          setIsAdding(false);
        },
      }
    );
  };

  const handleCancel = () => {
    resetForm();
    setIsAdding(false);
  };

  const toggleExpanded = (id: string) => {
    setExpanded((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  return (
    <div className="mx-auto flex w-full max-w-2xl flex-1 flex-col gap-4 p-4">
      <PageHeader title="Recurring" count={items?.length ?? 0} />

      {isAdding ? (
        <div className="flex flex-col gap-3 rounded-lg border border-input p-3">
          <Input autoFocus placeholder="Task name" value={title} onChange={(e) => setTitle(e.target.value)} />
          <Textarea placeholder="Notes" rows={1} value={notes} onChange={(e) => setNotes(e.target.value)} />
          <div className="flex flex-wrap items-center gap-2">
            <AreaChip value={area} onChange={setArea} />
          </div>
          <RuleEditor state={newRuleState} onChange={setNewRuleState} />
          <div className="flex items-center justify-end gap-2 border-t border-input pt-2">
            <Button variant="secondary" onClick={handleCancel}>
              Cancel
            </Button>
            <Button onClick={handleCreate} disabled={createItem.isPending || !finalTitle || !rule}>
              Add recurring reminder
            </Button>
          </div>
          {createItem.isError && <p className="text-sm text-destructive">{createItem.error.message}</p>}
        </div>
      ) : (
        <AddTaskRow onClick={() => setIsAdding(true)} />
      )}

      {isLoading && <p className="text-sm text-muted-foreground">Loading recurring reminders…</p>}
      {isError && <p className="text-sm text-destructive">{error.message}</p>}

      {items && items.length > 0 && (
        <div className="flex flex-col">
          {items.map((item) => (
            <RecurringItemRow
              key={item.id}
              item={item}
              expanded={expanded.has(item.id)}
              onToggleExpanded={() => toggleExpanded(item.id)}
              updateArea={updateArea}
              completeItem={completeItem}
              deleteItem={deleteItem}
            />
          ))}
        </div>
      )}
      {!isLoading && items && items.length === 0 && <p className="text-sm text-muted-foreground">No recurring reminders yet.</p>}
    </div>
  );
}
