import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { RouterProvider } from "@tanstack/react-router";

import "./index.css";
import { router } from "./router";
import { ThemeProvider } from "./lib/theme";
import { DisplayCurrencyProvider } from "./lib/displayCurrency";

const queryClient = new QueryClient();

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <ThemeProvider>
      <DisplayCurrencyProvider>
        <QueryClientProvider client={queryClient}>
          <RouterProvider router={router} />
        </QueryClientProvider>
      </DisplayCurrencyProvider>
    </ThemeProvider>
  </StrictMode>,
);
