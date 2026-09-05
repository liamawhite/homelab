import { useRef, useState, useEffect } from "react";
import { Calendar, Tag, X } from "lucide-react";

import { Area } from "@/gen/reminders/v1/one_off_item_pb";
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

export interface TaskEditorValues {
  title: string;
  notes: string;
  dueDate?: Date;
  area: Area;
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

// The task creation/editing form - shared by RemindersPage's "Add task" flow
// and TaskRow's click-to-edit flow, so both offer identical fields (title
// with Todoist-style "@area"/"friday" quick-add tokens, notes, due date,
// area) and the same Save/Cancel affordances.
export function TaskEditor({
  initial,
  submitLabel,
  onSubmit,
  onCancel,
  pending,
  error,
}: {
  initial: TaskEditorValues;
  submitLabel: string;
  onSubmit: (values: TaskEditorValues) => void;
  onCancel: () => void;
  pending?: boolean;
  error?: Error | null;
}) {
  const [title, setTitle] = useState(initial.title);
  const [notes, setNotes] = useState(initial.notes);
  const [dueDate, setDueDate] = useState<Date | undefined>(initial.dueDate);
  const [area, setArea] = useState<Area>(initial.area);
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

  const commitAreaToken = (a: Area) => {
    if (atQuery === null) return;
    const before = title.slice(0, atStart);
    const after = title.slice(atStart + 1 + atQuery.length);
    const insertion = `@${AREA_LABELS[a]} `;
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
    if (e.key === "Enter") handleSubmit();
  };

  // Todoist-style quick entry: typing "@work" or "friday" anywhere in the
  // title updates the area/due-date chips live. Clicking a chip directly
  // afterwards overrides it, since these effects only re-fire when the
  // typed token itself changes.
  const areaToken = extractArea(title);
  const dueDateToken = extractDueDate(areaToken ? areaToken.rest : title);

  useEffect(() => {
    if (areaToken) setArea(areaToken.area);
  }, [areaToken?.area]);

  // Once a due date is recognized, remove the matched text from the title
  // right away (rather than waiting for submit) so the chip and the text
  // don't both claim to represent it - dueDateToken.rest already reflects
  // the area token being stripped too, if one was also just matched.
  const dueDateTokenMs = dueDateToken?.date.getTime();
  useEffect(() => {
    if (dueDateToken) {
      setDueDate(dueDateToken.date);
      setTitle(dueDateToken.rest);
      requestAnimationFrame(() => {
        const pos = dueDateToken.rest.length;
        titleInputRef.current?.setSelectionRange(pos, pos);
      });
    }
  }, [dueDateTokenMs]);

  const finalTitle = (dueDateToken ?? areaToken ?? { rest: title }).rest.trim();

  const handleSubmit = () => {
    if (!finalTitle) return;
    onSubmit({ title: finalTitle, notes: notes.trim(), dueDate, area });
  };

  return (
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
            {atFilteredAreas.map((a, i) => (
              <button
                key={a}
                type="button"
                onMouseDown={(e) => e.preventDefault()}
                onClick={() => commitAreaToken(a)}
                className={cn(
                  "flex w-full items-center gap-1.5 rounded-md px-2 py-1.5 text-left text-sm",
                  AREA_TEXT_CLASSES[a],
                  i === highlightedIndex ? "bg-accent" : "hover:bg-accent/50"
                )}
              >
                <Tag className="size-3.5" />@{AREA_LABELS[a]}
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
          <DueDateChip value={dueDate} onChange={setDueDate} />
          <AreaChip value={area} onChange={setArea} />
        </div>
      </div>
      <div className="flex items-center justify-end gap-2 border-t border-input px-3 py-2">
        {error && <p className="mr-auto text-sm text-destructive">{error.message}</p>}
        <Button variant="secondary" onClick={onCancel}>
          Cancel
        </Button>
        <Button onClick={handleSubmit} disabled={pending || !finalTitle}>
          {submitLabel}
        </Button>
      </div>
    </div>
  );
}
