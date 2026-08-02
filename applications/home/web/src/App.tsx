import { useMemo, useState } from "react";
import { Search } from "lucide-react";

import { ThemeToggle } from "@/components/ThemeToggle";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { APPS, GROUP_LABELS, appHref, type Group } from "@/apps";

type GroupFilter = Group | "all";

const GROUP_FILTERS: { key: GroupFilter; label: string }[] = [
  { key: "all", label: "All" },
  { key: "apps", label: GROUP_LABELS.apps },
  { key: "infra", label: GROUP_LABELS.infra },
];

export function App() {
  const [query, setQuery] = useState("");
  const [groupFilter, setGroupFilter] = useState<GroupFilter>("all");

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    return APPS.filter((app) => {
      if (groupFilter !== "all" && app.group !== groupFilter) return false;
      if (!q) return true;
      return app.name.toLowerCase().includes(q) || app.description.toLowerCase().includes(q);
    });
  }, [query, groupFilter]);

  const groups: Group[] = ["apps", "infra"];

  return (
    <div className="mx-auto flex min-h-screen max-w-3xl flex-col px-4 py-6">
      <header className="flex items-center justify-between">
        <h1 className="font-heading text-lg font-semibold">Home</h1>
        <ThemeToggle />
      </header>

      <div className="mt-4 flex flex-wrap items-center gap-2">
        <div className="relative flex-1 min-w-40">
          <Search className="pointer-events-none absolute top-1/2 left-2.5 size-3.5 -translate-y-1/2 text-muted-foreground" />
          <Input
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Search apps..."
            className="pl-8"
            aria-label="Search apps"
          />
        </div>
        <div className="flex gap-1">
          {GROUP_FILTERS.map((f) => (
            <Button
              key={f.key}
              type="button"
              size="sm"
              variant={groupFilter === f.key ? "secondary" : "ghost"}
              onClick={() => setGroupFilter(f.key)}
            >
              {f.label}
            </Button>
          ))}
        </div>
      </div>

      <div className="mt-6 flex flex-col gap-8">
        {groups.map((group) => {
          const appsInGroup = filtered.filter((app) => app.group === group);
          if (appsInGroup.length === 0) return null;

          return (
            <section key={group}>
              <h2 className="mb-3 text-xs font-medium tracking-wide text-muted-foreground uppercase">
                {GROUP_LABELS[group]}
              </h2>
              <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                {appsInGroup.map((app) => {
                  return (
                    <a key={app.subdomain} href={appHref(app)} className="block">
                      <Card className="h-full transition-colors hover:bg-muted/50">
                        <CardContent className="flex items-center gap-3">
                          <div className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-muted text-lg">
                            {app.icon}
                          </div>
                          <div className="min-w-0">
                            <div className="font-medium">{app.name}</div>
                            <div className="truncate text-sm text-muted-foreground">
                              {app.description}
                            </div>
                          </div>
                        </CardContent>
                      </Card>
                    </a>
                  );
                })}
              </div>
            </section>
          );
        })}

        {filtered.length === 0 && (
          <p className="text-center text-sm text-muted-foreground">No apps match "{query}".</p>
        )}
      </div>
    </div>
  );
}
