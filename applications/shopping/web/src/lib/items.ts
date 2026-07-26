import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { itemClient } from "./client";

const itemsQueryKey = ["items"];

export function useItems() {
  return useQuery({
    queryKey: itemsQueryKey,
    queryFn: async () => (await itemClient.listItems({})).items,
  });
}

export function useCreateItem() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (name: string) => {
      const res = await itemClient.createItem({ name });
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
