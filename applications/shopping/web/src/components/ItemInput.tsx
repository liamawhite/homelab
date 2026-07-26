import { useRef, useState, type KeyboardEvent } from "react";

import type { Label } from "@/gen/shopping/v1/label_pb";
import { useCreateItem } from "@/lib/items";
import { badgeVariants } from "@/components/ui/badge";
import { contrastTextColor } from "@/lib/color";
import { cn } from "@/lib/utils";

const MAX_SUGGESTIONS = 8;
const CHIP_ATTR = "data-chip-label-id";
const CHIP_NAME_ATTR = "data-chip-label-name";

interface ItemInputProps {
  labels: Label[];
  onManageLabels: () => void;
  defaultLabelId?: string;
}

interface TokenMatch {
  textNode: Text;
  start: number;
  end: number;
  query: string;
}

// A single content-editable line that both names a new item and, via
// "@LabelName", picks which list it goes on - modeled on Todoist's quick-add:
// Enter adds and clears immediately, no separate submit button. Unlike a
// plain <input>, the chosen label renders as a real inline chip element (an
// atomic, non-editable span), not plain "@Groceries " text - that requires
// hand-rolling the caret/selection logic below rather than tracking a
// string, since contentEditable's DOM is the source of truth, not a value
// prop.
export function ItemInput({ labels, onManageLabels, defaultLabelId }: ItemInputProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const [query, setQuery] = useState<{ text: string } | null>(null);
  const tokenRef = useRef<TokenMatch | null>(null);
  const [highlightIndex, setHighlightIndex] = useState(0);
  const [needsLabel, setNeedsLabel] = useState(false);
  const createItem = useCreateItem();

  const suggestions = query
    ? labels
        .filter((l) => l.name.toLowerCase().includes(query.text.toLowerCase()))
        .slice(0, MAX_SUGGESTIONS)
    : [];

  function findActiveToken(): TokenMatch | null {
    const selection = window.getSelection();
    const container = containerRef.current;
    if (!selection || !container || selection.rangeCount === 0) return null;

    const range = selection.getRangeAt(0);
    if (!range.collapsed || range.startContainer.nodeType !== Node.TEXT_NODE) return null;
    if (!container.contains(range.startContainer)) return null;

    const textNode = range.startContainer as Text;
    const offset = range.startOffset;

    let start = -1;
    for (let i = offset - 1; i >= 0; i--) {
      const ch = textNode.data[i];
      if (ch === "@") {
        start = i;
        break;
      }
      if (/\s/.test(ch)) break;
    }
    if (start === -1) return null;

    return { textNode, start, end: offset, query: textNode.data.slice(start + 1, offset) };
  }

  function handleInput() {
    const token = findActiveToken();
    tokenRef.current = token;
    setQuery(token ? { text: token.query } : null);
    setHighlightIndex(0);
    setNeedsLabel(false);
  }

  function chipLabelId(): string | null {
    return containerRef.current?.querySelector(`[${CHIP_ATTR}]`)?.getAttribute(CHIP_ATTR) ?? null;
  }

  function plainName(): string {
    const container = containerRef.current;
    if (!container) return "";
    let text = "";
    for (const node of Array.from(container.childNodes)) {
      if (node.nodeType === Node.TEXT_NODE) text += node.textContent ?? "";
    }
    return text.replace(/\s+/g, " ").trim();
  }

  function buildChip(label: Label): HTMLSpanElement {
    const chip = document.createElement("span");
    chip.setAttribute(CHIP_ATTR, label.id);
    chip.setAttribute(CHIP_NAME_ATTR, label.name);
    chip.contentEditable = "false";
    chip.className = cn(badgeVariants(), "mx-0.5 align-baseline");
    chip.style.backgroundColor = label.color;
    chip.style.color = contrastTextColor(label.color);
    chip.textContent = label.name;
    return chip;
  }

  function selectSuggestion(label: Label) {
    const token = tokenRef.current;
    const container = containerRef.current;
    if (!token || !container) return;

    // Only one label per item - drop any chip already present before
    // inserting the new one.
    container.querySelector(`[${CHIP_ATTR}]`)?.remove();

    const { textNode, start, end } = token;
    const before = textNode.data.slice(0, start);
    const after = textNode.data.slice(end);

    const beforeNode = document.createTextNode(before);
    const chip = buildChip(label);
    const afterNode = document.createTextNode(" " + after);

    const parent = textNode.parentNode!;
    parent.replaceChild(afterNode, textNode);
    parent.insertBefore(chip, afterNode);
    parent.insertBefore(beforeNode, chip);

    const range = document.createRange();
    range.setStart(afterNode, 1);
    range.collapse(true);
    const selection = window.getSelection();
    selection?.removeAllRanges();
    selection?.addRange(range);

    tokenRef.current = null;
    setQuery(null);
    setNeedsLabel(false);
    container.focus();
  }

  function removeChipBeforeCaret(): boolean {
    const selection = window.getSelection();
    const container = containerRef.current;
    if (!selection || !container || selection.rangeCount === 0 || !selection.isCollapsed) return false;

    const range = selection.getRangeAt(0);
    let chip: Element | null = null;

    if (range.startContainer.nodeType === Node.TEXT_NODE && range.startOffset === 0) {
      const prev = (range.startContainer as Text).previousSibling;
      if (prev instanceof Element && prev.hasAttribute(CHIP_ATTR)) chip = prev;
    } else if (range.startContainer === container && range.startOffset > 0) {
      const prev = container.childNodes[range.startOffset - 1];
      if (prev instanceof Element && prev.hasAttribute(CHIP_ATTR)) chip = prev;
    }

    if (!chip) return false;
    chip.remove();
    return true;
  }

  function handleKeyDown(e: KeyboardEvent<HTMLDivElement>) {
    if (query && suggestions.length > 0) {
      if (e.key === "ArrowDown") {
        e.preventDefault();
        setHighlightIndex((i) => (i + 1) % suggestions.length);
        return;
      }
      if (e.key === "ArrowUp") {
        e.preventDefault();
        setHighlightIndex((i) => (i - 1 + suggestions.length) % suggestions.length);
        return;
      }
      if (e.key === "Enter" || e.key === "Tab") {
        e.preventDefault();
        selectSuggestion(suggestions[highlightIndex]);
        return;
      }
      if (e.key === "Escape") {
        e.preventDefault();
        setQuery(null);
        return;
      }
    }

    if (e.key === "Enter") {
      e.preventDefault();
      submit();
      return;
    }

    if (e.key === "Backspace" && removeChipBeforeCaret()) {
      e.preventDefault();
      handleInput();
    }
  }

  function submit() {
    const labelId = chipLabelId() ?? defaultLabelId ?? null;
    const name = plainName();

    if (!name) return;
    if (!labelId) {
      setNeedsLabel(true);
      return;
    }

    createItem.mutate(
      { name, labelId },
      {
        onSuccess: () => {
          if (containerRef.current) containerRef.current.textContent = "";
          setNeedsLabel(false);
        },
      },
    );
  }

  if (labels.length === 0) {
    return (
      <div className="grid gap-1.5 rounded-lg border border-dashed p-3 text-center text-sm text-muted-foreground">
        <span>Create a list before adding items.</span>
        <button
          type="button"
          onClick={onManageLabels}
          className="font-medium text-foreground underline underline-offset-4"
        >
          Manage Labels
        </button>
      </div>
    );
  }

  return (
    <div className="relative grid gap-1.5">
      <div
        ref={containerRef}
        contentEditable
        suppressContentEditableWarning
        onInput={handleInput}
        onKeyUp={(e) => {
          if (["ArrowLeft", "ArrowRight", "Home", "End"].includes(e.key)) handleInput();
        }}
        onClick={handleInput}
        onKeyDown={handleKeyDown}
        onBlur={() => setQuery(null)}
        data-placeholder={
          defaultLabelId ? "Add an item…" : "Add an item… type @ to pick a list"
        }
        className={cn(
          "h-8 w-full min-w-0 rounded-lg border border-input bg-transparent px-2.5 py-1 text-base leading-6 whitespace-pre-wrap outline-none transition-colors focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 md:text-sm",
          "empty:before:pointer-events-none empty:before:text-muted-foreground empty:before:content-[attr(data-placeholder)]",
        )}
      />
      {query && suggestions.length > 0 && (
        <div className="absolute top-full z-10 mt-1 w-full rounded-lg border bg-popover p-1 text-sm text-popover-foreground shadow-md">
          {suggestions.map((label, i) => (
            <button
              key={label.id}
              type="button"
              className={cn(
                "block w-full rounded-md px-2 py-1 text-left",
                i === highlightIndex && "bg-muted",
              )}
              onMouseDown={(e) => {
                e.preventDefault();
                selectSuggestion(label);
              }}
            >
              {label.name}
            </button>
          ))}
        </div>
      )}
      {needsLabel && (
        <p className="text-sm text-destructive">Pick a list with @ before adding.</p>
      )}
      {createItem.isError && (
        <p className="text-sm text-destructive">{createItem.error.message}</p>
      )}
    </div>
  );
}
