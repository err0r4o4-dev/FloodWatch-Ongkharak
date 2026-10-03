import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";

const alertsModule = "\0auth-alerts";

export default defineConfig({
  plugins: [
    react(),
    {
      name: "temporary-auth-alerts-module",
      resolveId(source) {
        return source.endsWith("/shared/alerts") ? alertsModule : undefined;
      },
      load(id) {
        if (id !== alertsModule) return undefined;
        return "export const notifySuccess = async () => {}; export const notifyError = async () => {}; export const notifyInfo = async () => {}; export const notifyWarning = async () => {};";
      }
    }
  ],
  test: {
    environment: "jsdom",
    globals: true,
    setupFiles: "./src/test/setup.ts"
  }
});
