import { createRootRoute, createRoute, createRouter } from "@tanstack/react-router";

import { RootLayout } from "@/components/RootLayout";
import { HomePage } from "@/routes/HomePage";
import { UsersPage } from "@/routes/UsersPage";
import { ExercisesPage } from "@/routes/ExercisesPage";
import { TrainingMaxesPage } from "@/routes/TrainingMaxesPage";

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

const exercisesRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/exercises",
  component: ExercisesPage,
});

const trainingMaxesRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/training-maxes",
  component: TrainingMaxesPage,
});

const routeTree = rootRoute.addChildren([indexRoute, usersRoute, exercisesRoute, trainingMaxesRoute]);

export const router = createRouter({ routeTree });

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}
