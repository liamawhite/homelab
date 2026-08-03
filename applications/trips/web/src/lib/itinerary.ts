import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { itineraryClient } from "./client";
import { ItineraryItemState } from "@/gen/trips/v1/itinerary_pb";

const itineraryItemsQueryKey = (tripId: string) => ["itineraryItems", tripId];

export function useItineraryItems(tripId: string) {
  return useQuery({
    queryKey: itineraryItemsQueryKey(tripId),
    queryFn: async () =>
      (await itineraryClient.listItineraryItems({ tripId })).itineraryItems,
    enabled: !!tripId,
  });
}

export function useAddItineraryItem(tripId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async ({
      title,
      placeId,
      timezone,
    }: {
      title: string;
      placeId?: string;
      timezone: string;
    }) => {
      const res = await itineraryClient.addItineraryItem({
        tripId,
        title,
        placeId,
        timezone,
      });
      if (!res.itineraryItem) {
        throw new Error(
          "AddItineraryItem response did not include an itinerary item",
        );
      }
      return res.itineraryItem;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: itineraryItemsQueryKey(tripId),
      });
    },
  });
}

export function useScheduleItineraryItem(tripId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (input: {
      id: string;
      state: ItineraryItemState;
      date: string;
      endDate: string;
      startMinute: number;
      endMinute: number;
      timezone: string;
    }) => {
      const res = await itineraryClient.scheduleItineraryItem(input);
      if (!res.itineraryItem) {
        throw new Error(
          "ScheduleItineraryItem response did not include an itinerary item",
        );
      }
      return res.itineraryItem;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: itineraryItemsQueryKey(tripId),
      });
    },
  });
}

export function useDeleteItineraryItem(tripId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (id: string) => {
      await itineraryClient.deleteItineraryItem({ id });
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: itineraryItemsQueryKey(tripId),
      });
    },
  });
}

// searchLocations is deliberately not a TanStack Query hook - it's
// ephemeral per-keystroke typeahead data, not a cacheable resource.
// LocationSearchInput calls this directly from a debounced effect and
// keeps results in local component state.
export async function searchLocations(query: string) {
  const res = await itineraryClient.searchLocations({ query });
  return res.candidates;
}
