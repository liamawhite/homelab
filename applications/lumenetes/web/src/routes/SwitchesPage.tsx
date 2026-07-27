import { useMemo, useState } from "react";
import { useNavigate, useSearch } from "@tanstack/react-router";
import { timestampDate } from "@bufbuild/protobuf/wkt";
import { ChevronDown, ChevronRight } from "lucide-react";

import { useSwitches } from "@/lib/switches";
import { relativeTime } from "@/lib/time";
import { parseOpenKeys, toggleOpenValue } from "@/lib/searchState";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import type { Switch as HueSwitch, SwitchBinding } from "@/gen/lumenetes/v1/switch_pb";

function bindingSummary(binding: SwitchBinding): string {
  const action = binding.action;
  if (!action) return binding.event;
  const parts: string[] = [];
  if (action.on !== undefined) parts.push(action.on ? "on" : "off");
  if (action.toggle) parts.push("toggle");
  if (action.brightness !== undefined) parts.push(`brightness=${action.brightness}`);
  if (action.brightnessDelta !== undefined) parts.push(`brightness${action.brightnessDelta >= 0 ? "+" : ""}${action.brightnessDelta}`);
  if (action.color !== undefined) parts.push(`color=${action.color}`);
  if (action.colorTempK !== undefined) parts.push(`${action.colorTempK}K`);
  const targets = action.targetLights.length > 0 ? action.targetLights.join(", ") : "(no targets)";
  return `${binding.event} → ${targets}: ${parts.join(", ") || "no-op"}`;
}

// SwitchDevice groups every button CR belonging to one physical device (they
// share BridgeID/Name/Battery/Product/Model/Reachable - see Switch's doc
// comment in api/v1alpha1/switch_types.go - and differ only by ControlID/
// LastEvent/LastEventTime/Bindings), so a multi-button device (e.g. a
// 4-button Hue Dimmer Switch) renders as one row with a button picker
// instead of one row per button.
interface SwitchDevice {
  key: string;
  name: string;
  battery: number;
  reachable: boolean;
  buttons: HueSwitch[];
}

function groupSwitches(switches: HueSwitch[]): SwitchDevice[] {
  const byKey = new Map<string, HueSwitch[]>();
  for (const sw of switches) {
    const key = `${sw.bridgeId}:${sw.name}`;
    const group = byKey.get(key);
    if (group) {
      group.push(sw);
    } else {
      byKey.set(key, [sw]);
    }
  }

  return Array.from(byKey.entries()).map(([key, buttons]) => {
    const sorted = buttons.slice().sort((a, b) => a.controlId - b.controlId);
    return {
      key,
      name: sorted[0].name,
      battery: sorted[0].battery,
      reachable: sorted.every((button) => button.reachable),
      buttons: sorted,
    };
  });
}

interface DeviceRowProps {
  device: SwitchDevice;
  expanded: boolean;
  onToggle: (open: boolean) => void;
}

function DeviceRow({ device, expanded, onToggle }: DeviceRowProps) {
  const [selectedId, setSelectedId] = useState(device.buttons[0].id);
  const selected = device.buttons.find((button) => button.id === selectedId) ?? device.buttons[0];

  return (
    <>
      <TableRow>
        <TableCell>
          <Button
            variant="ghost"
            size="icon-sm"
            aria-label={expanded ? "Collapse bindings" : "Expand bindings"}
            onClick={() => onToggle(!expanded)}
            disabled={selected.bindings.length === 0}
          >
            {expanded ? <ChevronDown /> : <ChevronRight />}
          </Button>
        </TableCell>
        <TableCell className="font-medium">{device.name}</TableCell>
        <TableCell>
          {device.buttons.length > 1 ? (
            <Select
              value={selectedId}
              onValueChange={(id) => {
                setSelectedId(id);
                onToggle(false);
              }}
            >
              <SelectTrigger size="sm">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {device.buttons.map((button) => (
                  <SelectItem key={button.id} value={button.id}>
                    Button {button.controlId}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          ) : (
            <span className="text-muted-foreground">Button {selected.controlId}</span>
          )}
        </TableCell>
        <TableCell>{device.battery < 0 ? "—" : `${device.battery}%`}</TableCell>
        <TableCell className="text-muted-foreground">{selected.lastEvent || "—"}</TableCell>
        <TableCell className="text-muted-foreground">
          {selected.lastEventTime ? relativeTime(timestampDate(selected.lastEventTime)) : "—"}
        </TableCell>
        <TableCell>
          {!device.reachable && <Badge variant="destructive">Unreachable</Badge>}
          {device.reachable && device.battery >= 0 && device.battery < 20 && (
            <Badge variant="destructive">Low battery</Badge>
          )}
        </TableCell>
      </TableRow>
      {expanded && selected.bindings.length > 0 && (
        <TableRow>
          <TableCell />
          <TableCell colSpan={6}>
            <ul className="list-inside list-disc text-sm text-muted-foreground">
              {selected.bindings.map((binding, i) => (
                <li key={i}>{bindingSummary(binding)}</li>
              ))}
            </ul>
          </TableCell>
        </TableRow>
      )}
    </>
  );
}

export function SwitchesPage() {
  const { data: switches, isLoading, isError, error } = useSwitches();
  const devices = useMemo(() => (switches ? groupSwitches(switches) : []), [switches]);

  const { open } = useSearch({ from: "/switches" });
  const navigate = useNavigate({ from: "/switches" });
  const openKeys = parseOpenKeys(open);
  const setOpen = (key: string, isOpen: boolean) =>
    navigate({ search: (prev) => ({ ...prev, open: toggleOpenValue(openKeys, key, isOpen) }), replace: true });

  return (
    <div className="mx-auto flex w-full max-w-5xl flex-1 flex-col gap-4 p-4">
      <h1 className="text-lg font-semibold">Switches</h1>

      {isLoading && <p className="text-sm text-muted-foreground">Loading switches…</p>}
      {isError && <p className="text-sm text-destructive">{error.message}</p>}

      {devices.length > 0 && (
        <div className="rounded-lg border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead />
                <TableHead>Name</TableHead>
                <TableHead>Button</TableHead>
                <TableHead>Battery</TableHead>
                <TableHead>Last Event</TableHead>
                <TableHead>Last Event Time</TableHead>
                <TableHead>Status</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {devices.map((device) => (
                <DeviceRow
                  key={device.key}
                  device={device}
                  expanded={openKeys.has(device.key)}
                  onToggle={(next) => setOpen(device.key, next)}
                />
              ))}
            </TableBody>
          </Table>
        </div>
      )}
      {switches && devices.length === 0 && (
        <p className="text-sm text-muted-foreground">No switches found.</p>
      )}
    </div>
  );
}
