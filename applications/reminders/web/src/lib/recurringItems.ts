import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { MessageInitShape } from "@bufbuild/protobuf";
import { timestampMs } from "@bufbuild/protobuf/wkt";

import { recurringItemClient } from "./client";
import type { RecurringItem } from "@/gen/reminders/v1/recurring_item_pb";
import {
  CreateRecurringItemRequestSchema,
  UpdateRecurringItemNotesRequestSchema,
  UpdateRecurringItemAreaRequestSchema,
  UpdateRecurringItemRuleRequestSchema,
} from "@/gen/reminders/v1/recurring_item_pb";

const recurringItemsQueryKey = ["recurringItems"];

// Display order is entirely a client concern, same as one-off items (see
// oneOffItems.ts) - soonest due first, since this is a schedule rather than
// a creation log.
function compareRecurringItems(a: RecurringItem, b: RecurringItem): number {
  return (a.nextDueDate ? timestampMs(a.nextDueDate) : 0) - (b.nextDueDate ? timestampMs(b.nextDueDate) : 0);
}

export function useRecurringItems() {
  return useQuery({
    queryKey: recurringItemsQueryKey,
    queryFn: async () => (await recurringItemClient.listRecurringItems({})).items,
    select: (items) => items.slice().sort(compareRecurringItems),
  });
}

export function useCreateRecurringItem() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (input: MessageInitShape<typeof CreateRecurringItemRequestSchema>) => {
      const res = await recurringItemClient.createRecurringItem(input);
      if (!res.item) {
        throw new Error("CreateRecurringItem response did not include an item");
      }
      return res.item;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: recurringItemsQueryKey });
    },
  });
}

export function useUpdateRecurringItemNotes() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (input: MessageInitShape<typeof UpdateRecurringItemNotesRequestSchema>) => {
      const res = await recurringItemClient.updateRecurringItemNotes(input);
      if (!res.item) {
        throw new Error("UpdateRecurringItemNotes response did not include an item");
      }
      return res.item;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: recurringItemsQueryKey });
    },
  });
}

export function useUpdateRecurringItemArea() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (input: MessageInitShape<typeof UpdateRecurringItemAreaRequestSchema>) => {
      const res = await recurringItemClient.updateRecurringItemArea(input);
      if (!res.item) {
        throw new Error("UpdateRecurringItemArea response did not include an item");
      }
      return res.item;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: recurringItemsQueryKey });
    },
  });
}

export function useUpdateRecurringItemRule() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (input: MessageInitShape<typeof UpdateRecurringItemRuleRequestSchema>) => {
      const res = await recurringItemClient.updateRecurringItemRule(input);
      if (!res.item) {
        throw new Error("UpdateRecurringItemRule response did not include an item");
      }
      return res.item;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: recurringItemsQueryKey });
    },
  });
}

export function useCompleteRecurringItem() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (id: string) => {
      const res = await recurringItemClient.completeRecurringItem({ id });
      if (!res.item) {
        throw new Error("CompleteRecurringItem response did not include an item");
      }
      return res.item;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: recurringItemsQueryKey });
    },
  });
}

export function useDeleteRecurringItem() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (id: string) => {
      await recurringItemClient.deleteRecurringItem({ id });
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: recurringItemsQueryKey });
    },
  });
}
