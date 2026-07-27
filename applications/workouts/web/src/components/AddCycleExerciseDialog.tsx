import { useMemo } from "react";

import { ExerciseCategory } from "@/gen/workouts/v1/exercise_pb";
import { useExercises } from "@/lib/exercises";
import { useAddCycleExercise } from "@/lib/cycles";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";

interface AddCycleExerciseDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  cycleId: string;
  existingExerciseIds: Set<string>;
}

export function AddCycleExerciseDialog({
  open,
  onOpenChange,
  cycleId,
  existingExerciseIds,
}: AddCycleExerciseDialogProps) {
  const { data: exercises, isLoading, isError, error } = useExercises();
  const addCycleExercise = useAddCycleExercise();

  const available = useMemo(
    () => (exercises ?? []).filter((e) => !e.archived && !existingExerciseIds.has(e.id)),
    [exercises, existingExerciseIds],
  );
  const mainLifts = available.filter((e) => e.category === ExerciseCategory.MAIN_LIFT);
  const accessories = available.filter((e) => e.category === ExerciseCategory.ACCESSORY);

  function add(exerciseId: string) {
    addCycleExercise.mutate({ cycleId, exerciseId }, { onSuccess: () => onOpenChange(false) });
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Add exercise</DialogTitle>
          <DialogDescription>
            Added to this cycle's shared lineup - available in every block.
          </DialogDescription>
        </DialogHeader>

        <div className="grid gap-4">
          {isLoading && <p className="text-sm text-muted-foreground">Loading exercises…</p>}
          {isError && <p className="text-sm text-destructive">{error.message}</p>}
          {addCycleExercise.isError && (
            <p className="text-sm text-destructive">{addCycleExercise.error.message}</p>
          )}

          {mainLifts.length > 0 && (
            <div className="grid gap-1">
              <span className="text-xs font-medium text-muted-foreground">Main lifts</span>
              {mainLifts.map((exercise) => (
                <Button
                  key={exercise.id}
                  type="button"
                  variant="ghost"
                  className="justify-start"
                  disabled={addCycleExercise.isPending}
                  onClick={() => add(exercise.id)}
                >
                  {exercise.name}
                </Button>
              ))}
            </div>
          )}

          {accessories.length > 0 && (
            <div className="grid gap-1">
              <span className="text-xs font-medium text-muted-foreground">Accessory</span>
              {accessories.map((exercise) => (
                <Button
                  key={exercise.id}
                  type="button"
                  variant="ghost"
                  className="justify-start"
                  disabled={addCycleExercise.isPending}
                  onClick={() => add(exercise.id)}
                >
                  {exercise.name}
                </Button>
              ))}
            </div>
          )}

          {!isLoading && available.length === 0 && (
            <p className="text-sm text-muted-foreground">
              No more exercises to add - every active exercise is already in this cycle.
            </p>
          )}
        </div>
      </DialogContent>
    </Dialog>
  );
}
