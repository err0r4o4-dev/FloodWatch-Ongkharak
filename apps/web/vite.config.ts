import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    proxy: {
      "/api": {
        target: process.env.VITE_API_PROXY_TARGET || "http://127.0.0.1:8080",
        changeOrigin: false
      }
    },
    watch: {
      usePolling: process.env.VITE_USE_POLLING === "true",
      interval: 300
    }
  },
  test: {
    environment: "jsdom",
    globals: true,
    setupFiles: "./src/test/setup.ts"
  }
});
