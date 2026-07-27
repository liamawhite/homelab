import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { accommodationClient } from "./client";

const accommodationsQueryKey = (tripId: string) => ["accommodations", tripId];

export function useAccommodations(tripId: string) {
  return useQuery({
    queryKey: accommodationsQueryKey(tripId),
    queryFn: async () => (await accommodationClient.listAccommodations({ tripId })).accommodations,
    enabled: !!tripId,
  });
}

export function useAddAccommodation(tripId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (input: { name: string; location: string; checkIn: string; checkOut: string }) => {
      const res = await accommodationClient.addAccommodation({ tripId, ...input });
      if (!res.accommodation) {
        throw new Error("AddAccommodation response did not include an accommodation");
      }
      return res.accommodation;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: accommodationsQueryKey(tripId) });
    },
  });
}

export function useDeleteAccommodation(tripId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (id: string) => {
      await accommodationClient.deleteAccommodation({ id });
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: accommodationsQueryKey(tripId) });
    },
  });
}
