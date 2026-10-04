import { render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ReportAreaDetailsContent } from "./ReportAreaDetails";

const { detailMock, reportsMock } = vi.hoisted(() => ({ detailMock: vi.fn(), reportsMock: vi.fn() }));
vi.mock("./api", () => ({ fetchReportAreaDetail: detailMock, fetchReportAreaReports: reportsMock }));

const area = {
  id: "area-1", name: "พื้นที่ชุมชนทดสอบ", location: { type: "Point" as const, coordinates: [101.02, 14.1] as [number, number] }, radiusM: 150,
  sourceType: "COMMUNITY" as const, depthBand: "MODERATE" as const, previousDepthBand: "LOW" as const, trend: "STABLE" as const, reportedTrend: "UNKNOWN" as const,
  freshness: "FRESH" as const, summaryReason: "SUMMARIZED" as const, latestObservedAt: "2026-10-02T01:00:00Z", computedAt: "2026-10-02T02:00:00Z",
  windowStart: "2026-10-01T02:00:00Z", windowEnd: "2026-10-02T02:00:00Z", reportCount: 3, sampleCount: 2, totalReportCount: 4, reportIds: ["report-1"]
};
const window = { windowStart: "2026-10-01T02:00:00Z", windowEnd: "2026-10-01T03:00:00Z", depthBand: "LOW" as const, previousDepthBand: "UNKNOWN" as const, trend: "STABLE" as const, reportedTrend: "UNKNOWN" as const, freshness: "FRESH" as const, summaryReason: "SUMMARIZED" as const, latestObservedAt: "2026-10-01T03:00:00Z", computedAt: "2026-10-01T03:00:00Z", reportCount: 1, sampleCount: 1, reportIds: ["report-1"] };
const report = { id: "report-1", type: "FLOOD" as const, description: "น้ำท่วมหน้าบ้าน", locationName: "คลองทดสอบ", location: { type: "Point" as const, coordinates: [101.02, 14.1] as [number, number] }, estimatedWaterDepthM: 0.25, status: "VERIFIED" as const, verificationStatus: "VERIFIED" as const, publicationStatus: "PUBLISHED" as const, sourceType: "COMMUNITY" as const, createdAt: "2026-10-02T00:00:00Z", updatedAt: "2026-10-02T00:30:00Z", verifiedAt: "2026-10-02T01:00:00Z", media: [] };

afterEach(() => { detailMock.mockReset(); reportsMock.mockReset(); });

describe("report area details", () => {
  it("separates derived timeline summaries from original source reports and preserves update coordinates", async () => {
    detailMock.mockResolvedValue({ area, timeline: [window, { ...window, windowStart: "2026-10-01T05:00:00Z", windowEnd: "2026-10-01T06:00:00Z" }] });
    reportsMock.mockResolvedValue({ items: [report], limit: 20, offset: 0, hasMore: false });
    render(<MemoryRouter><ReportAreaDetailsContent area={area} /></MemoryRouter>);

    expect(await screen.findByText("มีช่วงเวลาที่ไม่มีข้อมูล 1 ช่วง · ไม่ได้สรุปสถานะในช่วงที่หายไป")).toBeInTheDocument();
    expect(screen.getByText(/เนื้อหาต้นฉบับ/)).toBeInTheDocument();
    const update = screen.getByRole("link", { name: "แจ้งข้อมูลล่าสุดของพื้นที่นี้" });
    const url = new URL(update.getAttribute("href") ?? "", "http://localhost");
    expect(url.pathname).toBe("/reports/new");
    expect(url.searchParams.get("areaId")).toBe("area-1");
    expect(url.searchParams.get("lng")).toBe("101.02");
    expect(url.searchParams.get("lat")).toBe("14.1");
    expect(url.searchParams.get("locationName")).toBe("พื้นที่ชุมชนทดสอบ");
    await waitFor(() => expect(reportsMock).toHaveBeenCalledWith("area-1", 20, 0, expect.any(AbortSignal)));
  });
});
