import { describe, expect, it } from "vitest";
import { displayDepthBand, displayReportAreaTrend, effectiveReportAreaFreshness, maskReportArea } from "./presentation";

describe("report area presentation freshness", () => {
  it("expires a fresh area after one hour without a new observation", () => {
    const now = Date.parse("2026-10-02T03:00:00Z");
    expect(effectiveReportAreaFreshness("FRESH", "2026-10-02T02:01:00Z", now)).toBe("FRESH");
    expect(effectiveReportAreaFreshness("FRESH", "2026-10-02T01:59:59Z", now)).toBe("UNKNOWN");
    expect(effectiveReportAreaFreshness("FRESH", "2026-10-02T02:00:00Z", now)).toBe("UNKNOWN");
    expect(effectiveReportAreaFreshness("FRESH", "2026-10-02T03:01:00Z", now)).toBe("UNKNOWN");
    expect(displayDepthBand("HIGH", "FRESH", "2026-10-02T01:59:59Z", now)).toBe("UNKNOWN");
    expect(displayReportAreaTrend("FALLING", "FRESH", "2026-10-02T01:59:59Z", now)).toBe("UNKNOWN");
  });

  it("removes cached public identity and report counts from an unavailable summary", () => {
    const area = {
      id: "area-1", name: "Cached area", location: { type: "Point" as const, coordinates: [101, 14] as [number, number] }, radiusM: 150,
      sourceType: "COMMUNITY" as const, depthBand: "HIGH" as const, previousDepthBand: "LOW" as const, trend: "FALLING" as const, reportedTrend: "RISING" as const,
      freshness: "FRESH" as const, summaryReason: "SUMMARIZED" as const, latestObservedAt: "2026-10-02T02:00:00Z", computedAt: "2026-10-02T02:00:00Z",
      windowStart: "2026-10-01T02:00:00Z", windowEnd: "2026-10-02T02:00:00Z", reportCount: 3, sampleCount: 2, totalReportCount: 9, reportIds: ["report-1"]
    };
    const masked = maskReportArea(area);
    expect(masked.name).toBe("");
    expect(masked.totalReportCount).toBe(0);
    expect(masked.reportIds).toEqual([]);
  });
});
