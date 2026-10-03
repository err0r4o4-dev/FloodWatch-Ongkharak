import { execFileSync } from "node:child_process";
import base from "../apps/web/vite.config";

export default {
  ...base,
  plugins: [{
    name: "verify-existing-map-baseline",
    enforce: "pre" as const,
    load(id: string) {
      if (id.replaceAll("\\", "/").endsWith("/src/features/map/MapPage.tsx")) {
        return execFileSync("git", ["show", "39a8067"], { encoding: "utf8" });
      }
    }
  }, ...base.plugins!]
};
