import { useState, type FormEvent } from "react";

import { useCreateCycle, useDuplicateCycle, useCycles } from "@/lib/cycles";
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
import { Select } from "@/components/ui/select";

const BLANK = "";

interface AddCycleDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  userId: string;
  onCreated?: (cycleId: string) => void;
}

export function AddCycleDialog({ open, onOpenChange, userId, onCreated }: AddCycleDialogProps) {
  const [name, setName] = useState("");
  const [sourceCycleId, setSourceCycleId] = useState(BLANK);
  const { data: cycles } = useCycles(userId);
  const createCycle = useCreateCycle();
  const duplicateCycle = useDuplicateCycle();
  const isPending = createCycle.isPending || duplicateCycle.isPending;
  const error = createCycle.error ?? duplicateCycle.error;

  function reset() {
    setName("");
    setSourceCycleId(BLANK);
  }

  function handleSourceChange(id: string) {
    setSourceCycleId(id);
    if (id && !name.trim()) {
      const source = cycles?.find((c) => c.id === id);
      if (source) setName(`${source.name} copy`);
    }
  }

  function handleSubmit(e: FormEvent) {
    e.preventDefault();
    if (!name.trim()) return;

    if (sourceCycleId) {
      duplicateCycle.mutate(
        { sourceCycleId, name: name.trim() },
        {
          onSuccess: (cycle) => {
            reset();
            onOpenChange(false);
            onCreated?.(cycle.id);
          },
        },
      );
      return;
    }

    createCycle.mutate(
      { userId, name: name.trim() },
      {
        onSuccess: (cycle) => {
          reset();
          onOpenChange(false);
          onCreated?.(cycle.id);
        },
      },
    );
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>New cycle</DialogTitle>
          <DialogDescription>A training program made up of blocks (e.g. weeks).</DialogDescription>
        </DialogHeader>

        <form onSubmit={handleSubmit} className="grid gap-4">
          {cycles && cycles.length > 0 && (
            <div className="grid gap-1.5">
              <Label htmlFor="duplicate-from">Duplicate from (optional)</Label>
              <Select
                id="duplicate-from"
                value={sourceCycleId}
                onChange={(e) => handleSourceChange(e.target.value)}
                disabled={isPending}
              >
                <option value={BLANK}>Start blank</option>
                {cycles.map((cycle) => (
                  <option key={cycle.id} value={cycle.id}>
                    {cycle.name}
                  </option>
                ))}
              </Select>
            </div>
          )}

          <div className="grid gap-1.5">
            <Label htmlFor="new-cycle-name">Name</Label>
            <Input
              id="new-cycle-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="Name"
              disabled={isPending}
            />
          </div>

          {error && <p className="text-sm text-destructive">{error.message}</p>}

          <DialogFooter>
            <Button type="submit" disabled={isPending || !name.trim()}>
              {sourceCycleId ? "Duplicate" : "Add"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
