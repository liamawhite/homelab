import { createRouter } from "@tanstack/react-router";

import { Route } from "@/routes/ShoppingListPage";

export const router = createRouter({ routeTree: Route });

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}
