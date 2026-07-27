import { createRootRoute, createRoute, createRouter } from "@tanstack/react-router";

import { RootLayout } from "@/components/RootLayout";
import { TripsPage } from "@/routes/TripsPage";
import { TripDetailPage } from "@/routes/TripDetailPage";

const rootRoute = createRootRoute({
  component: RootLayout,
});

const indexRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/",
  component: TripsPage,
});

const tripDetailRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/trips/$tripId",
  component: TripDetailPage,
});

const routeTree = rootRoute.addChildren([indexRoute, tripDetailRoute]);

export const router = createRouter({ routeTree });

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}
