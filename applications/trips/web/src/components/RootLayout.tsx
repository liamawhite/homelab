import { Link, Outlet } from "@tanstack/react-router";

import { ThemeToggle } from "@/components/ThemeToggle";

export function RootLayout() {
  return (
    <div className="flex min-h-screen flex-col">
      <header className="flex items-center justify-between border-b px-4 py-3">
        <Link to="/" className="text-sm font-semibold">
          Trips
        </Link>
        <ThemeToggle />
      </header>
      <Outlet />
    </div>
  );
}
