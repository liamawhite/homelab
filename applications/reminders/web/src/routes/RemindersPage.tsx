import { useEffect, useRef, useState } from "react";
import { timestampDate, timestampFromDate } from "@bufbuild/protobuf/wkt";
import { Calendar, MoreHorizontal, NotebookPen, Tag, Trash2, X } from "lucide-react";

import {
  useOneOffItems,
  useCreateOneOffItem,
  useUpdateOneOffItemNotes,
  useUpdateOneOffItemDueDate,
  useUpdateOneOffItemArea,
  useUpdateOneOffItemStatus,
  useDeleteOneOffItem,
} from "@/lib/oneOffItems";
import { Area, OneOffItemStatus, type OneOffItem } from "@/gen/reminders/v1/one_off_item_pb";
import { AREA_OPTIONS, AREA_LABELS, AREA_TEXT_CLASSES, AREA_BORDER_CLASSES } from "@/lib/area";
import { extractArea, extractDueDate } from "@/lib/quickAdd";
import { formatChipDate, toDateInputValue, fromDateInputValue, isOverdue, startOfTodayUTC, addDays } from "@/lib/time";
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
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Popover, PopoverAnchor, PopoverContent } from "@/components/ui/popover";
import { TaskCheckbox } from "@/components/TaskCheckbox";
import { PageHeader } from "@/components/PageHeader";
import { AddTaskRow } from "@/components/AddTaskRow";

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
          className={cn("rounded-md", AREA_TEXT_CLASSES[value], AREA_BORDER_CLASSES[value])}
        >
          <Tag />@{AREA_LABELS[value]}
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent>
        <DropdownMenuRadioGroup
          value={String(value)}
          onValueChange={(v) => onChange(Number(v) as Area)}
        >
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

function DueDateChip({
  value,
  onChange,
  disabled,
}: {
  value?: Date;
  onChange: (date: Date | undefined) => void;
  disabled?: boolean;
}) {
  const overdue = value !== undefined && isOverdue(value);
  const isToday = value !== undefined && !overdue && value.getTime() === startOfTodayUTC().getTime();

  return (
    <div
      className={cn(
        "inline-flex items-center gap-0.5 rounded-md border py-0.5 pr-1 pl-1.5 text-sm",
        !value && "border-input text-muted-foreground",
        overdue && "border-destructive/40 text-destructive",
        isToday && "border-status-good/40 text-status-good",
        value && !overdue && !isToday && "border-primary/40 text-primary"
      )}
    >
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <button
            type="button"
            disabled={disabled}
            className="flex items-center gap-1 rounded-md px-1 py-0.5 outline-none hover:bg-foreground/5 disabled:pointer-events-none disabled:opacity-50"
          >
            <Calendar className="size-3.5" />
            {value ? formatChipDate(value) : "Due date"}
          </button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start" className="flex flex-col gap-1 p-2">
          <DropdownMenuItem onSelect={() => onChange(startOfTodayUTC())}>Today</DropdownMenuItem>
          <DropdownMenuItem onSelect={() => onChange(addDays(startOfTodayUTC(), 1))}>Tomorrow</DropdownMenuItem>
          <DropdownMenuSeparator />
          <input
            type="date"
            className="h-8 rounded-md border border-input bg-transparent px-2 text-sm text-foreground outline-none focus-visible:border-ring"
            value={value ? toDateInputValue(value) : ""}
            onChange={(e) => {
              const raw = e.target.value;
              onChange(raw ? fromDateInputValue(raw) : undefined);
            }}
          />
        </DropdownMenuContent>
      </DropdownMenu>
      {value && (
        <button
          type="button"
          aria-label="Clear due date"
          disabled={disabled}
          onClick={() => onChange(undefined)}
          className="rounded-md p-0.5 outline-none hover:bg-foreground/10 disabled:pointer-events-none disabled:opacity-50"
        >
          <X className="size-3" />
        </button>
      )}
    </div>
  );
}

function NotesEditor({ item }: { item: OneOffItem }) {
  const updateNotes = useUpdateOneOffItemNotes();
  const [notes, setNotes] = useState(item.notes);
  const dirty = notes !== item.notes;

  return (
    <div className="mt-2 flex flex-col gap-2 rounded-md bg-muted/30 p-2">
      <Textarea placeholder="Add notes…" value={notes} onChange={(e) => setNotes(e.target.value)} />
      <div className="flex items-center justify-end gap-2">
        {updateNotes.isError && <p className="text-sm text-destructive">{updateNotes.error.message}</p>}
        <Button size="sm" disabled={!dirty || updateNotes.isPending} onClick={() => updateNotes.mutate({ id: item.id, notes })}>
          Save notes
        </Button>
      </div>
    </div>
  );
}

