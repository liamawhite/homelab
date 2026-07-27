import { Outlet } from "@tanstack/react-router";

import { Navbar } from "@/components/Navbar";

export function RootLayout() {
  return (
    <div className="flex min-h-screen flex-col">
      <Navbar />
      <Outlet />
    </div>
  );
}
