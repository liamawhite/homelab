import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { timestampMs } from "@bufbuild/protobuf/wkt";

import { tripClient } from "./client";
import type { Trip } from "@/gen/trips/v1/trip_pb";

const tripsQueryKey = ["trips"];

// Display order is entirely a client concern - the server returns trips in
// no particular order (see queries.sql's ListTrips). Most recently created
// trip first.
function compareTrips(a: Trip, b: Trip): number {
  return (b.createdAt ? timestampMs(b.createdAt) : 0) - (a.createdAt ? timestampMs(a.createdAt) : 0);
}

export function useTrips() {
  return useQuery({
    queryKey: tripsQueryKey,
    queryFn: async () => (await tripClient.listTrips({})).trips,
    select: (trips) => trips.slice().sort(compareTrips),
  });
}

export function useCreateTrip() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (name: string) => {
      const res = await tripClient.createTrip({ name });
      if (!res.trip) {
        throw new Error("CreateTrip response did not include a trip");
      }
      return res.trip;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: tripsQueryKey });
    },
  });
}

export function useDeleteTrip() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (id: string) => {
      await tripClient.deleteTrip({ id });
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: tripsQueryKey });
    },
  });
}
