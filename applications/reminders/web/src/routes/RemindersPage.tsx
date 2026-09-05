import { useState } from "react";
import { timestampDate, timestampFromDate } from "@bufbuild/protobuf/wkt";
import { Calendar, MoreHorizontal, Tag, Trash2 } from "lucide-react";

import {
  useOneOffItems,
  useCreateOneOffItem,
  useUpdateOneOffItemTitle,
  useUpdateOneOffItemNotes,
  useUpdateOneOffItemDueDate,
  useUpdateOneOffItemArea,
  useUpdateOneOffItemStatus,
  useDeleteOneOffItem,
} from "@/lib/oneOffItems";
import { Area, OneOffItemStatus, type OneOffItem } from "@/gen/reminders/v1/one_off_item_pb";
import { AREA_LABELS, AREA_TEXT_CLASSES } from "@/lib/area";
import { formatChipDate, isOverdue } from "@/lib/time";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { TaskCheckbox } from "@/components/TaskCheckbox";
import { PageHeader } from "@/components/PageHeader";
import { AddTaskRow } from "@/components/AddTaskRow";
import { TaskEditor, type TaskEditorValues } from "@/components/TaskEditor";

function TaskRow({ item }: { item: OneOffItem }) {
  const [editing, setEditing] = useState(false);
  const [saveError, setSaveError] = useState<Error | null>(null);
  const updateTitle = useUpdateOneOffItemTitle();
  const updateNotes = useUpdateOneOffItemNotes();
  const updateDueDate = useUpdateOneOffItemDueDate();
  const updateArea = useUpdateOneOffItemArea();
  const updateStatus = useUpdateOneOffItemStatus();
  const deleteItem = useDeleteOneOffItem();
  const dueDate = item.dueDate ? timestampDate(item.dueDate) : undefined;
  const saving = updateTitle.isPending || updateNotes.isPending || updateDueDate.isPending || updateArea.isPending;

  const handleSave = async (values: TaskEditorValues) => {
    setSaveError(null);
    try {
      const tasks: Promise<unknown>[] = [];
      if (values.title !== item.title) tasks.push(updateTitle.mutateAsync({ id: item.id, title: values.title }));
      if (values.notes !== item.notes) tasks.push(updateNotes.mutateAsync({ id: item.id, notes: values.notes }));
      if ((values.dueDate?.getTime() ?? null) !== (dueDate?.getTime() ?? null)) {
        tasks.push(
          updateDueDate.mutateAsync({ id: item.id, dueDate: values.dueDate ? timestampFromDate(values.dueDate) : undefined })
        );
      }
      if (values.area !== item.area) tasks.push(updateArea.mutateAsync({ id: item.id, area: values.area }));
      await Promise.all(tasks);
      setEditing(false);
    } catch (err) {
      setSaveError(err instanceof Error ? err : new Error(String(err)));
    }
  };

  if (editing) {
    return (
      <div className="border-b border-border py-2.5 first:pt-0">
        <TaskEditor
          initial={{ title: item.title, notes: item.notes, dueDate, area: item.area }}
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
          disabled={updateStatus.isPending}
          onCheck={() => updateStatus.mutate({ id: item.id, status: OneOffItemStatus.COMPLETED })}
          aria-label={`Complete ${item.title}`}
        />
        <button
          type="button"
          onClick={() => setEditing(true)}
          className="flex flex-1 flex-col gap-1.5 rounded-md text-left outline-none"
        >
          <span className="text-sm font-medium">{item.title}</span>
          <div className="flex flex-wrap items-center gap-3 text-sm">
            {dueDate && (
              <span className={cn("inline-flex items-center gap-1", isOverdue(dueDate) ? "text-destructive" : "text-muted-foreground")}>
                <Calendar className="size-3.5" />
                {formatChipDate(dueDate)}
              </span>
            )}
            <span className={cn("inline-flex items-center gap-1", AREA_TEXT_CLASSES[item.area])}>
              <Tag className="size-3.5" />@{AREA_LABELS[item.area]}
            </span>
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

export function RemindersPage() {
  const { data: items, isLoading, isError, error } = useOneOffItems();
  const createItem = useCreateOneOffItem();
  const [isAdding, setIsAdding] = useState(false);

  const handleCreate = (values: TaskEditorValues) => {
    createItem.mutate(
      {
        title: values.title,
        notes: values.notes || undefined,
        area: values.area,
        dueDate: values.dueDate ? timestampFromDate(values.dueDate) : undefined,
      },
      {
        onSuccess: () => setIsAdding(false),
      }
    );
  };

  return (
    <div className="mx-auto flex w-full max-w-2xl flex-1 flex-col gap-4 p-4">
      <PageHeader title="Today" count={items?.length ?? 0} />

      {isAdding ? (
        <TaskEditor
          initial={{ title: "", notes: "", dueDate: undefined, area: Area.PERSONAL }}
          submitLabel="Add task"
          pending={createItem.isPending}
          error={createItem.error}
          onSubmit={handleCreate}
          onCancel={() => setIsAdding(false)}
        />
      ) : (
        <AddTaskRow onClick={() => setIsAdding(true)} />
      )}

      {isLoading && <p className="text-sm text-muted-foreground">Loading reminders…</p>}
      {isError && <p className="text-sm text-destructive">{error.message}</p>}

      {items && items.length > 0 && (
        <div className="flex flex-col">
          {items.map((item) => (
            <TaskRow key={item.id} item={item} />
          ))}
        </div>
      )}
      {!isLoading && items && items.length === 0 && (
        <p className="text-sm text-muted-foreground">No reminders yet.</p>
      )}
    </div>
  );
}
