import { timestampDate } from "@bufbuild/protobuf/wkt";

import { useBridges } from "@/lib/bridges";
import { relativeTime } from "@/lib/time";
import { Badge } from "@/components/ui/badge";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";

export function BridgesPage() {
  const { data: bridges, isLoading, isError, error } = useBridges();

  return (
    <div className="mx-auto flex w-full max-w-4xl flex-1 flex-col gap-4 p-4">
      <h1 className="text-lg font-semibold">Bridges</h1>

      {isLoading && <p className="text-sm text-muted-foreground">Loading bridges…</p>}
      {isError && <p className="text-sm text-destructive">{error.message}</p>}

      {bridges && bridges.length > 0 && (
        <div className="rounded-lg border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>IP</TableHead>
                <TableHead>Model</TableHead>
                <TableHead>Firmware</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Last Resolved</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {bridges.map((bridge) => (
                <TableRow key={bridge.id}>
                  <TableCell className="font-medium">{bridge.name}</TableCell>
                  <TableCell className="text-muted-foreground">{bridge.ip}</TableCell>
                  <TableCell className="text-muted-foreground">{bridge.modelId}</TableCell>
                  <TableCell className="text-muted-foreground">{bridge.swVersion}</TableCell>
                  <TableCell>
                    <Badge variant={bridge.reachable ? "default" : "destructive"}>
                      {bridge.reachable ? "Reachable" : "Unreachable"}
                    </Badge>
                  </TableCell>
                  <TableCell className="text-muted-foreground">
                    {bridge.lastResolved ? relativeTime(timestampDate(bridge.lastResolved)) : "—"}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}
      {bridges && bridges.length === 0 && (
        <p className="text-sm text-muted-foreground">No bridges found.</p>
      )}
    </div>
  );
}
