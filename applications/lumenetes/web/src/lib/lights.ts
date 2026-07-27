import { useQuery } from "@tanstack/react-query";

import { lightClient } from "./client";
import type { Light } from "@/gen/lumenetes/v1/light_pb";

function compareLights(a: Light, b: Light): number {
  return a.name.localeCompare(b.name);
}

export function useLights() {
  return useQuery({
    queryKey: ["lights"],
    queryFn: async () => (await lightClient.listLights({})).lights,
    select: (lights) => lights.slice().sort(compareLights),
    refetchInterval: 15_000,
  });
}
