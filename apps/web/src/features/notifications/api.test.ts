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
  it("loads the authenticated inbox page with bounded query parameters", async () => {
    const payload = {
      items: [{
        id: "notification-1",
        alertId: "alert-1",
        title: "ฝนตกหนัก",
        message: "โปรดติดตามสถานการณ์",
        severity: "WARNING",
        sourceType: "OFFICIAL",
        createdAt: "2026-10-04T08:00:00Z",
        readAt: null
      }],
      pagination: { total: 1, limit: 10, offset: 0 }
    };
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify(payload), { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);
    apiClient.setAccessToken("synthetic-test-token");

    await expect(notificationApi.list({ limit: 10, offset: 0 })).resolves.toEqual(payload);

    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(url).toBe("/api/v1/notifications?limit=10&offset=0");
    expect(init.method).toBeUndefined();
    expect(new Headers(init.headers).get("Authorization")).toBe("Bearer synthetic-test-token");
  });

  it("loads the authenticated unread count without inventing a fallback", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ count: 3 }), { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);
    apiClient.setAccessToken("synthetic-test-token");

    await expect(notificationApi.getUnreadCount()).resolves.toEqual({ count: 3 });
    expect(fetchMock).toHaveBeenCalledWith("/api/v1/notifications/unread-count", expect.objectContaining({ credentials: "same-origin" }));
  });

  it("marks one notification or the complete inbox as read", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(null, { status: 204 }));
    vi.stubGlobal("fetch", fetchMock);
    apiClient.setAccessToken("synthetic-test-token");

    await notificationApi.markRead("notification/with spaces");
    await notificationApi.markAllRead();

    expect(fetchMock.mock.calls[0]?.[0]).toBe("/api/v1/notifications/notification%2Fwith%20spaces/read");
    expect((fetchMock.mock.calls[0]?.[1] as RequestInit).method).toBe("PATCH");
    expect(fetchMock.mock.calls[1]?.[0]).toBe("/api/v1/notifications/read-all");
    expect((fetchMock.mock.calls[1]?.[1] as RequestInit).method).toBe("POST");
  });

  it("rejects malformed inbox responses", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify({ items: [], pagination: { total: -1, limit: 10, offset: 0 } }), { status: 200 })));
    await expect(notificationApi.list({ limit: 10, offset: 0 })).rejects.toThrow("Invalid notifications response");
  });

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
