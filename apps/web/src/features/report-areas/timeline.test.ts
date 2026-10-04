import { describe, expect, it } from "vitest";
import { findTimelineGaps } from "./timeline";
import type { ReportAreaTimelineWindow } from "./types";

const windowFor = (windowStart: string, windowEnd: string): ReportAreaTimelineWindow => ({
  windowStart, windowEnd, depthBand: "LOW", previousDepthBand: "UNKNOWN", trend: "STABLE", reportedTrend: "UNKNOWN",
  freshness: "FRESH", summaryReason: "SUMMARIZED", latestObservedAt: windowEnd, computedAt: windowEnd,
  reportCount: 1, sampleCount: 1, reportIds: ["report-1"]
});

describe("report area timeline", () => {
  it("identifies gaps without inventing a status for the missing interval", () => {
    expect(findTimelineGaps([
      windowFor("2026-10-01T00:00:00Z", "2026-10-01T01:00:00Z"),
      windowFor("2026-10-01T03:00:00Z", "2026-10-01T04:00:00Z")
    ])).toEqual([{ from: "2026-10-01T01:00:00Z", to: "2026-10-01T03:00:00Z" }]);
  });

  it("does not report gaps for overlapping or adjacent windows", () => {
    expect(findTimelineGaps([
      windowFor("2026-10-01T03:00:00Z", "2026-10-01T04:00:00Z"),
      windowFor("2026-10-01T00:00:00Z", "2026-10-01T03:00:00Z")
    ])).toEqual([]);
  });
});
