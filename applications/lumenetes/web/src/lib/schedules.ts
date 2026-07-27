import { useQuery } from "@tanstack/react-query";

import { circadianScheduleClient } from "./client";
import type { CircadianSchedule } from "@/gen/lumenetes/v1/circadian_schedule_pb";

function compareSchedules(a: CircadianSchedule, b: CircadianSchedule): number {
  return a.id.localeCompare(b.id);
}

export function useSchedules() {
  return useQuery({
    queryKey: ["schedules"],
    queryFn: async () => (await circadianScheduleClient.listCircadianSchedules({})).circadianSchedules,
    select: (schedules) => schedules.slice().sort(compareSchedules),
    refetchInterval: 15_000,
  });
}
