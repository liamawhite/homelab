import { useGroups } from "@/lib/groups";
import { activeSceneKindLabel } from "@/lib/format";
import { Badge } from "@/components/ui/badge";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";

export function GroupsPage() {
  const { data: groups, isLoading, isError, error } = useGroups();

  return (
    <div className="mx-auto flex w-full max-w-5xl flex-1 flex-col gap-4 p-4">
      <h1 className="text-lg font-semibold">Groups</h1>

      {isLoading && <p className="text-sm text-muted-foreground">Loading groups…</p>}
      {isError && <p className="text-sm text-destructive">{error.message}</p>}

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        {groups?.map((group) => (
          <Card key={group.id}>
            <CardHeader>
              <CardTitle className="flex items-center justify-between">
                <span>{group.id}</span>
                <Badge variant={group.activeScene ? "default" : "outline"}>
                  {group.activeScene ? activeSceneKindLabel(group.activeScene.kind) : "Unmanaged"}
                </Badge>
              </CardTitle>
            </CardHeader>
            <CardContent className="flex flex-col gap-2 text-sm">
              <p className="text-muted-foreground">
                {group.lightCount} light{group.lightCount === 1 ? "" : "s"}
                {group.activeScene && group.activeScene.name && ` · ${group.activeScene.name}`}
              </p>
              {group.missingLights.length > 0 && (
                <Badge variant="destructive">
                  {group.missingLights.length} missing: {group.missingLights.join(", ")}
                </Badge>
              )}
              {group.activeSceneError && <Badge variant="destructive">{group.activeSceneError}</Badge>}
            </CardContent>
          </Card>
        ))}
      </div>
      {groups && groups.length === 0 && (
        <p className="text-sm text-muted-foreground">No groups found.</p>
      )}
    </div>
  );
}
