import { describe, expect, it } from "vitest";
import type { CommunityReport } from "../reports/types";
import { groupCommunityReports } from "./report-clusters";

function report(id: string, longitude = 101, latitude = 14): CommunityReport {
  return {
    id, type: "FLOOD", description: "Synthetic report", locationName: "Synthetic location",
    location: { type: "Point", coordinates: [longitude, latitude] }, estimatedWaterDepthM: null,
    status: "PENDING", verificationStatus: "PENDING", publicationStatus: "PUBLISHED", sourceType: "COMMUNITY",
    createdAt: "2026-10-05T00:00:00Z", updatedAt: "2026-10-05T00:00:00Z", verifiedAt: null, media: []
  };
}

describe("geographic community report groups", () => {
  it("creates one group for eight nearby originals without needing road assignments", () => {
    const originals = Array.from({ length: 8 }, (_, index) => report(`report-${index}`, 101 + index * 0.0001));
    const groups = groupCommunityReports(originals);
    expect(groups).toHaveLength(1);
    expect(groups[0].reports).toHaveLength(8);
    expect(groups[0].location).toEqual(originals[0].location);
    expect(groups[0].sourceType).toBe("COMMUNITY");
    expect(groups[0]).not.toHaveProperty("roadSegmentId");
    expect(groups[0]).not.toHaveProperty("depthBand");
  });

  it("creates three pins when eight reports occupy three separate locations", () => {
    const originals = Array.from({ length: 8 }, (_, index) => report(`report-${index}`, 101 + Math.floor(index / 3) * 0.01));
    expect(groupCommunityReports(originals).map((group) => group.reports.length)).toEqual([3, 3, 2]);
  });

  it("creates no empty groups or fabricated reports", () => {
    expect(groupCommunityReports([])).toEqual([]);
    const original = report("only");
    expect(groupCommunityReports([original])[0].reports).toEqual([original]);
  });

  it("ignores road identity and groups only by the original coordinates", () => {
    const originals = [
      { ...report("a"), roadSegmentId: "road-one" },
      { ...report("b", 101.0005), roadSegmentId: "road-two" },
      { ...report("c", 101.01), roadSegmentId: "road-one" }
    ];
    expect(groupCommunityReports(originals).map((group) => group.reports.map((item) => item.id))).toEqual([["a", "b"], ["c"]]);
  });

  it("excludes removed reports and counts duplicate IDs only once", () => {
    const original = report("published");
    const removed = { ...report("removed"), status: "REJECTED" as const, verificationStatus: "REJECTED" as const, publicationStatus: "REMOVED" as const };
    expect(groupCommunityReports([original, original, removed])[0].reports).toEqual([original]);
    expect(groupCommunityReports([removed])).toEqual([]);
  });

  it("uses a 150 metre anchor radius without chaining distant reports together", () => {
    const metresToLatitude = (metres: number) => metres / 6_371_008.8 * 180 / Math.PI;
    const originals = [report("a"), report("b", 101, 14 + metresToLatitude(149)), report("c", 101, 14 + metresToLatitude(151))];
    expect(groupCommunityReports(originals).map((group) => group.reports.map((item) => item.id))).toEqual([["a", "b"], ["c"]]);
  });

  it("keeps group identities and membership deterministic when API order changes", () => {
    const originals = [report("c", 101.01), report("b", 101.0005), report("a")];
    expect(groupCommunityReports(originals)).toEqual(groupCommunityReports([...originals].reverse()));
    expect(originals.map((item) => item.id)).toEqual(["c", "b", "a"]);
  });

  it("retains old and unreviewed evidence without turning it into a safety status", () => {
    const original = { ...report("old"), observedAt: "2020-01-01T00:00:00Z", estimatedWaterDepthM: 0.1 };
    const group = groupCommunityReports([original])[0];
    expect(group.reports[0]).toBe(original);
    expect(group).not.toHaveProperty("status");
    expect(group).not.toHaveProperty("freshness");
  });
});
