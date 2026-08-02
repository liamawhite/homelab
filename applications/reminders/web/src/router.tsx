import { createRootRoute, createRoute, createRouter } from "@tanstack/react-router";

import { RootLayout } from "@/components/RootLayout";
import { RemindersPage } from "@/routes/RemindersPage";

const rootRoute = createRootRoute({
  component: RootLayout,
});

const indexRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/",
  component: RemindersPage,
});

const routeTree = rootRoute.addChildren([indexRoute]);

export const router = createRouter({ routeTree });

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}
