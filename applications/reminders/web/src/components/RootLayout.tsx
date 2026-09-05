import { Link, Outlet } from "@tanstack/react-router";

import { ThemeToggle } from "@/components/ThemeToggle";

export function RootLayout() {
  return (
    <div className="flex min-h-screen flex-col">
      <header className="flex items-center justify-between border-b px-4 py-3">
        <nav className="flex items-center gap-4">
          <span className="text-sm font-semibold">Reminders</span>
          <Link
            to="/"
            activeOptions={{ exact: true }}
            className="text-sm text-muted-foreground"
            activeProps={{ className: "text-sm font-medium text-foreground" }}
          >
            One-off
          </Link>
          <Link
            to="/recurring"
            className="text-sm text-muted-foreground"
            activeProps={{ className: "text-sm font-medium text-foreground" }}
          >
            Recurring
          </Link>
        </nav>
        <ThemeToggle />
      </header>
      <Outlet />
    </div>
  );
}
