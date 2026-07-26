import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { RouterProvider } from "@tanstack/react-router";

import "./index.css";
import { router } from "./router";
import { ThemeProvider } from "./lib/theme";
import { ActiveUserProvider } from "./lib/activeUser";

const queryClient = new QueryClient();

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <ThemeProvider>
      <QueryClientProvider client={queryClient}>
        <ActiveUserProvider>
          <RouterProvider router={router} />
        </ActiveUserProvider>
      </QueryClientProvider>
    </ThemeProvider>
  </StrictMode>,
);
