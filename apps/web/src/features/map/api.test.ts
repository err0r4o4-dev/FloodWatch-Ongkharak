import { afterEach, describe, expect, it, vi } from "vitest";
import { fetchMapLayer, formatBbox } from "./api";
import { effectiveRoadStatus, parseRoutePoint, validGeometry } from "./utils";

const source = { id: "source-1", code: "sample", name: "หน่วยงานทดสอบ", sourceType: "OFFICIAL" };
const station = {
  id: "station-1", name: "สถานีทดสอบ", source, active: true,
  location: { type: "Point", coordinates: [101.02, 14.1] }
};
const publicReport = {
  id: "report-1", type: "FLOOD", description: "Synthetic report", locationName: "พื้นที่รายงานทดสอบ",
  location: { type: "Point", coordinates: [101.02, 14.1] }, sourceType: "COMMUNITY",
  estimatedWaterDepthM: null, status: "PENDING", verificationStatus: "PENDING", publicationStatus: "PUBLISHED",
  createdAt: "2026-10-05T00:00:00Z", updatedAt: "2026-10-05T00:00:00Z", verifiedAt: null, media: []
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

  it("requests original COMMUNITY reports within the viewport and groups nearby positions", async () => {
    const request = vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve({ items: [publicReport, { ...publicReport, id: "report-2" }], limit: 100, offset: 0 }) });
    vi.stubGlobal("fetch", request);
    const page = await fetchMapLayer("reports", { west: 100.9, south: 14, east: 101.2, north: 14.3 }, new AbortController().signal);
    expect(page.items).toHaveLength(1);
    expect(page.items[0].reports).toHaveLength(2);
    expect(page.hasMore).toBe(false);
    const url = new URL(request.mock.calls[0][0], "http://localhost");
    expect(url.pathname).toBe("/api/v1/reports");
    expect(url.searchParams.get("bbox")).toBe("100.9,14,101.2,14.3");
    expect(url.searchParams.get("limit")).toBe("100");
  });

  it("does not create community pins for an empty original-report response", async () => {
    const request = vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve({ items: [], limit: 100, offset: 0 }) });
    vi.stubGlobal("fetch", request);
    await expect(fetchMapLayer("reports", { west: 100, south: 14, east: 102, north: 15 }, new AbortController().signal)).resolves.toEqual({ items: [], hasMore: false });
    expect(request).toHaveBeenCalledTimes(1);
  });

  it("marks a full report page as truncated even when it becomes just one grouped pin", async () => {
    const reports = Array.from({ length: 100 }, (_, index) => ({ ...publicReport, id: `report-${index}` }));
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve({ items: reports, limit: 100, offset: 0 }) }));
    const page = await fetchMapLayer("reports", { west: 100, south: 14, east: 102, north: 15 }, new AbortController().signal);
    expect(page.items).toHaveLength(1);
    expect(page.items[0].reports).toHaveLength(100);
    expect(page.hasMore).toBe(true);
  });

  it("rejects malformed original reports rather than hiding bad evidence", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve({ items: [{ ...publicReport, location: { type: "Point", coordinates: [201, 14] } }], limit: 100, offset: 0 }) }));
    await expect(fetchMapLayer("reports", { west: 100, south: 14, east: 102, north: 15 }, new AbortController().signal)).rejects.toThrow("Invalid reports API response");
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
