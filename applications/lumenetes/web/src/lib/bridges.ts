import { useQuery } from "@tanstack/react-query";

import { bridgeClient } from "./client";
import type { Bridge } from "@/gen/lumenetes/v1/bridge_pb";

function compareBridges(a: Bridge, b: Bridge): number {
  return a.name.localeCompare(b.name);
}

export function useBridges() {
  return useQuery({
    queryKey: ["bridges"],
    queryFn: async () => (await bridgeClient.listBridges({})).bridges,
    select: (bridges) => bridges.slice().sort(compareBridges),
    refetchInterval: 15_000,
  });
}
