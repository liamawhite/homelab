import { useState } from "react";
import type { MessageInitShape } from "@bufbuild/protobuf";
import { timestampDate } from "@bufbuild/protobuf/wkt";
import { MoreHorizontal, Repeat, Trash2 } from "lucide-react";

import {
  useRecurringItems,
  useCreateRecurringItem,
  useUpdateRecurringItemTitle,
  useUpdateRecurringItemNotes,
  useUpdateRecurringItemArea,
  useUpdateRecurringItemRule,
  useCompleteRecurringItem,
  useDeleteRecurringItem,
} from "@/lib/recurringItems";
import { Area } from "@/gen/reminders/v1/one_off_item_pb";
import { RecurrenceRuleSchema, type RecurrenceRule, type RecurringItem } from "@/gen/reminders/v1/recurring_item_pb";
import { AREA_LABELS, AREA_TEXT_CLASSES } from "@/lib/area";
import { formatRule } from "@/lib/recurrenceDisplay";
import { formatChipDate, isOverdue } from "@/lib/time";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { TaskCheckbox } from "@/components/TaskCheckbox";
import { PageHeader } from "@/components/PageHeader";
import { AddTaskRow } from "@/components/AddTaskRow";
import { RecurringItemEditor, type RecurringItemEditorValues } from "@/components/RecurringItemEditor";

// Whether a freshly-edited rule actually differs from the item's current
// one - used to skip UpdateRecurringItemRule when only e.g. notes changed,
// since that RPC always recomputes next_due_date from today (see its doc
// comment), which would silently reset an untouched schedule.
function ruleChanged(next: MessageInitShape<typeof RecurrenceRuleSchema>, prev: RecurrenceRule | undefined): boolean {
  if (next.rule?.case !== prev?.rule.case) return true;
  const sortedNums = (a: number[]) => [...a].sort((x, y) => x - y);
  switch (next.rule?.case) {
    case "weekly":
      return (
        prev?.rule.case !== "weekly" ||
        !sameArray(sortedNums(next.rule.value.daysOfWeek ?? []), sortedNums(prev.rule.value.daysOfWeek))
      );
    case "monthly":
      return (
        prev?.rule.case !== "monthly" ||
        !sameArray(sortedNums(next.rule.value.daysOfMonth ?? []), sortedNums(prev.rule.value.daysOfMonth))
      );
    case "yearly": {
      if (prev?.rule.case !== "yearly") return true;
      const key = (d: { month?: number; day?: number }) => `${d.month ?? 0}-${d.day ?? 0}`;
      const nextKeys = (next.rule.value.dates ?? []).map(key).sort();
      const prevKeys = prev.rule.value.dates.map(key).sort();
      return !sameArray(nextKeys, prevKeys);
    }
    case "interval":
      return prev?.rule.case !== "interval" || next.rule.value.intervalDays !== prev.rule.value.intervalDays;
    default:
      return false;
  }
}

function sameArray<T>(a: T[], b: T[]): boolean {
  return a.length === b.length && a.every((v, i) => v === b[i]);
}

