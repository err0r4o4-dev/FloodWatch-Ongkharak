import { afterEach, describe, expect, it, vi } from "vitest";
import { apiClient } from "../auth/api";
import { notificationApi } from "./api";

afterEach(() => { apiClient.clearAccessToken(); vi.unstubAllGlobals(); });

function subscriptionFixture(): PushSubscription {
  return {
    endpoint: "https://push.example.test/device",
    expirationTime: null,
    options: { applicationServerKey: null, userVisibleOnly: true },
    getKey: () => null,
    toJSON: () => ({ endpoint: "https://push.example.test/device", keys: { p256dh: "public-key", auth: "test-auth" } }),
    unsubscribe: async () => true
  };
}

describe("notificationApi", () => {
  it("sends the account authorization when subscribing", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(null, { status: 204 }));
    vi.stubGlobal("fetch", fetchMock);
    apiClient.setAccessToken("synthetic-test-token");
    const subscription = subscriptionFixture();
    await notificationApi.subscribe(subscription);
    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(url).toBe("/api/v1/notifications/subscribe");
    expect(init.method).toBe("POST");
    expect(new Headers(init.headers).get("Authorization")).toBe("Bearer synthetic-test-token");
    expect(JSON.parse(String(init.body))).toEqual({ endpoint: "https://push.example.test/device", keys: { p256dh: "public-key", auth: "test-auth" } });
  });

  it("authorizes and encodes the exact subscription endpoint when unsubscribing", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(null, { status: 204 }));
    vi.stubGlobal("fetch", fetchMock);
    apiClient.setAccessToken("synthetic-test-token");
    await notificationApi.unsubscribe("https://push.example.test/device?key=a+b&part=1");
    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(new URL(url, "http://localhost").searchParams.get("endpoint")).toBe("https://push.example.test/device?key=a+b&part=1");
    expect(init.method).toBe("DELETE");
    expect(new Headers(init.headers).get("Authorization")).toBe("Bearer synthetic-test-token");
  });

  it("rejects an unavailable public key rather than pretending push is configured", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify({ publicKey: "" }), { status: 200 })));
    await expect(notificationApi.getPublicKey()).rejects.toThrow();
  });

  it("propagates server failure instead of returning subscription success", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(null, { status: 503 })));
    const subscription = subscriptionFixture();
    await expect(notificationApi.subscribe(subscription)).rejects.toThrow();
  });
});
