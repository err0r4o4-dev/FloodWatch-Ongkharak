import { afterEach, describe, expect, it, vi } from "vitest";
import { fetchReportAreaDetail, fetchReportAreaReports, fetchReportAreas } from "./api";

const area = {
  id: "area-1",
  name: "ชุมชนคลองทดสอบ",
  location: { type: "Point", coordinates: [101.02, 14.1] },
  radiusM: 150,
  sourceType: "COMMUNITY",
  depthBand: "MODERATE",
  previousDepthBand: "LOW",
  trend: "RISING",
  reportedTrend: "UNKNOWN",
  freshness: "FRESH",
  summaryReason: "SUMMARIZED",
  latestObservedAt: "2026-10-02T01:00:00Z",
  computedAt: "2026-10-02T02:00:00Z",
  windowStart: "2026-10-01T02:00:00Z",
  windowEnd: "2026-10-02T02:00:00Z",
  reportCount: 4,
  sampleCount: 3,
  totalReportCount: 5,
  reportIds: ["report-1", "report-2"]
};

const timeline = [
  { ...area, name: undefined, location: undefined, radiusM: undefined, sourceType: undefined, totalReportCount: undefined },
  { ...area, windowStart: "2026-10-03T02:00:00Z", windowEnd: "2026-10-04T02:00:00Z", name: undefined, location: undefined, radiusM: undefined, sourceType: undefined, totalReportCount: undefined }
];

const report = {
  id: "report-1", type: "FLOOD", description: "น้ำท่วมหน้าบ้าน", locationName: "คลองทดสอบ",
  location: { type: "Point", coordinates: [101.02, 14.1] }, estimatedWaterDepthM: 0.25, observedAt: null, waterTrend: "RISING",
  status: "VERIFIED", verificationStatus: "VERIFIED", publicationStatus: "PUBLISHED", sourceType: "COMMUNITY",
  createdAt: "2026-10-02T00:00:00Z", updatedAt: "2026-10-02T00:30:00Z", verifiedAt: "2026-10-02T01:00:00Z", media: []
};

afterEach(() => { vi.unstubAllGlobals(); vi.restoreAllMocks(); });

describe("report areas API boundary", () => {
  it("requests bounded aggregated areas and validates the flat area contract", async () => {
    const request = vi.fn().mockResolvedValue({ ok: true, json: async () => ({ items: [area], limit: 100, offset: 0 }) });
    vi.stubGlobal("fetch", request);

    const page = await fetchReportAreas({ west: 100.9, south: 14, east: 101.2, north: 14.3 }, new AbortController().signal);

    expect(page.items[0]).toEqual(area);
    const url = new URL(request.mock.calls[0][0], "http://localhost");
    expect(url.pathname).toBe("/api/v1/report-areas");
    expect(url.searchParams.get("bbox")).toBe("100.9,14,101.2,14.3");
    expect(url.searchParams.get("limit")).toBe("100");
    expect(url.searchParams.get("offset")).toBe("0");
  });

  it("rejects malformed area evidence instead of drawing an unsafe feature", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: true, json: async () => ({ items: [{ ...area, radiusM: -1 }], limit: 100, offset: 0 }) }));
    await expect(fetchReportAreas({ west: 100, south: 14, east: 102, north: 15 }, new AbortController().signal)).rejects.toThrow("report area");

    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: true, json: async () => ({ items: [{ ...area, location: { type: "Point", coordinates: [181, 14] } }], limit: 100, offset: 0 }) }));
    await expect(fetchReportAreas({ west: 100, south: 14, east: 102, north: 15 }, new AbortController().signal)).rejects.toThrow("report area");
  });

  it("validates detail timeline windows and retains original report fields", async () => {
    const request = vi.fn()
      .mockResolvedValueOnce({ ok: true, json: async () => ({ area, timeline }) })
      .mockResolvedValueOnce({ ok: true, json: async () => ({ items: [report], limit: 20, offset: 0 }) });
    vi.stubGlobal("fetch", request);

    const detail = await fetchReportAreaDetail("area/one", new AbortController().signal);
    const reports = await fetchReportAreaReports("area/one", 20, 0, new AbortController().signal);

    expect(detail.timeline).toHaveLength(2);
    expect(reports.items[0]).toEqual(report);
    expect(new URL(request.mock.calls[0][0], "http://localhost").pathname).toBe("/api/v1/report-areas/area%2Fone");
    expect(new URL(request.mock.calls[1][0], "http://localhost").pathname).toBe("/api/v1/report-areas/area%2Fone/reports");
  });

  it("rejects negative original report depths", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: true, json: async () => ({ items: [{ ...report, estimatedWaterDepthM: -0.1 }], limit: 20, offset: 0 }) }));
    await expect(fetchReportAreaReports("area-1", 20, 0, new AbortController().signal)).rejects.toThrow("original report");
  });
});
