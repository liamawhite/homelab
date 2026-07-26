import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { labelClient } from "./client";
import type { Label } from "@/gen/shopping/v1/label_pb";

const labelsQueryKey = ["labels"];

// Display order is entirely a client concern - the server returns labels in
// no particular order (see queries.sql's ListLabels). Active labels sort
// alphabetically before archived ones, alphabetically within each group.
function compareLabels(a: Label, b: Label): number {
  if (a.archived !== b.archived) return a.archived ? 1 : -1;
  return a.name.localeCompare(b.name);
}

// Returns every label, active and archived - callers filter as needed (the
// item input/tabs want only active ones, the manage-labels dialog wants
// both, split into sections).
export function useLabels() {
  return useQuery({
    queryKey: labelsQueryKey,
    queryFn: async () => (await labelClient.listLabels({})).labels,
    select: (labels) => labels.slice().sort(compareLabels),
  });
}

export function useCreateLabel() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (name: string) => {
      const res = await labelClient.createLabel({ name });
      if (!res.label) {
        throw new Error("CreateLabel response did not include a label");
      }
      return res.label;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: labelsQueryKey });
    },
  });
}

export function useArchiveLabel() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (id: string) => {
      await labelClient.archiveLabel({ id });
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: labelsQueryKey });
    },
  });
}

export function useRestoreLabel() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (id: string) => {
      await labelClient.restoreLabel({ id });
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: labelsQueryKey });
    },
  });
}

export function useUpdateLabelColor() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async ({ id, color }: { id: string; color: string }) => {
      const res = await labelClient.updateLabelColor({ id, color });
      if (!res.label) {
        throw new Error("UpdateLabelColor response did not include a label");
      }
      return res.label;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: labelsQueryKey });
    },
  });
}
