import { useState, type FormEvent } from "react";
import { Check, ChevronUp, ChevronDown, Trash2 } from "lucide-react";

import { MoveDirection, type ExerciseSet } from "@/gen/workouts/v1/cycle_pb";
import { WeightUnit, type TrainingMax } from "@/gen/workouts/v1/training_max_pb";
import { useUpdateSet, useRemoveSet, useMoveSet } from "@/lib/cycles";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";

interface SetCellProps {
  set: ExerciseSet;
  isFirst: boolean;
  isLast: boolean;
  currentTrainingMax?: TrainingMax;
}

// One prescribed set, inline-editable - reps + optional % of training max,
// with move-up/down (within its cell) and remove. Local state is seeded
// from `set` via useState's initializer, same pattern as TrainingMaxCell;
// the parent (CycleDetailPage) only renders this once useCycle's isLoading
// has resolved, so `set` is never stale/undefined on mount.
export function SetCell({ set, isFirst, isLast, currentTrainingMax }: SetCellProps) {
  const [reps, setReps] = useState(String(set.reps));
  const [percentage, setPercentage] = useState(
    set.percentageOfTm !== undefined ? String(set.percentageOfTm) : "",
  );
  const updateSet = useUpdateSet();
  const removeSet = useRemoveSet();
  const moveSet = useMoveSet();

  const parsedReps = Number(reps);
  const repsValid = reps.trim() !== "" && !Number.isNaN(parsedReps) && parsedReps > 0;
  const parsedPercentage = percentage.trim() === "" ? undefined : Number(percentage);
  const percentageValid = parsedPercentage === undefined || (!Number.isNaN(parsedPercentage) && parsedPercentage > 0);

  function handleSubmit(e: FormEvent) {
    e.preventDefault();
    if (!repsValid || !percentageValid) return;
    updateSet.mutate({ id: set.id, reps: parsedReps, percentageOfTm: parsedPercentage });
  }

  const computedWeight =
    parsedPercentage !== undefined && currentTrainingMax
      ? (parsedPercentage / 100) * currentTrainingMax.weight
      : undefined;

  return (
    <form onSubmit={handleSubmit} className="flex flex-col gap-1">
      <div className="flex items-center gap-1.5">
        <Input
          type="number"
          min="1"
          step="1"
          value={reps}
          onChange={(e) => setReps(e.target.value)}
          aria-label="Reps"
          className="h-7 w-14"
          disabled={updateSet.isPending}
        />
        <span className="text-xs text-muted-foreground">reps @</span>
        <Input
          type="number"
          min="0"
          step="1"
          value={percentage}
          onChange={(e) => setPercentage(e.target.value)}
          placeholder="—"
          aria-label="Percent of training max"
          className="h-7 w-16"
          disabled={updateSet.isPending}
        />
        <span className="text-xs text-muted-foreground">%</span>
        {computedWeight !== undefined && (
          <span className="text-xs text-muted-foreground">
            ({computedWeight.toFixed(1)} {currentTrainingMax?.unit === WeightUnit.KG ? "kg" : "lb"})
          </span>
        )}
        <Button
          type="submit"
          variant="ghost"
          size="icon-sm"
          aria-label="Save set"
          disabled={!repsValid || !percentageValid || updateSet.isPending}
        >
          <Check />
        </Button>
        <Button
          type="button"
          variant="ghost"
          size="icon-sm"
          aria-label="Move set up"
          disabled={isFirst || moveSet.isPending}
          onClick={() => moveSet.mutate({ id: set.id, direction: MoveDirection.UP })}
        >
          <ChevronUp />
        </Button>
        <Button
          type="button"
          variant="ghost"
          size="icon-sm"
          aria-label="Move set down"
          disabled={isLast || moveSet.isPending}
          onClick={() => moveSet.mutate({ id: set.id, direction: MoveDirection.DOWN })}
        >
          <ChevronDown />
        </Button>
        <Button
          type="button"
          variant="ghost"
          size="icon-sm"
          aria-label="Remove set"
          disabled={removeSet.isPending}
          onClick={() => removeSet.mutate(set.id)}
        >
          <Trash2 />
        </Button>
      </div>
      {updateSet.isError && <span className="text-xs text-destructive">{updateSet.error.message}</span>}
    </form>
  );
}
