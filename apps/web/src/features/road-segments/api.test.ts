import { afterEach, describe, expect, it, vi } from "vitest";
import { fetchRoadSegmentCandidates, fetchRoadSegments, listRoadOptionPage, listRoadOptions } from "./api";

const segment = {
  id: "segment-1", roadId: "road-1", name: "ถนนทดสอบ", startLabel: "ต้นทาง", endLabel: "ปลายทาง",
  geometry: { type: "LineString", coordinates: [[101, 14], [101.1, 14.1]] },
  location: { type: "Point", coordinates: [101.05, 14.05] }, version: 1, supersedesId: null, active: true, createdAt: "2026-10-02T00:00:00Z"
};

afterEach(() => { vi.unstubAllGlobals(); vi.restoreAllMocks(); });

describe("road segment API boundary", () => {
  it("requests bounded segment lists and validates LineString geometry", async () => {
    const request = vi.fn().mockResolvedValue({ ok: true, json: async () => ({ items: [segment], limit: 100, offset: 0 }) });
    vi.stubGlobal("fetch", request);
    const page = await fetchRoadSegments({ west: 100, south: 14, east: 102, north: 15 }, 100, 0, new AbortController().signal);
    expect(page.items[0]).toEqual(segment);
    expect(new URL(request.mock.calls[0][0], "http://localhost").search).toContain("bbox=100%2C14%2C102%2C15");
  });

  it("rejects malformed segment geometry instead of allowing a guessed road", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: true, json: async () => ({ items: [{ ...segment, geometry: { type: "MultiLineString", coordinates: [] } }], limit: 100, offset: 0 }) }));
    await expect(fetchRoadSegments(null, 100, 0, new AbortController().signal)).rejects.toThrow("road segment");
  });

  it("loads candidate assignment status with the matching distance limit", async () => {
    const request = vi.fn().mockResolvedValue({ ok: true, json: async () => ({ items: [{ ...segment, distanceM: 5 }], assignmentStatus: "SINGLE_CANDIDATE", matchingDistanceM: 30 }) });
    vi.stubGlobal("fetch", request);
    const result = await fetchRoadSegmentCandidates(101.05, 14.05, new AbortController().signal);
    expect(result.assignmentStatus).toBe("SINGLE_CANDIDATE");
    const url = new URL(request.mock.calls[0][0], "http://localhost");
    expect(url.searchParams.get("longitude")).toBe("101.05");
    expect(url.searchParams.get("latitude")).toBe("14.05");
  });

  it("loads road options by page and exposes an explicit more-results state", async () => {
    const request = vi.fn().mockResolvedValue({ ok: true, json: async () => ({ items: [{ id: "road-101", name: "Imported road", code: "OSM-WAY-101" }], limit: 1, offset: 100 }) });
    vi.stubGlobal("fetch", request);

    await expect(listRoadOptionPage(1, 100, new AbortController().signal)).resolves.toEqual({
      items: [{ id: "road-101", name: "Imported road", code: "OSM-WAY-101" }],
      limit: 1,
      offset: 100,
      hasMore: true
    });
    const url = new URL(request.mock.calls[0][0], "http://localhost");
    expect(url.searchParams.get("limit")).toBe("1");
    expect(url.searchParams.get("offset")).toBe("100");
  });

  it("keeps the legacy road-option list helper returning only items", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: true, json: async () => ({ items: [{ id: "road-1", name: "Road one" }], limit: 100, offset: 0 }) }));

    await expect(listRoadOptions()).resolves.toEqual([{ id: "road-1", name: "Road one" }]);
  });
});
