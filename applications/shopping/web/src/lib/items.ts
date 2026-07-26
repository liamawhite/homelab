import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { timestampMs } from "@bufbuild/protobuf/wkt";

import { itemClient } from "./client";
import { ItemStatus, type Item } from "@/gen/shopping/v1/item_pb";

const itemsQueryKey = ["items"];

// Display order is entirely a client concern - the server returns items in
// no particular order (see queries.sql's ListItems). Todo items sort most
// recently created first; done items sort after all todo items, most
// recently completed first.
function compareItems(a: Item, b: Item): number {
  const aDone = a.status === ItemStatus.DONE;
  const bDone = b.status === ItemStatus.DONE;
  if (aDone !== bDone) return aDone ? 1 : -1;

  if (aDone) {
    return (b.completedAt ? timestampMs(b.completedAt) : 0) - (a.completedAt ? timestampMs(a.completedAt) : 0);
  }
  return (b.createdAt ? timestampMs(b.createdAt) : 0) - (a.createdAt ? timestampMs(a.createdAt) : 0);
}

export function useItems() {
  return useQuery({
    queryKey: itemsQueryKey,
    queryFn: async () => (await itemClient.listItems({})).items,
    select: (items) => items.slice().sort(compareItems),
  });
}

export function useCreateItem() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async ({ name, labelId }: { name: string; labelId?: string }) => {
      const res = await itemClient.createItem({ name, labelId });
      if (!res.item) {
        throw new Error("CreateItem response did not include an item");
      }
      return res.item;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: itemsQueryKey });
    },
  });
}

export function useUpdateItemStatus() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async ({ id, status }: { id: string; status: ItemStatus }) => {
      const res = await itemClient.updateItemStatus({ id, status });
      if (!res.item) {
        throw new Error("UpdateItemStatus response did not include an item");
      }
      return res.item;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: itemsQueryKey });
    },
  });
}

export function useDeleteItem() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (id: string) => {
      await itemClient.deleteItem({ id });
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: itemsQueryKey });
    },
  });
}
