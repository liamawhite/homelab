import { timestampDate } from "@bufbuild/protobuf/wkt";

import { useLights } from "@/lib/lights";
import { relativeTime } from "@/lib/time";
import { formatBrightness, formatColorTempK } from "@/lib/format";
import { Badge } from "@/components/ui/badge";
import { ColorSwatch } from "@/components/ColorSwatch";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import type { Light } from "@/gen/lumenetes/v1/light_pb";

function hasDrift(light: Light): boolean {
  if (light.reactive) return false;
  return (
    light.desiredOn !== light.observedOn ||
    light.desiredBrightness !== light.observedBrightness ||
    light.desiredColor !== light.observedColor ||
    light.desiredColorTempK !== light.observedColorTempK
  );
}

export function LightsPage() {
  const { data: lights, isLoading, isError, error } = useLights();

  return (
    <div className="mx-auto flex w-full max-w-5xl flex-1 flex-col gap-4 p-4">
      <h1 className="text-lg font-semibold">Lights</h1>

      {isLoading && <p className="text-sm text-muted-foreground">Loading lights…</p>}
      {isError && <p className="text-sm text-destructive">{error.message}</p>}

      {lights && lights.length > 0 && (
        <div className="rounded-lg border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Bridge</TableHead>
                <TableHead>On</TableHead>
                <TableHead>Brightness</TableHead>
                <TableHead>Color</TableHead>
                <TableHead>Color Temp</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Last Synced</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {lights.map((light) => (
                <TableRow key={light.id}>
                  <TableCell className="font-medium">{light.name}</TableCell>
                  <TableCell className="text-muted-foreground">{light.bridgeId}</TableCell>
                  <TableCell>
                    <Badge variant={light.observedOn ? "default" : "outline"}>
                      {light.observedOn ? "On" : "Off"}
                    </Badge>
                  </TableCell>
                  <TableCell>{formatBrightness(light.observedBrightness)}</TableCell>
                  <TableCell>
                    <ColorSwatch color={light.observedColor} />
                  </TableCell>
                  <TableCell>{formatColorTempK(light.observedColorTempK)}</TableCell>
                  <TableCell>
                    <div className="flex flex-wrap gap-1">
                      {!light.reachable && <Badge variant="destructive">Unreachable</Badge>}
                      {light.reactive && <Badge variant="secondary">Reactive</Badge>}
                      {light.enactError ? (
                        <Badge variant="destructive" title={light.enactError}>
                          Enact error
                        </Badge>
                      ) : (
                        hasDrift(light) && <Badge variant="outline">Pending</Badge>
                      )}
                    </div>
                  </TableCell>
                  <TableCell className="text-muted-foreground">
                    {light.lastSynced ? relativeTime(timestampDate(light.lastSynced)) : "—"}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}
      {lights && lights.length === 0 && (
        <p className="text-sm text-muted-foreground">No lights found.</p>
      )}
    </div>
  );
}
