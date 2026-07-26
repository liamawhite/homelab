import { fileURLToPath, URL } from "node:url";

import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

// Connect's HTTP paths are all under /<package>.<Service>/, so proxying
// everything under /shopping.* to the local Go server (see
// cmd/shopping/main.go, PORT defaults to 8080) lets `npm run dev` and
// `go run ./cmd/shopping` run side by side against the same browser origin -
// matching the single-binary same-origin setup this app runs as in
// production, so no CORS handling is ever needed.
export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
    },
  },
  server: {
    proxy: {
      "/shopping.v1.ItemService": "http://localhost:8080",
      "/shopping.v1.LabelService": "http://localhost:8080",
    },
  },
});
