import { useState } from "react";
import { Link } from "@tanstack/react-router";
import { timestampDate } from "@bufbuild/protobuf/wkt";
import { Plus } from "lucide-react";

import { WeightUnit } from "@/gen/workouts/v1/training_max_pb";
import { useActiveUser } from "@/lib/activeUser";
import { useExercises } from "@/lib/exercises";
import { useCurrentTrainingMaxes } from "@/lib/trainingMaxes";
import { relativeTime } from "@/lib/time";
import { RecordTrainingMaxDialog } from "@/components/RecordTrainingMaxDialog";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";

export function TrainingMaxesPage() {
  const [activeUserId] = useActiveUser();
  const { data: exercises } = useExercises();
  const { data: trainingMaxes, isLoading, isError, error } = useCurrentTrainingMaxes(activeUserId);
  const [dialogExerciseId, setDialogExerciseId] = useState<string | undefined>(undefined);
  const [dialogOpen, setDialogOpen] = useState(false);

  const activeExercises = exercises?.filter((e) => !e.archived) ?? [];

  function openDialog(exerciseId?: string) {
    setDialogExerciseId(exerciseId);
    setDialogOpen(true);
  }

  if (!activeUserId) {
    return (
      <div className="flex flex-1 items-center justify-center p-4">
        <p className="text-sm text-muted-foreground">
          Select or create a{" "}
          <Link to="/users" className="underline underline-offset-4">
            user
          </Link>{" "}
          to track training maxes.
        </p>
      </div>
    );
  }

  return (
    <div className="mx-auto flex w-full max-w-md flex-1 flex-col justify-center gap-4 p-4">
      <Card>
        <CardHeader className="flex flex-row items-center justify-between gap-2">
          <CardTitle>Training maxes</CardTitle>
          <Button
            variant="outline"
            size="sm"
            className="gap-1"
            disabled={activeExercises.length === 0}
            onClick={() => openDialog(undefined)}
          >
            <Plus />
            Record
          </Button>
        </CardHeader>
        <CardContent className="grid gap-3">
          {isLoading && <p className="text-sm text-muted-foreground">Loading…</p>}
          {isError && <p className="text-sm text-destructive">{error.message}</p>}

          {trainingMaxes && trainingMaxes.length === 0 && (
            <p className="text-sm text-muted-foreground">
              No training maxes recorded yet.{" "}
              {activeExercises.length === 0 && (
                <>
                  Add an exercise on the{" "}
                  <Link to="/exercises" className="underline underline-offset-4">
                    exercises
                  </Link>{" "}
                  page first.
                </>
              )}
            </p>
          )}

          {trainingMaxes?.map((max) => (
            <div key={max.exerciseId} className="flex items-center justify-between gap-2 text-sm">
              <div>
                <div className="font-medium">{max.exerciseName}</div>
                <div className="text-xs text-muted-foreground">
                  {max.weight} {max.unit === WeightUnit.KG ? "kg" : "lb"}
                  {max.effectiveAt && <> · {relativeTime(timestampDate(max.effectiveAt))}</>}
                </div>
              </div>
              <Button variant="ghost" size="sm" onClick={() => openDialog(max.exerciseId)}>
                Record new
              </Button>
            </div>
          ))}
        </CardContent>
      </Card>

      <RecordTrainingMaxDialog
        open={dialogOpen}
        onOpenChange={setDialogOpen}
        userId={activeUserId}
        exercises={activeExercises}
        initialExerciseId={dialogExerciseId}
      />
    </div>
  );
}
