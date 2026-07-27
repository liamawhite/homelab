import { createRootRoute, createRoute, createRouter } from "@tanstack/react-router";

import { validateOpenSearch } from "@/lib/searchState";
import { RootLayout } from "@/components/RootLayout";
import { DashboardPage } from "@/routes/DashboardPage";
import { LightsPage } from "@/routes/LightsPage";
import { SwitchesPage } from "@/routes/SwitchesPage";
import { GroupsPage } from "@/routes/GroupsPage";
import { ScenesPage } from "@/routes/ScenesPage";
import { SchedulesPage } from "@/routes/SchedulesPage";
import { BridgesPage } from "@/routes/BridgesPage";

const rootRoute = createRootRoute({
  component: RootLayout,
});

const indexRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/",
  validateSearch: validateOpenSearch,
  component: DashboardPage,
});

const lightsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/lights",
  component: LightsPage,
});

const switchesRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/switches",
  validateSearch: validateOpenSearch,
  component: SwitchesPage,
});

const groupsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/groups",
  component: GroupsPage,
});

const scenesRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/scenes",
  component: ScenesPage,
});

const schedulesRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/schedules",
  component: SchedulesPage,
});

const bridgesRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/bridges",
  component: BridgesPage,
});

const routeTree = rootRoute.addChildren([
  indexRoute,
  lightsRoute,
  switchesRoute,
  groupsRoute,
  scenesRoute,
  schedulesRoute,
  bridgesRoute,
]);

export const router = createRouter({ routeTree });

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}