function RecurringItemRow({
  item,
  completeItem,
  deleteItem,
}: {
  item: RecurringItem;
  completeItem: ReturnType<typeof useCompleteRecurringItem>;
  deleteItem: ReturnType<typeof useDeleteRecurringItem>;
}) {
  const [editing, setEditing] = useState(false);
  const [saveError, setSaveError] = useState<Error | null>(null);
  const updateTitle = useUpdateRecurringItemTitle();
  const updateNotes = useUpdateRecurringItemNotes();
  const updateArea = useUpdateRecurringItemArea();
  const updateRule = useUpdateRecurringItemRule();
  const nextDueDate = item.nextDueDate ? timestampDate(item.nextDueDate) : undefined;
  const overdue = nextDueDate !== undefined && isOverdue(nextDueDate);
  const saving = updateTitle.isPending || updateNotes.isPending || updateArea.isPending || updateRule.isPending;

  const handleSave = async (values: RecurringItemEditorValues) => {
    setSaveError(null);
    try {
      const tasks: Promise<unknown>[] = [];
      if (values.title !== item.title) tasks.push(updateTitle.mutateAsync({ id: item.id, title: values.title }));
      if (values.notes !== item.notes) tasks.push(updateNotes.mutateAsync({ id: item.id, notes: values.notes }));
      if (values.area !== item.area) tasks.push(updateArea.mutateAsync({ id: item.id, area: values.area }));
      if (ruleChanged(values.rule, item.rule)) tasks.push(updateRule.mutateAsync({ id: item.id, rule: values.rule }));
      await Promise.all(tasks);
      setEditing(false);
    } catch (err) {
      setSaveError(err instanceof Error ? err : new Error(String(err)));
    }
  };

  if (editing) {
    return (
      <div className="border-b border-border py-2.5 first:pt-0">
        <RecurringItemEditor
          initial={{ title: item.title, notes: item.notes, area: item.area, rule: item.rule }}
          submitLabel="Save"
          pending={saving}
          error={saveError}
          onSubmit={handleSave}
          onCancel={() => setEditing(false)}
        />
      </div>
    );
  }

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
        <button
          type="button"
          onClick={() => setEditing(true)}
          className="flex flex-1 flex-col gap-1.5 rounded-md text-left outline-none"
        >
          <span className="text-sm font-medium">{item.title}</span>
          <div className="flex flex-wrap items-center gap-3 text-sm">
            <span className={cn("inline-flex items-center gap-1", overdue ? "text-destructive" : "text-muted-foreground")}>
              <Repeat className="size-3.5" />
              {nextDueDate ? formatChipDate(nextDueDate) : null} · {formatRule(item.rule)}
            </span>
            <span className={cn("inline-flex items-center gap-1", AREA_TEXT_CLASSES[item.area])}>@{AREA_LABELS[item.area]}</span>
          </div>
        </button>
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button
              variant="ghost"
              size="icon-sm"
              aria-label={`More actions for ${item.title}`}
              className="opacity-0 transition-opacity group-hover:opacity-100"
            >
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
      {deleteItem.isError && <p className="text-sm text-destructive">{deleteItem.error.message}</p>}
    </div>
  );
}

export function RecurringItemsPage() {
  const { data: items, isLoading, isError, error } = useRecurringItems();
  const createItem = useCreateRecurringItem();
  const completeItem = useCompleteRecurringItem();
  const deleteItem = useDeleteRecurringItem();

  const [isAdding, setIsAdding] = useState(false);

  const handleCreate = (values: RecurringItemEditorValues) => {
    createItem.mutate(
      { title: values.title, notes: values.notes || undefined, area: values.area, rule: values.rule },
      {
        onSuccess: () => setIsAdding(false),
      }
    );
  };

  return (
    <div className="mx-auto flex w-full max-w-2xl flex-1 flex-col gap-4 p-4">
      <PageHeader title="Recurring" count={items?.length ?? 0} />

      {isAdding ? (
        <RecurringItemEditor
          initial={{ title: "", notes: "", area: Area.PERSONAL, rule: undefined }}
          submitLabel="Add recurring reminder"
          pending={createItem.isPending}
          error={createItem.error}
          onSubmit={handleCreate}
          onCancel={() => setIsAdding(false)}
        />
      ) : (
        <AddTaskRow onClick={() => setIsAdding(true)} />
      )}

      {isLoading && <p className="text-sm text-muted-foreground">Loading recurring reminders…</p>}
      {isError && <p className="text-sm text-destructive">{error.message}</p>}

      {items && items.length > 0 && (
        <div className="flex flex-col">
          {items.map((item) => (
            <RecurringItemRow key={item.id} item={item} completeItem={completeItem} deleteItem={deleteItem} />
          ))}
        </div>
      )}
      {!isLoading && items && items.length === 0 && <p className="text-sm text-muted-foreground">No recurring reminders yet.</p>}
    </div>
  );
}
