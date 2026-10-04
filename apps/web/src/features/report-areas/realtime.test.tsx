import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { useReportAreaUpdatedRevision } from "./realtime";

const revisionMock = vi.hoisted(() => vi.fn());
vi.mock("../realtime/RealtimeProvider", () => ({ useRealtimeRevision: revisionMock }));

function Probe() { return <output>{useReportAreaUpdatedRevision()}</output>; }

describe("report area realtime invalidation", () => {
  it("uses the shared report-area revision instead of opening another SSE connection", () => {
    revisionMock.mockReturnValue(4);
    render(<Probe />);
    expect(screen.getByText("4")).toBeInTheDocument();
    expect(revisionMock).toHaveBeenCalledWith(["report-area.updated"]);
  });
});
