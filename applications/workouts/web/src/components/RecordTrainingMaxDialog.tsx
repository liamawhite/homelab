import { useEffect, useState, type FormEvent } from "react";

import type { Exercise } from "@/gen/workouts/v1/exercise_pb";
import { WeightUnit } from "@/gen/workouts/v1/training_max_pb";
import { useRecordTrainingMax } from "@/lib/trainingMaxes";
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
import { cn } from "@/lib/utils";

interface RecordTrainingMaxDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  userId: string;
  exercises: Exercise[];
  initialExerciseId?: string;
}

export function RecordTrainingMaxDialog({
  open,
  onOpenChange,
  userId,
  exercises,
  initialExerciseId,
}: RecordTrainingMaxDialogProps) {
  const [exerciseId, setExerciseId] = useState(initialExerciseId ?? exercises[0]?.id ?? "");
  const [weight, setWeight] = useState("");
  const [unit, setUnit] = useState<WeightUnit>(WeightUnit.KG);
  const recordTrainingMax = useRecordTrainingMax();

  // Re-seed from props each time the dialog opens, rather than once at
  // mount, since the same dialog instance gets reused across "Record new"
  // clicks for different exercises.
  useEffect(() => {
    if (!open) return;
    setExerciseId(initialExerciseId ?? exercises[0]?.id ?? "");
    setWeight("");
    setUnit(WeightUnit.KG);
  }, [open, initialExerciseId, exercises]);

  function handleSubmit(e: FormEvent) {
    e.preventDefault();
    if (!exerciseId || weight.trim() === "") return;
    const parsed = Number(weight);
    // 0 is valid - it means bodyweight only, no added load.
    if (Number.isNaN(parsed) || parsed < 0) return;

    recordTrainingMax.mutate(
      { userId, exerciseId, weight: parsed, unit },
      { onSuccess: () => onOpenChange(false) },
    );
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Record training max</DialogTitle>
          <DialogDescription>
            Recording a new value keeps the previous one in history - it doesn&apos;t overwrite it.
          </DialogDescription>
        </DialogHeader>

        <form onSubmit={handleSubmit} className="grid gap-4">
          <div className="grid gap-1.5">
            <Label htmlFor="tm-exercise">Exercise</Label>
            <Select
              id="tm-exercise"
              value={exerciseId}
              onChange={(e) => setExerciseId(e.target.value)}
            >
              {exercises.length === 0 && <option value="">No exercises yet</option>}
              {exercises.map((exercise) => (
                <option key={exercise.id} value={exercise.id}>
                  {exercise.name}
                </option>
              ))}
            </Select>
          </div>

          <div className="grid gap-1.5">
            <Label htmlFor="tm-weight">Weight</Label>
            <div className="flex gap-2">
              <Input
                id="tm-weight"
                type="number"
                step="0.5"
                min="0"
                value={weight}
                onChange={(e) => setWeight(e.target.value)}
                placeholder="0"
              />
              {[WeightUnit.KG, WeightUnit.LB].map((option) => (
                <Button
                  key={option}
                  type="button"
                  variant="outline"
                  size="sm"
                  aria-expanded={unit === option}
                  className={cn("shrink-0", unit === option && "border-ring")}
                  onClick={() => setUnit(option)}
                >
                  {option === WeightUnit.KG ? "kg" : "lb"}
                </Button>
              ))}
            </div>
          </div>

          {recordTrainingMax.isError && (
            <p className="text-sm text-destructive">{recordTrainingMax.error.message}</p>
          )}

          <DialogFooter>
            <Button
              type="submit"
              disabled={
                recordTrainingMax.isPending ||
                !exerciseId ||
                weight.trim() === "" ||
                Number.isNaN(Number(weight)) ||
                Number(weight) < 0
              }
            >
              Record
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
