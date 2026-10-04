import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { describe, expect, it, vi } from "vitest";
import { CreateReportPage } from "./CreateReportPage";
import type { ReportLocation } from "./types";

vi.mock("./ReportForm", () => ({
  ReportForm: ({ initialLocation, initialLocationName }: { initialLocation?: ReportLocation; initialLocationName?: string }) => <div data-testid="report-prefill">{initialLocation ? JSON.stringify(initialLocation.coordinates) : "no selected location"}<span>{initialLocationName}</span></div>
}));

describe("area-update report prefill", () => {
  it("uses valid coordinates as an editable initial point and bounds the community label", () => {
    const name = "สถานที่".repeat(50);
    render(<MemoryRouter initialEntries={[`/reports/new?lng=101.01&lat=14.03&locationName=${encodeURIComponent(name)}`]}><CreateReportPage /></MemoryRouter>);
    expect(screen.getByTestId("report-prefill")).toHaveTextContent("[101.01,14.03]");
    expect(screen.getByTestId("report-prefill").querySelector("span")?.textContent).toBe(Array.from(name).slice(0, 200).join(""));
    expect(screen.getByText(/กรุณายืนยันหรือปรับหมุด/)).toBeInTheDocument();
  });

  it.each(["lng=NaN&lat=14", "lng=181&lat=14", "lng=101&lat=-91", "lng=&lat=14", "lng=101"])("does not preselect invalid query coordinates: %s", (query) => {
    render(<MemoryRouter initialEntries={[`/reports/new?${query}&locationName=untrusted`]}><CreateReportPage /></MemoryRouter>);
    expect(screen.getByTestId("report-prefill")).toHaveTextContent("no selected location");
    expect(screen.queryByText("untrusted")).not.toBeInTheDocument();
  });
});
