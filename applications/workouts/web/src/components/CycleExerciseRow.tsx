import { useState } from "react";
import { ChevronUp, ChevronDown, ChevronRight, Trash2, Plus } from "lucide-react";

import { MoveDirection, type CycleExercise, type ExerciseSet } from "@/gen/workouts/v1/cycle_pb";
import type { TrainingMax } from "@/gen/workouts/v1/training_max_pb";
import { useRemoveCycleExercise, useMoveCycleExercise, useAddSet } from "@/lib/cycles";
import { SetCell } from "@/components/SetCell";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";

interface CycleExerciseRowProps {
  cycleExercise: CycleExercise;
  blockId?: string;
  sets: ExerciseSet[];
  isFirstInGroup: boolean;
  isLastInGroup: boolean;
  currentTrainingMax?: TrainingMax;
}

// One row in the cycle builder: an exercise's name, move-up/down within
// its category group (cycle-wide - moving here reorders the exercise for
// every block, not just the currently selected one), a remove button, and
// the selected block's prescribed sets plus an "Add set" affordance.
export function CycleExerciseRow({
  cycleExercise,
  blockId,
  sets,
  isFirstInGroup,
  isLastInGroup,
  currentTrainingMax,
}: CycleExerciseRowProps) {
  const [expanded, setExpanded] = useState(false);
  const removeCycleExercise = useRemoveCycleExercise();
  const moveCycleExercise = useMoveCycleExercise();
  const addSet = useAddSet();

  return (
    <div className="grid gap-2 border-b py-3 last:border-b-0">
      <div className="flex items-center justify-between gap-2">
        <button
          type="button"
          className="flex items-center gap-1.5 text-sm font-medium"
          aria-expanded={expanded}
          onClick={() => setExpanded((v) => !v)}
        >
          {expanded ? <ChevronDown className="size-4" /> : <ChevronRight className="size-4" />}
          {cycleExercise.exerciseName}
        </button>
        <div className="flex items-center gap-1">
          <Badge variant={sets.length > 0 ? "secondary" : "outline"}>
            {sets.length} {sets.length === 1 ? "set" : "sets"}
          </Badge>
          <Button
            variant="ghost"
            size="icon-sm"
            aria-label={`Move ${cycleExercise.exerciseName} up`}
            disabled={isFirstInGroup || moveCycleExercise.isPending}
            onClick={() =>
              moveCycleExercise.mutate({ id: cycleExercise.id, direction: MoveDirection.UP })
            }
          >
            <ChevronUp />
          </Button>
          <Button
            variant="ghost"
            size="icon-sm"
            aria-label={`Move ${cycleExercise.exerciseName} down`}
            disabled={isLastInGroup || moveCycleExercise.isPending}
            onClick={() =>
              moveCycleExercise.mutate({ id: cycleExercise.id, direction: MoveDirection.DOWN })
            }
          >
            <ChevronDown />
          </Button>
          <Button
            variant="ghost"
            size="icon-sm"
            aria-label={`Remove ${cycleExercise.exerciseName}`}
            disabled={removeCycleExercise.isPending}
            onClick={() => removeCycleExercise.mutate(cycleExercise.id)}
          >
            <Trash2 />
          </Button>
        </div>
      </div>

      {expanded && (
        <div className="grid gap-1 pl-1">
          {!blockId && (
            <span className="text-xs text-muted-foreground">Select or create a block to add sets.</span>
          )}
          {blockId && (
            <>
              {sets.map((set, index) => (
                <SetCell
                  key={set.id}
                  set={set}
                  isFirst={index === 0}
                  isLast={index === sets.length - 1}
                  currentTrainingMax={currentTrainingMax}
                />
              ))}

              <Button
                variant="ghost"
                size="sm"
                className="w-fit gap-1"
                disabled={addSet.isPending}
                onClick={() => addSet.mutate({ cycleExerciseId: cycleExercise.id, blockId, reps: 5 })}
              >
                <Plus />
                Add set
              </Button>
              {addSet.isError && <span className="text-xs text-destructive">{addSet.error.message}</span>}
            </>
          )}
        </div>
      )}
    </div>
  );
}
