import { useState, type FormEvent } from "react";
import { ArchiveRestore, Archive as ArchiveIcon } from "lucide-react";

import {
  useLabels,
  useCreateLabel,
  useArchiveLabel,
  useRestoreLabel,
  useUpdateLabelColor,
} from "@/lib/labels";
import { LABEL_PALETTE } from "@/lib/color";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label as FormLabel } from "@/components/ui/label";
import { cn } from "@/lib/utils";

interface ManageLabelsDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function ManageLabelsDialog({ open, onOpenChange }: ManageLabelsDialogProps) {
  const { data: labels, isLoading, isError, error } = useLabels();
  const [name, setName] = useState("");
  const [editingColorFor, setEditingColorFor] = useState<string | null>(null);
  const createLabel = useCreateLabel();
  const archiveLabel = useArchiveLabel();
  const restoreLabel = useRestoreLabel();
  const updateColor = useUpdateLabelColor();

  const active = labels?.filter((l) => !l.archived) ?? [];
  const archived = labels?.filter((l) => l.archived) ?? [];

  function handleSubmit(e: FormEvent) {
    e.preventDefault();
    if (!name.trim()) return;
    createLabel.mutate(name.trim(), { onSuccess: () => setName("") });
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Manage labels</DialogTitle>
          <DialogDescription>
            Labels control which list an item belongs to. Archiving a label hides it from
            suggestions without deleting its items.
          </DialogDescription>
        </DialogHeader>

        <div className="grid gap-4">
          {isLoading && <p className="text-sm text-muted-foreground">Loading labels…</p>}
          {isError && <p className="text-sm text-destructive">{error.message}</p>}

          {active.length > 0 && (
            <div className="grid gap-1">
              {active.map((label) => (
                <div key={label.id} className="grid gap-1.5">
                  <div className="flex items-center justify-between gap-2 text-sm">
                    <button
                      type="button"
                      className="flex items-center gap-2"
                      aria-label={`Change ${label.name}'s color`}
                      onClick={() =>
                        setEditingColorFor((current) => (current === label.id ? null : label.id))
                      }
                    >
                      <span
                        className="size-3 shrink-0 rounded-full"
                        style={{ backgroundColor: label.color }}
                      />
                      {label.name}
                    </button>
                    <Button
                      variant="ghost"
                      size="icon-sm"
                      aria-label={`Archive ${label.name}`}
                      disabled={archiveLabel.isPending}
                      onClick={() => archiveLabel.mutate(label.id)}
                    >
                      <ArchiveIcon />
                    </Button>
                  </div>
                  {editingColorFor === label.id && (
                    <div className="flex flex-wrap gap-1.5 pb-1">
                      {LABEL_PALETTE.map((color) => (
                        <button
                          key={color}
                          type="button"
                          aria-label={`Set color ${color}`}
                          disabled={updateColor.isPending}
                          onClick={() => {
                            updateColor.mutate({ id: label.id, color });
                            setEditingColorFor(null);
                          }}
                          className={cn(
                            "size-5 rounded-full ring-1 ring-foreground/10",
                            color === label.color && "ring-2 ring-foreground/60",
                          )}
                          style={{ backgroundColor: color }}
                        />
                      ))}
                    </div>
                  )}
                </div>
              ))}
              {archiveLabel.isError && (
                <p className="text-sm text-destructive">{archiveLabel.error.message}</p>
              )}
              {updateColor.isError && (
                <p className="text-sm text-destructive">{updateColor.error.message}</p>
              )}
            </div>
          )}

          {archived.length > 0 && (
            <div className="grid gap-1 border-t pt-3">
              <span className="text-xs font-medium text-muted-foreground">Archived</span>
              {archived.map((label) => (
                <div
                  key={label.id}
                  className="flex items-center justify-between gap-2 text-sm text-muted-foreground"
                >
                  <span className="flex items-center gap-2">
                    <span
                      className="size-3 shrink-0 rounded-full opacity-60"
                      style={{ backgroundColor: label.color }}
                    />
                    {label.name}
                  </span>
                  <Button
                    variant="ghost"
                    size="icon-sm"
                    aria-label={`Restore ${label.name}`}
                    disabled={restoreLabel.isPending}
                    onClick={() => restoreLabel.mutate(label.id)}
                  >
                    <ArchiveRestore />
                  </Button>
                </div>
              ))}
            </div>
          )}

          <form onSubmit={handleSubmit} className="grid gap-1.5 border-t pt-3">
            <FormLabel htmlFor="new-label-name">New label</FormLabel>
            <div className="flex gap-2">
              <Input
                id="new-label-name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="Name"
                disabled={createLabel.isPending}
              />
              <Button type="submit" disabled={createLabel.isPending || !name.trim()}>
                Add
              </Button>
            </div>
            {createLabel.isError && (
              <p className="text-sm text-destructive">{createLabel.error.message}</p>
            )}
          </form>
        </div>
      </DialogContent>
    </Dialog>
  );
}
