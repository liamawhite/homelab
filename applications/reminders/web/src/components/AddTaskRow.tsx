import { Plus } from "lucide-react";

// Collapsed entry point for the create-task form, Todoist-style - a plain
// row rather than an always-open form, so the list reads as a list first.
export function AddTaskRow({ onClick }: { onClick: () => void }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="flex items-center gap-2.5 rounded-md px-1 py-2 text-left text-sm text-muted-foreground hover:text-foreground"
    >
      <Plus className="size-4 text-destructive" />
      Add task
    </button>
  );
}
