import { createRootRoute, createRoute, createRouter } from "@tanstack/react-router";

import { RootLayout } from "@/components/RootLayout";
import { HomePage } from "@/routes/HomePage";
import { UsersPage } from "@/routes/UsersPage";

const rootRoute = createRootRoute({
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
