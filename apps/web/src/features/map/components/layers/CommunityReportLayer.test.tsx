import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { CommunityReport } from "../../../reports/types";
import type { MapSelectHandler, MapSelection, Position } from "../../types";
import { groupCommunityReports } from "../../report-clusters";
import { CommunityReportLayer } from "./CommunityReportLayer";

vi.mock("../MapMarker", () => ({
  MapMarker: ({ selection, position, onSelect }: { selection: Extract<MapSelection, { layer: "reports" }>; position: Position; onSelect: MapSelectHandler }) =>
    <button type="button" onClick={() => onSelect(selection, position)}>{selection.feature.reports.length} reports</button>
}));

describe("grouped community report layer", () => {
  it("draws one selectable pin for eight nearby reports instead of road extents", () => {
    const originals: CommunityReport[] = Array.from({ length: 8 }, (_, index) => ({
      id: `report-${index}`, type: "FLOOD", description: "Synthetic report",
      location: { type: "Point", coordinates: [101 + index * 0.0001, 14] }, estimatedWaterDepthM: null,
      status: "PENDING", verificationStatus: "PENDING", sourceType: "COMMUNITY",
      createdAt: "2026-10-05T00:00:00Z", updatedAt: "2026-10-05T00:00:00Z", verifiedAt: null, media: []
    }));
    const groups = groupCommunityReports(originals);
    const onSelect = vi.fn();
    const { container } = render(<CommunityReportLayer items={groups} onSelect={onSelect} />);
    expect(screen.getAllByRole("button")).toHaveLength(1);
    expect(container.querySelector("svg")).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "8 reports" }));
    expect(onSelect).toHaveBeenCalledWith({ layer: "reports", feature: groups[0] }, groups[0].location.coordinates);
  });

  it("creates no pins when there are no published reports", () => {
    render(<CommunityReportLayer items={[]} onSelect={vi.fn()} />);
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
  });
});
