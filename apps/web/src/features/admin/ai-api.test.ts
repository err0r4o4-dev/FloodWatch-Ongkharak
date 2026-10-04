import { afterEach, describe, expect, it, vi } from "vitest";
import { apiClient } from "../auth/api";
import { aiApi } from "./ai-api";

afterEach(() => {
  apiClient.clearAccessToken();
  vi.unstubAllGlobals();
});

const evidence = {
  id: "evidence-1",
  reportId: "report-1",
  mediaId: "media-1",
  sourceType: "AI",
  modelVersion: "vision-1",
  flood: { severity: "WARNING", water_depth_m: 0, confidence: 0, bounding_box: [0, 0, 1, 1] },
  road: null,
  observedAt: "2026-10-04T08:00:00Z",
  receivedAt: "2026-10-04T08:01:00Z",
  limitations: ["ภาพมุมเดียว"],
  status: "UNVERIFIED"
} as const;

describe("aiApi", () => {
  it("loads only unverified evidence with bounded pagination and auth", async () => {
    const payload = { items: [evidence], pagination: { total: 1, limit: 20, offset: 0 } };
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify(payload), { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);
    apiClient.setAccessToken("admin-token");

    await expect(aiApi.list({ limit: 20, offset: 0 })).resolves.toEqual(payload);

    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(url).toBe("/api/v1/admin/ai-evidence?status=UNVERIFIED&limit=20&offset=0");
    expect(new Headers(init.headers).get("Authorization")).toBe("Bearer admin-token");
  });

  it("loads a detail record and bounded actual-road options", async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(new Response(JSON.stringify({ evidence }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ items: [{ id: "road-1", name: "ถนนองครักษ์" }] }), { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);
    apiClient.setAccessToken("admin-token");

    await expect(aiApi.get("evidence/1")).resolves.toEqual(evidence);
    await expect(aiApi.listRoads({ limit: 20, offset: 0 })).resolves.toEqual([{ id: "road-1", name: "ถนนองครักษ์" }]);
    expect(fetchMock.mock.calls[0]?.[0]).toBe("/api/v1/admin/ai-evidence/evidence%2F1");
    expect(fetchMock.mock.calls[1]?.[0]).toBe("/api/v1/roads?limit=20&offset=0");
  });

  it("sends explicit verification and rejection payloads", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(null, { status: 204 }));
    vi.stubGlobal("fetch", fetchMock);
    apiClient.setAccessToken("admin-token");

    await aiApi.verify("evidence-1", { roadId: "road-1" });
    await aiApi.reject("evidence-1", { reason: "หลักฐานไม่ชัดเจน" });

    expect(fetchMock.mock.calls[0]?.[0]).toBe("/api/v1/admin/ai-evidence/evidence-1/verify");
    expect((fetchMock.mock.calls[0]?.[1] as RequestInit).method).toBe("POST");
    expect(JSON.parse(String((fetchMock.mock.calls[0]?.[1] as RequestInit).body))).toEqual({ roadId: "road-1" });
    expect(fetchMock.mock.calls[1]?.[0]).toBe("/api/v1/admin/ai-evidence/evidence-1/reject");
    expect(JSON.parse(String((fetchMock.mock.calls[1]?.[1] as RequestInit).body))).toEqual({ reason: "หลักฐานไม่ชัดเจน" });
  });

  it("rejects malformed evidence instead of rendering fabricated review data", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify({ evidence: { ...evidence, flood: { ...evidence.flood, confidence: 1.1 } } }), { status: 200 })));
    await expect(aiApi.get("evidence-1")).rejects.toThrow("Invalid AI evidence response");
  });

  it("accepts nullable bounding boxes and known unknown result labels", async () => {
    const payload = {
      ...evidence,
      flood: { ...evidence.flood, severity: "UNKNOWN", bounding_box: null },
      road: { status: "UNKNOWN", confidence: 0, bounding_box: null }
    };
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify({ evidence: payload }), { status: 200 })));

    await expect(aiApi.get("evidence-1")).resolves.toEqual(payload);
  });

  it.each([
    ["an unknown flood severity", { ...evidence, flood: { ...evidence.flood, severity: "BOGUS" } }],
    ["an unknown road status", { ...evidence, road: { status: "BOGUS", confidence: 0.5, bounding_box: null } }],
    ["a bounding box with the wrong length", { ...evidence, flood: { ...evidence.flood, bounding_box: [0, 0, 1] } }],
    ["a bounding box outside normalized coordinates", { ...evidence, flood: { ...evidence.flood, bounding_box: [0, 0, 2, 1] } }],
    ["a reversed bounding box", { ...evidence, flood: { ...evidence.flood, bounding_box: [0.8, 0.2, 0.1, 0.9] } }]
  ])("rejects %s", async (_description, payload) => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify({ evidence: payload }), { status: 200 })));

    await expect(aiApi.get("evidence-1")).rejects.toThrow("Invalid AI evidence response");
  });
});
