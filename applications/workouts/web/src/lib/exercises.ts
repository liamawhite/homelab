import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { exerciseClient } from "./client";
import { ExerciseCategory, type Exercise } from "@/gen/workouts/v1/exercise_pb";

const exercisesQueryKey = ["exercises"];

// Display order is entirely a client concern - the server returns exercises
// in creation order (see queries.sql's ListExercises). Active exercises
// sort alphabetically before archived ones, alphabetically within each
// group - same convention as shopping's compareLabels.
function compareExercises(a: Exercise, b: Exercise): number {
  if (a.archived !== b.archived) return a.archived ? 1 : -1;
  return a.name.localeCompare(b.name);
}

export function useExercises() {
  return useQuery({
    queryKey: exercisesQueryKey,
    queryFn: async () => (await exerciseClient.listExercises({})).exercises,
    select: (exercises) => exercises.slice().sort(compareExercises),
  });
}

export function useCreateExercise() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async ({ name, category }: { name: string; category: ExerciseCategory }) => {
      const res = await exerciseClient.createExercise({ name, category });
      if (!res.exercise) {
        throw new Error("CreateExercise response did not include an exercise");
      }
      return res.exercise;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: exercisesQueryKey });
    },
  });
}

export function useArchiveExercise() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (id: string) => {
      await exerciseClient.archiveExercise({ id });
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: exercisesQueryKey });
    },
  });
}

export function useRestoreExercise() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (id: string) => {
      await exerciseClient.restoreExercise({ id });
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: exercisesQueryKey });
    },
  });
}
