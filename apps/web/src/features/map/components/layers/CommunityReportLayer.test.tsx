import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { CommunityReportLayer } from "./CommunityReportLayer";
import type { ReportArea } from "../../../report-areas/types";

vi.mock("react-leaflet", () => ({
  Circle: () => <div data-testid="radius-circle" />,
  Polyline: ({ eventHandlers }: { eventHandlers: { click: () => void } }) => <button type="button" onClick={eventHandlers.click}>ช่วงถนน</button>
}));
vi.mock("../MapMarker", () => ({ MapMarker: () => <div data-testid="keyboard-marker" /> }));

describe("community road-segment layer", () => {
  it("draws a clickable road extent instead of a radius and retains a keyboard marker", () => {
    const area: ReportArea = {
      id: "segment", name: "Synthetic road", location: { type: "Point", coordinates: [101, 14] }, radiusM: 0,
      areaType: "ROAD_SEGMENT", geometry: { type: "LineString", coordinates: [[101, 14], [101.001, 14]] },
      roadSegmentId: "segment", segmentVersion: 1, segmentActive: true, startLabel: "A", endLabel: "B",
      sourceType: "COMMUNITY", depthBand: "UNKNOWN", previousDepthBand: "UNKNOWN", trend: "UNKNOWN", reportedTrend: "UNKNOWN",
      freshness: "UNKNOWN", summaryReason: "NO_RECENT_REPORTS", latestObservedAt: null, computedAt: "2026-10-05T00:00:00Z",
      windowStart: "2026-10-04T23:00:00Z", windowEnd: "2026-10-05T00:00:00Z", reportCount: 0, sampleCount: 0, totalReportCount: 0, reportIds: []
    };
    const onSelect = vi.fn();
    render(<CommunityReportLayer items={[area]} onSelect={onSelect} />);
    expect(screen.queryByTestId("radius-circle")).not.toBeInTheDocument();
    expect(screen.getByTestId("keyboard-marker")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "ช่วงถนน" }));
    expect(onSelect).toHaveBeenCalledWith({ layer: "reports", feature: area }, area.location.coordinates);
  });
});
