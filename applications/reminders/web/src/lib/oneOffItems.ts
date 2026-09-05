import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { timestampMs } from "@bufbuild/protobuf/wkt";
import type { Timestamp } from "@bufbuild/protobuf/wkt";

import { oneOffItemClient } from "./client";
import { OneOffItemStatus, type Area, type OneOffItem } from "@/gen/reminders/v1/one_off_item_pb";

const oneOffItemsQueryKey = ["oneOffItems"];

// Display order is entirely a client concern - the server returns items in
// no particular order (see queries.sql's ListOneOffItems). Most recently
// created item first.
function compareOneOffItems(a: OneOffItem, b: OneOffItem): number {
  return (b.createdAt ? timestampMs(b.createdAt) : 0) - (a.createdAt ? timestampMs(a.createdAt) : 0);
}

export function useOneOffItems() {
  return useQuery({
    queryKey: oneOffItemsQueryKey,
    queryFn: async () => (await oneOffItemClient.listOneOffItems({})).items,
    // Completed items just disappear from the list, Todoist-style - there's
    // no separate "completed" view yet, so filtering is a client concern
    // rather than a server-side flag, same as sort order above.
    select: (items) => items.filter((item) => item.status !== OneOffItemStatus.COMPLETED).sort(compareOneOffItems),
  });
}

export function useCreateOneOffItem() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async ({
      title,
      notes,
      dueDate,
      area,
    }: {
      title: string;
      notes?: string;
      dueDate?: Timestamp;
      area: Area;
    }) => {
      const res = await oneOffItemClient.createOneOffItem({ title, notes, dueDate, area });
      if (!res.item) {
        throw new Error("CreateOneOffItem response did not include an item");
      }
      return res.item;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: oneOffItemsQueryKey });
    },
  });
}

export function useUpdateOneOffItemNotes() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async ({ id, notes }: { id: string; notes: string }) => {
      const res = await oneOffItemClient.updateOneOffItemNotes({ id, notes });
      if (!res.item) {
        throw new Error("UpdateOneOffItemNotes response did not include an item");
      }
      return res.item;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: oneOffItemsQueryKey });
    },
  });
}

export function useUpdateOneOffItemDueDate() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async ({ id, dueDate }: { id: string; dueDate?: Timestamp }) => {
      const res = await oneOffItemClient.updateOneOffItemDueDate({ id, dueDate });
      if (!res.item) {
        throw new Error("UpdateOneOffItemDueDate response did not include an item");
      }
      return res.item;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: oneOffItemsQueryKey });
    },
  });
}

export function useUpdateOneOffItemArea() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async ({ id, area }: { id: string; area: Area }) => {
      const res = await oneOffItemClient.updateOneOffItemArea({ id, area });
      if (!res.item) {
        throw new Error("UpdateOneOffItemArea response did not include an item");
      }
      return res.item;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: oneOffItemsQueryKey });
    },
  });
}

export function useUpdateOneOffItemStatus() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async ({ id, status }: { id: string; status: OneOffItemStatus }) => {
      const res = await oneOffItemClient.updateOneOffItemStatus({ id, status });
      if (!res.item) {
        throw new Error("UpdateOneOffItemStatus response did not include an item");
      }
      return res.item;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: oneOffItemsQueryKey });
    },
  });
}

export function useDeleteOneOffItem() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (id: string) => {
      await oneOffItemClient.deleteOneOffItem({ id });
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: oneOffItemsQueryKey });
    },
  });
}
