import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { cycleClient } from "./client";
import { MoveDirection } from "@/gen/workouts/v1/cycle_pb";

const cyclesQueryKey = ["cycles"];

export function useCycles(userId: string | null) {
  return useQuery({
    queryKey: [...cyclesQueryKey, "list", userId],
    queryFn: async () => (await cycleClient.listCycles({ userId: userId! })).cycles,
    enabled: !!userId,
  });
}

export function useCycle(cycleId: string | null) {
  return useQuery({
    queryKey: [...cyclesQueryKey, "detail", cycleId],
    queryFn: async () => {
      const res = await cycleClient.getCycle({ id: cycleId! });
      if (!res.cycle) {
        throw new Error("GetCycle response did not include a cycle");
      }
      return res.cycle;
    },
    enabled: !!cycleId,
  });
}

// Every mutation below invalidates the whole ["cycles"] prefix rather than
// a specific key - cheap and covers both the list and any open detail
// query, same rationale as useRecordTrainingMax.
function useInvalidateCycles() {
  const queryClient = useQueryClient();
  return () => queryClient.invalidateQueries({ queryKey: cyclesQueryKey });
}

export function useCreateCycle() {
  const invalidate = useInvalidateCycles();

  return useMutation({
    mutationFn: async ({ userId, name }: { userId: string; name: string }) => {
      const res = await cycleClient.createCycle({ userId, name });
      if (!res.cycle) {
        throw new Error("CreateCycle response did not include a cycle");
      }
      return res.cycle;
    },
    onSuccess: () => void invalidate(),
  });
}

export function useDuplicateCycle() {
  const invalidate = useInvalidateCycles();

  return useMutation({
    mutationFn: async ({ sourceCycleId, name }: { sourceCycleId: string; name: string }) => {
      const res = await cycleClient.duplicateCycle({ sourceCycleId, name });
      if (!res.cycle) {
        throw new Error("DuplicateCycle response did not include a cycle");
      }
      return res.cycle;
    },
    onSuccess: () => void invalidate(),
  });
}

export function useDeleteCycle() {
  const invalidate = useInvalidateCycles();

  return useMutation({
    mutationFn: async (id: string) => {
      await cycleClient.deleteCycle({ id });
    },
    onSuccess: () => void invalidate(),
  });
}

export function useCreateBlock() {
  const invalidate = useInvalidateCycles();

  return useMutation({
    mutationFn: async ({ cycleId, name }: { cycleId: string; name: string }) => {
      const res = await cycleClient.createBlock({ cycleId, name });
      if (!res.block) {
        throw new Error("CreateBlock response did not include a block");
      }
      return res.block;
    },
    onSuccess: () => void invalidate(),
  });
}

export function useDeleteBlock() {
  const invalidate = useInvalidateCycles();

  return useMutation({
    mutationFn: async (id: string) => {
      await cycleClient.deleteBlock({ id });
    },
    onSuccess: () => void invalidate(),
  });
}

export function useAddCycleExercise() {
  const invalidate = useInvalidateCycles();

  return useMutation({
    mutationFn: async ({ cycleId, exerciseId }: { cycleId: string; exerciseId: string }) => {
      const res = await cycleClient.addCycleExercise({ cycleId, exerciseId });
      if (!res.cycleExercise) {
        throw new Error("AddCycleExercise response did not include a cycle exercise");
      }
      return res.cycleExercise;
    },
    onSuccess: () => void invalidate(),
  });
}

export function useRemoveCycleExercise() {
  const invalidate = useInvalidateCycles();

  return useMutation({
    mutationFn: async (id: string) => {
      await cycleClient.removeCycleExercise({ id });
    },
    onSuccess: () => void invalidate(),
  });
}

export function useMoveCycleExercise() {
  const invalidate = useInvalidateCycles();

  return useMutation({
    mutationFn: async ({ id, direction }: { id: string; direction: MoveDirection }) => {
      await cycleClient.moveCycleExercise({ id, direction });
    },
    onSuccess: () => void invalidate(),
  });
}

export function useAddSet() {
  const invalidate = useInvalidateCycles();

  return useMutation({
    mutationFn: async ({
      cycleExerciseId,
      blockId,
      reps,
      percentageOfTm,
    }: {
      cycleExerciseId: string;
      blockId: string;
      reps: number;
      percentageOfTm?: number;
    }) => {
      const res = await cycleClient.addSet({
        cycleExerciseId,
        blockId,
        reps: BigInt(reps),
        percentageOfTm,
      });
      if (!res.exerciseSet) {
        throw new Error("AddSet response did not include an exercise set");
      }
      return res.exerciseSet;
    },
    onSuccess: () => void invalidate(),
  });
}

export function useUpdateSet() {
  const invalidate = useInvalidateCycles();

  return useMutation({
    mutationFn: async ({
      id,
      reps,
      percentageOfTm,
    }: {
      id: string;
      reps: number;
      percentageOfTm?: number;
    }) => {
      const res = await cycleClient.updateSet({ id, reps: BigInt(reps), percentageOfTm });
      if (!res.exerciseSet) {
        throw new Error("UpdateSet response did not include an exercise set");
      }
      return res.exerciseSet;
    },
    onSuccess: () => void invalidate(),
  });
}

export function useRemoveSet() {
  const invalidate = useInvalidateCycles();

  return useMutation({
    mutationFn: async (id: string) => {
      await cycleClient.removeSet({ id });
    },
    onSuccess: () => void invalidate(),
  });
}

export function useMoveSet() {
  const invalidate = useInvalidateCycles();

  return useMutation({
    mutationFn: async ({ id, direction }: { id: string; direction: MoveDirection }) => {
      await cycleClient.moveSet({ id, direction });
    },
    onSuccess: () => void invalidate(),
  });
}
