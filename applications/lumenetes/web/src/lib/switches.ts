import { useQuery } from "@tanstack/react-query";

import { switchClient } from "./client";
import type { Switch } from "@/gen/lumenetes/v1/switch_pb";

function compareSwitches(a: Switch, b: Switch): number {
  if (a.name !== b.name) return a.name.localeCompare(b.name);
  return a.controlId - b.controlId;
}

export function useSwitches() {
  return useQuery({
    queryKey: ["switches"],
    queryFn: async () => (await switchClient.listSwitches({})).switches,
    select: (switches) => switches.slice().sort(compareSwitches),
    refetchInterval: 15_000,
  });
}