function TaskRow({
  item,
  expanded,
  onToggleExpanded,
}: {
  item: OneOffItem;
  expanded: boolean;
  onToggleExpanded: () => void;
}) {
  const updateDueDate = useUpdateOneOffItemDueDate();
  const updateArea = useUpdateOneOffItemArea();
  const updateStatus = useUpdateOneOffItemStatus();
  const deleteItem = useDeleteOneOffItem();
  const dueDate = item.dueDate ? timestampDate(item.dueDate) : undefined;

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
        <div className="flex flex-1 flex-col gap-1.5">
          <div className="flex items-center justify-between gap-2">
            <span className="text-sm font-medium">{item.title}</span>
            <div className="flex items-center gap-0.5 opacity-0 transition-opacity group-hover:opacity-100">
              <Button
                variant="ghost"
                size="icon-sm"
                aria-label={`${expanded ? "Hide" : "Show"} notes for ${item.title}`}
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
            <DueDateChip
              value={dueDate}
              disabled={updateDueDate.isPending}
              onChange={(date) => updateDueDate.mutate({ id: item.id, dueDate: date ? timestampFromDate(date) : undefined })}
            />
            <AreaChip value={item.area} disabled={updateArea.isPending} onChange={(area) => updateArea.mutate({ id: item.id, area })} />
          </div>
          {deleteItem.isError && <p className="text-sm text-destructive">{deleteItem.error.message}</p>}
        </div>
      </div>
      {expanded && <NotesEditor item={item} />}
    </div>
  );
}

