import { afterEach, describe, expect, it, vi } from "vitest";

afterEach(() => {
  vi.unstubAllEnvs();
  vi.resetModules();
});

describe("development API proxy", () => {
  it("forwards same-origin API requests to the host backend without stripping the path", async () => {
    vi.stubEnv("VITE_API_PROXY_TARGET", undefined);
    vi.resetModules();
    const { default: config } = await import("./vite.config");
    expect(config.server?.proxy?.["/api"]).toEqual({ target: "http://127.0.0.1:8080", changeOrigin: false });
  });

  it("uses the configured backend service inside Docker", async () => {
    vi.stubEnv("VITE_API_PROXY_TARGET", "http://api:8080");
    vi.resetModules();
    const { default: config } = await import("./vite.config");
    expect(config.server?.proxy?.["/api"]).toEqual({ target: "http://api:8080", changeOrigin: false });
  });
});
