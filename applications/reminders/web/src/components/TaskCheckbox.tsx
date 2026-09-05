import { Check } from "lucide-react";

import { cn } from "@/lib/utils";

// A Todoist-style round checkbox: an outlined ring in the task's area
// color, filled solid with a checkmark once checked; hovering an unchecked
// box previews the checkmark in the ring color so it's clear what clicking
// does.
export function TaskCheckbox({
  checked = false,
  colorClassName,
  disabled,
  onCheck,
  "aria-label": ariaLabel,
}: {
  checked?: boolean;
  colorClassName: string;
  disabled?: boolean;
  onCheck: () => void;
  "aria-label": string;
}) {
  return (
    <button
      type="button"
      role="checkbox"
      aria-checked={checked}
      aria-label={ariaLabel}
      disabled={disabled}
      onClick={onCheck}
      className={cn(
        "group/checkbox mt-0.5 flex size-[18px] shrink-0 items-center justify-center rounded-full border-2 border-current outline-none transition-colors disabled:pointer-events-none disabled:opacity-50",
        colorClassName,
        checked && "bg-current"
      )}
    >
      <Check
        className={cn(
          "size-3 stroke-[3] transition-opacity",
          checked ? "text-background opacity-100" : "text-current opacity-0 group-hover/checkbox:opacity-70"
        )}
      />
    </button>
  );
}
