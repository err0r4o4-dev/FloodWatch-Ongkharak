import { afterEach, describe, expect, it, vi } from "vitest";
import { fetchMapLayer, formatBbox } from "./api";
import { effectiveRoadStatus, parseRoutePoint, validGeometry } from "./utils";

const source = { id: "source-1", code: "sample", name: "หน่วยงานทดสอบ", sourceType: "OFFICIAL" };
const station = {
  id: "station-1", name: "สถานีทดสอบ", source, active: true,
  location: { type: "Point", coordinates: [101.02, 14.1] }
};
const reportArea = {
  id: "area-1", name: "พื้นที่รายงานทดสอบ", location: { type: "Point", coordinates: [101.02, 14.1] }, radiusM: 150,
  sourceType: "COMMUNITY", depthBand: "MODERATE", previousDepthBand: "LOW", trend: "RISING", reportedTrend: "UNKNOWN",
  freshness: "FRESH", summaryReason: "SUMMARIZED", latestObservedAt: "2026-10-02T01:00:00Z",
  computedAt: "2026-10-02T02:00:00Z", windowStart: "2026-10-01T02:00:00Z", windowEnd: "2026-10-02T02:00:00Z",
  reportCount: 4, sampleCount: 3, totalReportCount: 5, reportIds: ["report-1"]
};

afterEach(() => { vi.unstubAllGlobals(); vi.restoreAllMocks(); });

describe("map API boundary", () => {
  it("sends bounded WGS84 bbox in longitude,latitude order", async () => {
    const request = vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve({ items: [station], limit: 100, offset: 0 }) });
    vi.stubGlobal("fetch", request);
    const bounds = { west: 100.9, south: 14, east: 101.2, north: 14.3 };
    const page = await fetchMapLayer("water", bounds, new AbortController().signal);
    expect(page.items).toHaveLength(1);
    expect(page.hasMore).toBe(false);
    expect(formatBbox(bounds)).toBe("100.9,14,101.2,14.3");
    const url = new URL(request.mock.calls[0][0], "http://localhost");
    expect(url.pathname).toBe("/api/v1/water-stations");
    expect(url.searchParams.get("bbox")).toBe("100.9,14,101.2,14.3");
    expect(url.searchParams.get("limit")).toBe("100");
    expect(url.searchParams.get("offset")).toBe("0");
  });

  it("requests aggregated COMMUNITY report areas within the viewport", async () => {
    const request = vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve({ items: [reportArea], limit: 100, offset: 0 }) });
    vi.stubGlobal("fetch", request);

    await expect(fetchMapLayer("reports", { west: 100.9, south: 14, east: 101.2, north: 14.3 }, new AbortController().signal)).resolves.toEqual({ items: [reportArea], hasMore: false });
    const url = new URL(request.mock.calls[0][0], "http://localhost");
    expect(url.pathname).toBe("/api/v1/report-areas");
    expect(url.searchParams.get("bbox")).toBe("100.9,14,101.2,14.3");
    expect(url.searchParams.get("limit")).toBe("100");
  });

  it("rejects malformed geometry instead of drawing misleading features", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve({ items: [{ ...station, location: { type: "Point", coordinates: [14.1, 201] } }], limit: 100, offset: 0 }) }));
    await expect(fetchMapLayer("water", { west: 100, south: 14, east: 102, north: 15 }, new AbortController().signal)).rejects.toThrow("geometry");
    expect(validGeometry({ type: "Point", coordinates: [101.02, 14.1] })).toBe(true);
    expect(validGeometry({ type: "Point", coordinates: [14.1, 201] })).toBe(false);
  });

  it("rejects missing provenance instead of crashing the selected-feature panel", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve({ items: [{ ...station, source: null }], limit: 100, offset: 0 }) }));
    await expect(fetchMapLayer("water", { west: 100, south: 14, east: 102, north: 15 }, new AbortController().signal)).rejects.toThrow("evidence");
  });

  it("marks a full page as truncated rather than fetching an unbounded database", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve({ items: Array(100).fill(station), limit: 100, offset: 0 }) }));
    const page = await fetchMapLayer("water", { west: 100, south: 14, east: 102, north: 15 }, new AbortController().signal);
    expect(page.hasMore).toBe(true);
  });

  it("never treats stale, unverified or missing road conditions as passable", () => {
    const condition = { status: "PASSABLE" as const, freshness: "FRESH" as const, verificationStatus: "VERIFIED" as const };
    expect(effectiveRoadStatus(condition)).toBe("PASSABLE");
    expect(effectiveRoadStatus({ ...condition, freshness: "STALE" })).toBe("UNKNOWN");
    expect(effectiveRoadStatus({ ...condition, verificationStatus: "UNVERIFIED" })).toBe("UNKNOWN");
    expect(effectiveRoadStatus(null)).toBe("UNKNOWN");
  });

  it("accepts only valid route-point coordinates and never creates a path", () => {
    expect(parseRoutePoint({ longitude: "101.02", latitude: "14.1" })).toEqual([101.02, 14.1]);
    expect(parseRoutePoint({ longitude: "201", latitude: "14.1" })).toBeNull();
    expect(parseRoutePoint({ longitude: "", latitude: "14.1" })).toBeNull();
  });
});
