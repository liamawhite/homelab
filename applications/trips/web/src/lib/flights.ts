import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { flightClient } from "./client";

const flightsQueryKey = (tripId: string) => ["flights", tripId];

export function useFlights(tripId: string) {
  return useQuery({
    queryKey: flightsQueryKey(tripId),
    queryFn: async () => (await flightClient.listFlights({ tripId })).flights,
    enabled: !!tripId,
  });
}

export function useAddFlight(tripId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async ({ flightNumber, date }: { flightNumber: string; date: string }) => {
      const res = await flightClient.addFlight({ tripId, flightNumber, date });
      if (!res.flight) {
        throw new Error("AddFlight response did not include a flight");
      }
      return res.flight;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: flightsQueryKey(tripId) });
    },
  });
}

export function useDeleteFlight(tripId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (id: string) => {
      await flightClient.deleteFlight({ id });
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: flightsQueryKey(tripId) });
    },
  });
}

export function useSyncFlight(tripId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (id: string) => {
      const res = await flightClient.syncFlight({ id });
      if (!res.flight) {
        throw new Error("SyncFlight response did not include a flight");
      }
      return res.flight;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: flightsQueryKey(tripId) });
    },
  });
}
