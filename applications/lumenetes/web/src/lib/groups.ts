import { useQuery } from "@tanstack/react-query";

import { groupClient } from "./client";
import type { Group } from "@/gen/lumenetes/v1/group_pb";

function compareGroups(a: Group, b: Group): number {
  return a.id.localeCompare(b.id);
}

export function useGroups() {
  return useQuery({
    queryKey: ["groups"],
    queryFn: async () => (await groupClient.listGroups({})).groups,
    select: (groups) => groups.slice().sort(compareGroups),
    refetchInterval: 15_000,
  });
}
