import { useNavigate, useSearch } from "@tanstack/react-router";
import { ChevronDown, ChevronRight } from "lucide-react";

import { useBridges } from "@/lib/bridges";
import { useGroups } from "@/lib/groups";
import { useLights } from "@/lib/lights";
import { activeSceneKindLabel, formatBrightness } from "@/lib/format";
import { hexToRgba, lightSwatchColor } from "@/lib/color";
import { parseOpenKeys, toggleOpenValue } from "@/lib/searchState";
import { cn } from "@/lib/utils";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { ColorSwatch } from "@/components/ColorSwatch";
import type { Group } from "@/gen/lumenetes/v1/group_pb";
import type { Light } from "@/gen/lumenetes/v1/light_pb";

interface GroupCardProps {
  group: Group;
  lights: Map<string, Light>;
  expanded: boolean;
  onToggle: (open: boolean) => void;
}

function GroupCard({ group, lights, expanded, onToggle }: GroupCardProps) {
  const members = group.lights.map((id) => lights.get(id)).filter((light): light is Light => light !== undefined);
  const onCount = members.filter((light) => light.observedOn).length;

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center justify-between">
          <span>{group.id}</span>
          <Badge variant={group.activeScene ? "default" : "outline"}>
            {group.activeScene ? activeSceneKindLabel(group.activeScene.kind) : "Unmanaged"}
          </Badge>
        </CardTitle>
      </CardHeader>
      <CardContent className="flex flex-col gap-3 text-sm">
        <p className="text-muted-foreground">
          {onCount}/{members.length} light{members.length === 1 ? "" : "s"} on
          {group.activeScene && group.activeScene.name && ` · ${group.activeScene.name}`}
        </p>
        {group.missingLights.length > 0 && (
          <Badge variant="destructive">
            {group.missingLights.length} missing: {group.missingLights.join(", ")}
          </Badge>
        )}
        {group.activeSceneError && <Badge variant="destructive">{group.activeSceneError}</Badge>}

        {members.length > 0 && (
          <div>
            <Button
              variant="ghost"
              size="sm"
              className="gap-1 px-2"
              onClick={() => onToggle(!expanded)}
            >
              {expanded ? <ChevronDown /> : <ChevronRight />}
              {expanded ? "Hide lights" : `Show ${members.length} light${members.length === 1 ? "" : "s"}`}
            </Button>
            {expanded && (
              <ul className="mt-2 flex flex-col gap-1.5">
                {members.map((light) => {
                  const swatch = lightSwatchColor(light);
                  return (
                    <li
                      key={light.id}
                      className={cn(
                        "flex flex-wrap items-center gap-2 rounded-md border p-2 transition-colors",
                        !light.observedOn && "opacity-60",
                      )}
                      style={
                        swatch
                          ? { backgroundColor: hexToRgba(swatch, 0.16), borderColor: hexToRgba(swatch, 0.5) }
                          : undefined
                      }
                    >
                      <span className="flex-1 font-medium">{light.name}</span>
                      <Badge variant={light.observedOn ? "default" : "outline"}>
                        {light.observedOn ? "On" : "Off"}
                      </Badge>
                      <span className="text-muted-foreground">{formatBrightness(light.observedBrightness)}</span>
                      <ColorSwatch color={light.observedColor} />
                      {!light.reachable && <Badge variant="destructive">Unreachable</Badge>}
                    </li>
                  );
                })}
              </ul>
            )}
          </div>
        )}
      </CardContent>
    </Card>
  );
}

export function DashboardPage() {
  const { data: bridges } = useBridges();
  const { data: groups, isLoading, isError, error } = useGroups();
  const { data: lights } = useLights();

  const { open } = useSearch({ from: "/" });
  const navigate = useNavigate({ from: "/" });
  const openKeys = parseOpenKeys(open);
  const setOpen = (key: string, isOpen: boolean) =>
    navigate({ search: (prev) => ({ ...prev, open: toggleOpenValue(openKeys, key, isOpen) }), replace: true });

  const lightsById = new Map((lights ?? []).map((light) => [light.id, light]));

  return (
    <div className="mx-auto flex w-full max-w-5xl flex-1 flex-col gap-4 p-4">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h1 className="text-lg font-semibold">Dashboard</h1>
        {bridges && bridges.length > 0 && (
          <div className="flex flex-wrap gap-2">
            {bridges.map((bridge) => (
              <Badge key={bridge.id} variant={bridge.reachable ? "default" : "destructive"}>
                {bridge.name || bridge.id} {bridge.reachable ? "online" : "offline"}
              </Badge>
            ))}
          </div>
        )}
      </div>

      {isLoading && <p className="text-sm text-muted-foreground">Loading groups…</p>}
      {isError && <p className="text-sm text-destructive">{error.message}</p>}

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        {groups?.map((group) => (
          <GroupCard
            key={group.id}
            group={group}
            lights={lightsById}
            expanded={openKeys.has(group.id)}
            onToggle={(next) => setOpen(group.id, next)}
          />
        ))}
      </div>
      {groups && groups.length === 0 && (
        <p className="text-sm text-muted-foreground">No groups found.</p>
      )}
    </div>
  );
}
