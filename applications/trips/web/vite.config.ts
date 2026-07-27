import { fileURLToPath, URL } from "node:url";

import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

// Connect's HTTP paths are all under /<package>.<Service>/, so proxying
// everything under /trips.* to the local Go server (see cmd/trips/main.go,
// PORT defaults to 8080) lets `npm run dev` and `go run ./cmd/trips` run
// side by side against the same browser origin - matching the single-binary
// same-origin setup this app runs as in production, so no CORS handling is
// ever needed.
export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
    },
  },
  server: {
    proxy: {
      "/trips.v1.TripService": "http://localhost:8080",
      "/trips.v1.FlightService": "http://localhost:8080",
      "/trips.v1.AccommodationService": "http://localhost:8080",
    },
  },
});
