import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { timestampMs } from "@bufbuild/protobuf/wkt";

import { oneOffItemClient } from "./client";
import type { OneOffItem } from "@/gen/reminders/v1/one_off_item_pb";

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
    select: (items) => items.slice().sort(compareOneOffItems),
  });
}

export function useCreateOneOffItem() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (title: string) => {
      const res = await oneOffItemClient.createOneOffItem({ title });
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
