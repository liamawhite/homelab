import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { trainingMaxClient } from "./client";
import type { WeightUnit } from "@/gen/workouts/v1/training_max_pb";

const trainingMaxesQueryKey = ["trainingMaxes"];

// Current training maxes are sorted by exercise name server-side (see
// queries.sql's ListCurrentTrainingMaxes), so no client-side sort needed.
export function useCurrentTrainingMaxes(userId: string | null) {
  return useQuery({
    queryKey: [...trainingMaxesQueryKey, "current", userId],
    queryFn: async () => (await trainingMaxClient.listCurrentTrainingMaxes({ userId: userId! })).trainingMaxes,
    enabled: !!userId,
  });
}

export function useTrainingMaxHistory(userId: string | null, exerciseId: string | null) {
  return useQuery({
    queryKey: [...trainingMaxesQueryKey, "history", userId, exerciseId],
    queryFn: async () =>
      (await trainingMaxClient.listTrainingMaxHistory({ userId: userId!, exerciseId: exerciseId! })).trainingMaxes,
    enabled: !!userId && !!exerciseId,
  });
}

export function useRecordTrainingMax() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async ({
      userId,
      exerciseId,
      weight,
      unit,
    }: {
      userId: string;
      exerciseId: string;
      weight: number;
      unit: WeightUnit;
    }) => {
      const res = await trainingMaxClient.recordTrainingMax({ userId, exerciseId, weight, unit });
      if (!res.trainingMax) {
        throw new Error("RecordTrainingMax response did not include a training max");
      }
      return res.trainingMax;
    },
    onSuccess: () => {
      // Prefix match covers both "current" and any open "history" queries.
      void queryClient.invalidateQueries({ queryKey: trainingMaxesQueryKey });
    },
  });
}
