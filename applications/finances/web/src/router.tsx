import { createRootRoute, createRoute, createRouter } from "@tanstack/react-router";

import { RootLayout } from "@/components/RootLayout";
import { HomePage } from "@/routes/HomePage";
import { UsersPage } from "@/routes/UsersPage";

// `users` holds the comma-separated IDs of whoever is selected in the
// navbar's user filter (see @/lib/selectedUsers) - lives on the root route
// so every page shares the same selection via the URL, and it survives
// reload/sharing a link the way a client-only store wouldn't.
const rootRoute = createRootRoute({
  validateSearch: (search: Record<string, unknown>): { users?: string } => ({
    users: typeof search.users === "string" ? search.users : undefined,
  }),
  component: RootLayout,
});

const indexRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/",
  component: HomePage,
});

const usersRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/users",
  component: UsersPage,
});

const routeTree = rootRoute.addChildren([indexRoute, usersRoute]);

export const router = createRouter({ routeTree });

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}
