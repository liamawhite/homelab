import { useState, type FormEvent } from "react";
import { Link } from "@tanstack/react-router";
import { Archive as ArchiveIcon, ArchiveRestore } from "lucide-react";

import { ExerciseCategory } from "@/gen/workouts/v1/exercise_pb";
import { useExercises, useCreateExercise, useArchiveExercise, useRestoreExercise } from "@/lib/exercises";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { cn } from "@/lib/utils";

function categoryLabel(category: ExerciseCategory): string {
  return category === ExerciseCategory.MAIN_LIFT ? "Main lift" : "Accessory";
}

export function ExercisesPage() {
  const { data: exercises, isLoading, isError, error } = useExercises();
  const [name, setName] = useState("");
  const [category, setCategory] = useState<ExerciseCategory>(ExerciseCategory.MAIN_LIFT);
  const createExercise = useCreateExercise();
  const archiveExercise = useArchiveExercise();
  const restoreExercise = useRestoreExercise();

  const active = exercises?.filter((e) => !e.archived) ?? [];
  const archived = exercises?.filter((e) => e.archived) ?? [];

  function handleSubmit(e: FormEvent) {
    e.preventDefault();
    if (!name.trim()) return;
    createExercise.mutate({ name: name.trim(), category }, { onSuccess: () => setName("") });
  }

  return (
    <div className="mx-auto flex w-full max-w-md flex-1 flex-col justify-center gap-4 p-4">
      <Card>
        <CardHeader>
          <CardTitle>Manage exercises</CardTitle>
        </CardHeader>
        <CardContent className="grid gap-4">
          {isLoading && <p className="text-sm text-muted-foreground">Loading exercises…</p>}
          {isError && <p className="text-sm text-destructive">{error.message}</p>}

          {active.length > 0 && (
            <div className="grid gap-1">
              {active.map((exercise) => (
                <div key={exercise.id} className="flex items-center justify-between gap-2 text-sm">
                  <span>
                    {exercise.name}
                    <span className="ml-2 text-xs text-muted-foreground">
                      {categoryLabel(exercise.category)}
                    </span>
                  </span>
                  <Button
                    variant="ghost"
                    size="icon-sm"
                    aria-label={`Archive ${exercise.name}`}
                    disabled={archiveExercise.isPending}
                    onClick={() => archiveExercise.mutate(exercise.id)}
                  >
                    <ArchiveIcon />
                  </Button>
                </div>
              ))}
              {archiveExercise.isError && (
                <p className="text-sm text-destructive">{archiveExercise.error.message}</p>
              )}
            </div>
          )}
          {active.length === 0 && !isLoading && (
            <p className="text-sm text-muted-foreground">No exercises yet.</p>
          )}

          {archived.length > 0 && (
            <div className="grid gap-1 border-t pt-3">
              <span className="text-xs font-medium text-muted-foreground">Archived</span>
              {archived.map((exercise) => (
                <div
                  key={exercise.id}
                  className="flex items-center justify-between gap-2 text-sm text-muted-foreground"
                >
                  <span>{exercise.name}</span>
                  <Button
                    variant="ghost"
                    size="icon-sm"
                    aria-label={`Restore ${exercise.name}`}
                    disabled={restoreExercise.isPending}
                    onClick={() => restoreExercise.mutate(exercise.id)}
                  >
                    <ArchiveRestore />
                  </Button>
                </div>
              ))}
              {restoreExercise.isError && (
                <p className="text-sm text-destructive">{restoreExercise.error.message}</p>
              )}
            </div>
          )}

          <form onSubmit={handleSubmit} className="grid gap-1.5 border-t pt-3">
            <Label htmlFor="new-exercise-name">New exercise</Label>
            <div className="flex gap-2">
              <Input
                id="new-exercise-name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="Name"
                disabled={createExercise.isPending}
              />
              <Button type="submit" disabled={createExercise.isPending || !name.trim()}>
                Add
              </Button>
            </div>
            <div className="flex gap-1.5">
              {[ExerciseCategory.MAIN_LIFT, ExerciseCategory.ACCESSORY].map((option) => (
                <Button
                  key={option}
                  type="button"
                  variant="outline"
                  size="sm"
                  aria-expanded={category === option}
                  className={cn(category === option && "border-ring")}
                  onClick={() => setCategory(option)}
                >
                  {categoryLabel(option)}
                </Button>
              ))}
            </div>
            {createExercise.isError && (
              <p className="text-sm text-destructive">{createExercise.error.message}</p>
            )}
          </form>

          <Link to="/" className="text-sm text-muted-foreground underline underline-offset-4">
            Back
          </Link>
        </CardContent>
      </Card>
    </div>
  );
}
