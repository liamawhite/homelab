import { useScenes } from "@/lib/scenes";
import { formatBrightness, formatColorTempK } from "@/lib/format";
import { Badge } from "@/components/ui/badge";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import type { SceneLightState } from "@/gen/lumenetes/v1/scene_pb";

function lightStateSummary(state: SceneLightState): string {
  const parts: string[] = [];
  if (state.on !== undefined) parts.push(state.on ? "on" : "off");
  if (state.brightness !== undefined) parts.push(formatBrightness(state.brightness));
  if (state.color !== undefined) parts.push(state.color);
  if (state.colorTempK !== undefined) parts.push(formatColorTempK(state.colorTempK));
  return `${state.name}: ${parts.join(", ") || "unchanged"}`;
}

export function ScenesPage() {
  const { data: scenes, isLoading, isError, error } = useScenes();

  return (
    <div className="mx-auto flex w-full max-w-5xl flex-1 flex-col gap-4 p-4">
      <h1 className="text-lg font-semibold">Scenes</h1>

      {isLoading && <p className="text-sm text-muted-foreground">Loading scenes…</p>}
      {isError && <p className="text-sm text-destructive">{error.message}</p>}

      {scenes && scenes.length > 0 && (
        <div className="rounded-lg border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Group</TableHead>
                <TableHead>Lights</TableHead>
                <TableHead>Status</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {scenes.map((scene) => (
                <TableRow key={scene.id}>
                  <TableCell className="font-medium">{scene.id}</TableCell>
                  <TableCell className="text-muted-foreground">{scene.group}</TableCell>
                  <TableCell>
                    <ul className="list-inside list-disc text-sm text-muted-foreground">
                      {scene.lights.map((state) => (
                        <li key={state.name}>{lightStateSummary(state)}</li>
                      ))}
                    </ul>
                  </TableCell>
                  <TableCell>
                    {scene.invalidLights.length > 0 && (
                      <Badge variant="destructive">
                        {scene.invalidLights.length} invalid: {scene.invalidLights.join(", ")}
                      </Badge>
                    )}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}
      {scenes && scenes.length === 0 && (
        <p className="text-sm text-muted-foreground">No scenes found.</p>
      )}
    </div>
  );
}
