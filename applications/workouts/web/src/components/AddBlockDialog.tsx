import { useState, type FormEvent } from "react";

import { useCreateBlock } from "@/lib/cycles";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

interface AddBlockDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  cycleId: string;
  onCreated?: (blockId: string) => void;
}

export function AddBlockDialog({ open, onOpenChange, cycleId, onCreated }: AddBlockDialogProps) {
  const [name, setName] = useState("");
  const createBlock = useCreateBlock();

  function handleSubmit(e: FormEvent) {
    e.preventDefault();
    if (!name.trim()) return;
    createBlock.mutate(
      { cycleId, name: name.trim() },
      {
        onSuccess: (block) => {
          setName("");
          onOpenChange(false);
          onCreated?.(block.id);
        },
      },
    );
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>New block</DialogTitle>
          <DialogDescription>A time period within this cycle (e.g. "Week 1", "Deload").</DialogDescription>
        </DialogHeader>

        <form onSubmit={handleSubmit} className="grid gap-4">
          <div className="grid gap-1.5">
            <Label htmlFor="new-block-name">Name</Label>
            <Input
              id="new-block-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="Name"
              disabled={createBlock.isPending}
            />
          </div>

          {createBlock.isError && (
            <p className="text-sm text-destructive">{createBlock.error.message}</p>
          )}

          <DialogFooter>
            <Button type="submit" disabled={createBlock.isPending || !name.trim()}>
              Add
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
