import { act, render, screen, waitFor } from "@testing-library/react";
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

afterEach(() => { detailMock.mockReset(); reportsMock.mockReset(); vi.useRealTimers(); });

describe("report area details", () => {
  it("shows segment identity and archived history while prefilling the explicit segment", async () => {
    const segmentArea = {
      ...area,
      id: "segment-area-1",
      name: "ถนนทดสอบ",
      radiusM: 0,
      areaType: "ROAD_SEGMENT" as const,
      geometry: { type: "LineString" as const, coordinates: [[101.01, 14.09], [101.02, 14.1]] as [number, number][] },
      roadSegmentId: "segment-1",
      segmentVersion: 3,
      segmentActive: false,
      startLabel: "สะพาน",
      endLabel: "ตลาด"
    };
    detailMock.mockResolvedValue({ area: segmentArea, timeline: [] });
    reportsMock.mockResolvedValue({ items: [], limit: 20, offset: 0, hasMore: false });
    render(<MemoryRouter><ReportAreaDetailsContent area={segmentArea} /></MemoryRouter>);

    expect(await screen.findByText("สะพาน–ตลาด")).toBeInTheDocument();
    expect(screen.getByText("รุ่นช่วงถนน 3")).toBeInTheDocument();
    expect(screen.getByText(/ถูกเก็บถาวรแล้ว/)).toBeInTheDocument();
    expect(screen.getByText(/ไม่ได้หมายความว่าน้ำท่วมหรือสภาพเดียวกันตลอดทั้งช่วงถนน/)).toBeInTheDocument();
    const update = screen.getByRole("link", { name: "แจ้งข้อมูลล่าสุดของพื้นที่นี้" });
    const url = new URL(update.getAttribute("href") ?? "", "http://localhost");
    expect(url.searchParams.get("roadSegmentId")).toBe("segment-1");
    expect(url.searchParams.get("lng")).toBe("101.02");
    expect(url.searchParams.get("lat")).toBe("14.1");
    expect(screen.getByRole("heading", { name: "ประวัติสถานการณ์น้ำในช่วงถนนนี้" })).toBeInTheDocument();
  });

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

  it("masks the previously fresh summary when a canonical refresh fails", async () => {
    detailMock.mockRejectedValue(new Error("area removed"));
    reportsMock.mockResolvedValue({ items: [], limit: 20, offset: 0, hasMore: false });
    render(<MemoryRouter><ReportAreaDetailsContent area={area} /></MemoryRouter>);

    await screen.findByRole("alert");
    expect(screen.getAllByText("ไม่ทราบ/ข้อมูลเก่า").length).toBeGreaterThan(0);
    expect(screen.queryByText("10–น้อยกว่า 30 ซม.")).not.toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: area.name })).not.toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "พื้นที่รายงานชุมชน" })).toBeInTheDocument();
    const update = screen.getByRole("link", { name: "แจ้งข้อมูลล่าสุดของพื้นที่นี้" });
    expect(new URL(update.getAttribute("href") ?? "", "http://localhost").searchParams.get("locationName")).toBeNull();
  });

  it("keeps historical depth relative to each window while showing the current falling summary", async () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-10-05T03:00:00Z"));
    const currentArea = {
      ...area, depthBand: "LOW" as const, previousDepthBand: "VERY_HIGH" as const, trend: "FALLING" as const,
      latestObservedAt: "2026-10-05T02:30:00Z", computedAt: "2026-10-05T03:00:00Z", windowStart: "2026-10-05T02:00:00Z", windowEnd: "2026-10-05T03:00:00Z"
    };
    const historical = { ...window, windowStart: "2026-10-05T00:00:00Z", windowEnd: "2026-10-05T01:00:00Z", depthBand: "VERY_HIGH" as const, latestObservedAt: "2026-10-05T00:30:00Z", computedAt: "2026-10-05T01:00:00Z" };
    const current = { ...window, windowStart: "2026-10-05T02:00:00Z", windowEnd: "2026-10-05T03:00:00Z", depthBand: "LOW" as const, trend: "FALLING" as const, latestObservedAt: "2026-10-05T02:30:00Z", computedAt: "2026-10-05T03:00:00Z" };
    detailMock.mockResolvedValue({ area: currentArea, timeline: [historical, current] });
    reportsMock.mockResolvedValue({ items: [], limit: 20, offset: 0, hasMore: false });
    render(<MemoryRouter><ReportAreaDetailsContent area={area} /></MemoryRouter>);

    await act(async () => { await Promise.resolve(); await Promise.resolve(); });
    expect(screen.getAllByText("มากกว่า 50 ซม.").length).toBeGreaterThan(0);
    expect(screen.getAllByText("น้อยกว่า 10 ซม.").length).toBeGreaterThan(0);
    expect(screen.getAllByText("ลดลง").length).toBeGreaterThan(0);
  });

  it("refreshes the detail and original reports every 60 seconds", async () => {
    vi.useFakeTimers();
    detailMock.mockResolvedValue({ area, timeline: [] });
    reportsMock.mockResolvedValue({ items: [], limit: 20, offset: 0, hasMore: false });
    render(<MemoryRouter><ReportAreaDetailsContent area={area} /></MemoryRouter>);
    await act(async () => { await Promise.resolve(); });
    const detailCalls = detailMock.mock.calls.length;
    const reportCalls = reportsMock.mock.calls.length;

    await act(async () => {
      vi.advanceTimersByTime(60_000);
      await Promise.resolve();
    });

    expect(detailMock.mock.calls.length).toBeGreaterThan(detailCalls);
    expect(reportsMock.mock.calls.length).toBeGreaterThan(reportCalls);
  });
});