export function RemindersPage() {
  const { data: items, isLoading, isError, error } = useOneOffItems();
  const createItem = useCreateOneOffItem();
  const [isAdding, setIsAdding] = useState(false);
  const [title, setTitle] = useState("");
  const [notes, setNotes] = useState("");
  const [newDueDate, setNewDueDate] = useState<Date | undefined>(undefined);
  const [newArea, setNewArea] = useState<Area>(Area.PERSONAL);
  const [expanded, setExpanded] = useState<Set<string>>(new Set());
  const [titleFocused, setTitleFocused] = useState(false);
  const [notesFocused, setNotesFocused] = useState(false);

  // "@" inline autocomplete in the title field: atQuery is the in-progress
  // partial word after "@" (e.g. "wo" for "@wo"), atStart is the index of
  // the "@" character itself, so a commit can splice the full "@label "
  // back in at the right spot.
  const titleInputRef = useRef<HTMLInputElement>(null);
  const [atQuery, setAtQuery] = useState<string | null>(null);
  const [atStart, setAtStart] = useState(0);
  const [highlightedIndex, setHighlightedIndex] = useState(0);

  const atFilteredAreas =
    atQuery !== null
      ? AREA_OPTIONS.filter((a) => AREA_LABELS[a].startsWith(atQuery.toLowerCase()))
      : [];
  const autocompleteOpen = atQuery !== null && atFilteredAreas.length > 0;

  const handleTitleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const value = e.target.value;
    const caret = e.target.selectionStart ?? value.length;
    setTitle(value);

    const match = /(?:^|\s)@(\w*)$/.exec(value.slice(0, caret));
    if (match) {
      setAtQuery(match[1]);
      setAtStart(caret - match[1].length - 1);
      setHighlightedIndex(0);
    } else {
      setAtQuery(null);
    }
  };

  const commitAreaToken = (area: Area) => {
    if (atQuery === null) return;
    const before = title.slice(0, atStart);
    const after = title.slice(atStart + 1 + atQuery.length);
    const insertion = `@${AREA_LABELS[area]} `;
    setTitle(before + insertion + after);
    setAtQuery(null);
    requestAnimationFrame(() => {
      const pos = before.length + insertion.length;
      titleInputRef.current?.setSelectionRange(pos, pos);
      titleInputRef.current?.focus();
    });
  };

  const handleTitleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (autocompleteOpen) {
      if (e.key === "ArrowDown") {
        e.preventDefault();
        setHighlightedIndex((i) => (i + 1) % atFilteredAreas.length);
        return;
      }
      if (e.key === "ArrowUp") {
        e.preventDefault();
        setHighlightedIndex((i) => (i - 1 + atFilteredAreas.length) % atFilteredAreas.length);
        return;
      }
      if (e.key === "Enter" || e.key === "Tab") {
        e.preventDefault();
        commitAreaToken(atFilteredAreas[highlightedIndex]);
        return;
      }
      if (e.key === "Escape") {
        e.preventDefault();
        setAtQuery(null);
        return;
      }
    }
    if (e.key === "Enter") handleCreate();
  };

  const toggleExpanded = (id: string) => {
    setExpanded((prev) => {
      const next = new Set(prev);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });
  };

  // Todoist-style quick entry: typing "@work" or "friday" anywhere in the
  // title updates the area/due-date chips live and is stripped from the
  // title on submit. Clicking a chip directly afterwards overrides it,
  // since these effects only re-fire when the typed token itself changes.
  const areaToken = extractArea(title);
  const dueDateToken = extractDueDate(areaToken ? areaToken.rest : title);

  useEffect(() => {
    if (areaToken) setNewArea(areaToken.area);
  }, [areaToken?.area]);

  const dueDateTokenMs = dueDateToken?.date.getTime();
  useEffect(() => {
    if (dueDateToken) setNewDueDate(dueDateToken.date);
  }, [dueDateTokenMs]);

  const strippedTitle = (dueDateToken ?? areaToken ?? { rest: title }).rest;
  const finalTitle = strippedTitle.trim();

  const resetForm = () => {
    setTitle("");
    setNotes("");
    setNewDueDate(undefined);
    setNewArea(Area.PERSONAL);
  };

  const handleCreate = () => {
    if (!finalTitle) return;

    createItem.mutate(
      {
        title: finalTitle,
        notes: notes.trim() || undefined,
        area: newArea,
        dueDate: newDueDate ? timestampFromDate(newDueDate) : undefined,
      },
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

  return (
    <div className="mx-auto flex w-full max-w-2xl flex-1 flex-col gap-4 p-4">
      <PageHeader title="Today" count={items?.length ?? 0} />

      {isAdding ? (
        <div className="flex flex-col gap-2 rounded-lg border border-input">
          <div className="flex flex-col gap-1 p-3">
            <Popover
              open={autocompleteOpen}
              onOpenChange={(open) => {
                if (!open) setAtQuery(null);
              }}
            >
              <PopoverAnchor asChild>
                <Input
                  ref={titleInputRef}
                  autoFocus
                  placeholder={titleFocused ? "" : "Task name - try @work or friday"}
                  className="h-auto border-0 bg-transparent p-0.5 text-base shadow-none focus-visible:ring-0 dark:bg-transparent"
                  value={title}
                  onChange={handleTitleChange}
                  onKeyDown={handleTitleKeyDown}
                  onFocus={() => setTitleFocused(true)}
                  onBlur={() => setTitleFocused(false)}
                />
              </PopoverAnchor>
              <PopoverContent
                align="start"
                className="w-auto min-w-40 p-1"
                onOpenAutoFocus={(e) => e.preventDefault()}
              >
                {atFilteredAreas.map((area, i) => (
                  <button
                    key={area}
                    type="button"
                    onMouseDown={(e) => e.preventDefault()}
                    onClick={() => commitAreaToken(area)}
                    className={cn(
                      "flex w-full items-center gap-1.5 rounded-md px-2 py-1.5 text-left text-sm",
                      AREA_TEXT_CLASSES[area],
                      i === highlightedIndex ? "bg-accent" : "hover:bg-accent/50"
                    )}
                  >
                    <Tag className="size-3.5" />@{AREA_LABELS[area]}
                  </button>
                ))}
              </PopoverContent>
            </Popover>
            <Textarea
              placeholder={notesFocused ? "" : "Notes"}
              rows={1}
              className="min-h-0 resize-none border-0 bg-transparent p-0.5 text-sm shadow-none focus-visible:ring-0 dark:bg-transparent"
              value={notes}
              onChange={(e) => setNotes(e.target.value)}
              onFocus={() => setNotesFocused(true)}
              onBlur={() => setNotesFocused(false)}
            />
            <div className="mt-1 flex flex-wrap items-center gap-2">
              <DueDateChip value={newDueDate} onChange={setNewDueDate} />
              <AreaChip value={newArea} onChange={setNewArea} />
            </div>
          </div>
          <div className="flex items-center justify-end gap-2 border-t border-input px-3 py-2">
            <Button variant="secondary" onClick={handleCancel}>
              Cancel
            </Button>
            <Button onClick={handleCreate} disabled={createItem.isPending || !finalTitle}>
              Add task
            </Button>
          </div>
        </div>
      ) : (
        <AddTaskRow onClick={() => setIsAdding(true)} />
      )}

      {isLoading && <p className="text-sm text-muted-foreground">Loading reminders…</p>}
      {isError && <p className="text-sm text-destructive">{error.message}</p>}

      {items && items.length > 0 && (
        <div className="flex flex-col">
          {items.map((item) => (
            <TaskRow key={item.id} item={item} expanded={expanded.has(item.id)} onToggleExpanded={() => toggleExpanded(item.id)} />
          ))}
        </div>
      )}
      {!isLoading && items && items.length === 0 && (
        <p className="text-sm text-muted-foreground">No reminders yet.</p>
      )}
      {createItem.isError && (
        <p className="text-sm text-destructive">{createItem.error.message}</p>
      )}
    </div>
  );
}
