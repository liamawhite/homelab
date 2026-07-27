import { Link } from "@tanstack/react-router";
import { LayoutDashboard, Lightbulb, ToggleLeft, Group as GroupIcon, Clapperboard, SunMedium, Router } from "lucide-react";

import { ThemeToggle } from "@/components/ThemeToggle";
import { Button } from "@/components/ui/button";

const links = [
  { to: "/", label: "Dashboard", icon: LayoutDashboard },
  { to: "/lights", label: "Lights", icon: Lightbulb },
  { to: "/switches", label: "Switches", icon: ToggleLeft },
  { to: "/groups", label: "Groups", icon: GroupIcon },
  { to: "/scenes", label: "Scenes", icon: Clapperboard },
  { to: "/schedules", label: "Schedules", icon: SunMedium },
  { to: "/bridges", label: "Bridges", icon: Router },
] as const;

export function Navbar() {
  return (
    <header className="flex items-center justify-between border-b px-4 py-3">
      <span className="text-sm font-semibold">Lumenetes</span>
      <div className="flex items-center gap-2">
        {links.map(({ to, label, icon: Icon }) => (
          <Button key={to} variant="ghost" size="icon" aria-label={label} asChild>
            <Link to={to} activeProps={{ className: "bg-muted" }}>
              <Icon />
            </Link>
          </Button>
        ))}
        <ThemeToggle />
      </div>
    </header>
  );
}
