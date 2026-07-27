import { useQuery } from "@tanstack/react-query";

import { sceneClient } from "./client";
import type { Scene } from "@/gen/lumenetes/v1/scene_pb";

function compareScenes(a: Scene, b: Scene): number {
  return a.id.localeCompare(b.id);
}

export function useScenes() {
  return useQuery({
    queryKey: ["scenes"],
    queryFn: async () => (await sceneClient.listScenes({})).scenes,
    select: (scenes) => scenes.slice().sort(compareScenes),
    refetchInterval: 15_000,
  });
}
