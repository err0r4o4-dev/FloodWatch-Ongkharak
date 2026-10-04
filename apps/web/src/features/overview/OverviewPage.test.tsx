import { fireEvent, render as renderUI, screen, within } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { afterEach, describe, expect, it, vi } from "vitest";
import { OverviewPage } from "./OverviewPage";
import { AppShell } from "../../app/AppShell";

function render(ui: Parameters<typeof renderUI>[0]) {
  return renderUI(<MemoryRouter initialEntries={["/overview"]}>{ui}</MemoryRouter>);
}

const source = (sourceType: string) => ({ id: "source-1", name: "หน่วยงานตัวอย่าง", code: "sample", sourceType });
const event = (overrides: Record<string, unknown> = {}) => ({
  id: "event-1", title: "ข้อมูลทดสอบ", source: source("OFFICIAL"), severity: "WARNING",
  effectiveSeverity: "WARNING", status: "ACTIVE", effectiveStatus: "ACTIVE",
  observedAt: "2026-10-02T00:00:00Z", freshness: "FRESH", verificationStatus: "VERIFIED", ...overrides
});

function mockCollections(collections: Record<string, unknown[]>) {
  const request = vi.fn((url: string) => Promise.resolve({
    ok: true, json: () => Promise.resolve({ items: collections[url.split("?")[0]] ?? [], limit: 20, offset: 0 })
  }));
  vi.stubGlobal("fetch", request);
  return request;
}

afterEach(() => { vi.unstubAllGlobals(); vi.restoreAllMocks(); });

describe("OverviewPage", () => {
  it("shows measured rainfall without inventing a 24-hour total or a trend", async () => {
    mockCollections({
      "/api/v1/water-stations": [{ id: "station-1", name: "สถานีฝนทดสอบ", source: source("OFFICIAL") }],
      "/api/v1/water-stations/station-1/readings": [{ source: source("OFFICIAL"), observedAt: "2026-10-02T00:00:00Z", waterLevelM: 1.2, flowRateM3s: null, rainfallMm: 12, freshness: "STALE", verificationStatus: "VERIFIED" }]
    });
    render(<OverviewPage />);
    const rainfall = await screen.findByRole("region", { name: "ปริมาณฝนที่รายงาน" });
    expect(await within(rainfall).findByText("12 มม.")).toBeInTheDocument();
    expect(within(rainfall).getByText("ข้อมูลเก่า")).toBeInTheDocument();
    expect(within(rainfall).getByText("ไม่ทราบสถานะ")).toBeInTheDocument();
    expect(within(rainfall).queryByText(/24 ชม\.|%/)).not.toBeInTheDocument();
    expect(screen.queryByText(/25°C/)).not.toBeInTheDocument();
  });

  it("shows only verified individual reports with a link to the selected report", async () => {
    const report = {
      id: "report-1", type: "FLOOD", description: "น้ำขังหน้าบ้านจากบุคคล", location: { type: "Point", coordinates: [101.1, 14.1] },
      estimatedWaterDepthM: null, status: "VERIFIED", verificationStatus: "VERIFIED", sourceType: "COMMUNITY",
      createdAt: "2026-10-02T00:00:00Z", updatedAt: "2026-10-02T00:00:00Z", verifiedAt: "2026-10-02T01:00:00Z", media: []
    };
    mockCollections({ "/api/v1/reports": [report, { ...report, id: "pending", description: "รายงานรอตรวจ", status: "PENDING" }, { ...report, id: "unverified", description: "หลักฐานยังไม่ยืนยัน", verificationStatus: "PENDING" }] });
    render(<OverviewPage />);
    const reports = await screen.findByRole("region", { name: "รายงานจากประชาชน (ล่าสุด)" });
    expect(await within(reports).findByRole("link", { name: /น้ำขังหน้าบ้านจากบุคคล/ })).toHaveAttribute("href", "/reports?report=report-1");
    expect(within(reports).getByText("รายงานชุมชน")).toBeInTheDocument();
    expect(within(reports).getByText("ไม่มีภาพประกอบ")).toBeInTheDocument();
    expect(within(reports).queryByText("รายงานรอตรวจ")).not.toBeInTheDocument();
    expect(within(reports).queryByText("หลักฐานยังไม่ยืนยัน")).not.toBeInTheDocument();
  });

  it("loads published alerts independently from flood events", async () => {
    mockCollections({ "/api/v1/alerts": [{ id: "alert-1", ruleId: "rule-1", severity: "WARNING", title: "ประกาศจริงจาก API", message: "ตรวจสอบพื้นที่ก่อนเดินทาง", sourceType: "OFFICIAL", resourceType: "water_station", resourceId: "station-1", triggeredAt: "2026-10-02T00:00:00Z", createdAt: "2026-10-02T00:00:00Z" }] });
    render(<OverviewPage />);
    expect(await screen.findByText("ประกาศจริงจาก API")).toBeInTheDocument();
    expect(screen.getByRole("region", { name: "ประกาศล่าสุด" })).toHaveTextContent("เตือนภัย");
    expect(screen.getByRole("region", { name: "เหตุการณ์น้ำที่รายงาน" })).toHaveTextContent("ยังไม่มีเหตุการณ์ในรายการ");
  });

  it("retries a failed report feed without hiding the available flood data", async () => {
    const request = vi.fn((url: string) => url.includes("/reports")
      ? Promise.reject(new Error("offline"))
      : Promise.resolve({ ok: true, json: () => Promise.resolve({ items: url.includes("flood-events") ? [event()] : [], limit: 20, offset: 0 }) }));
    vi.stubGlobal("fetch", request);
    render(<OverviewPage />);
    expect(await screen.findByText("ข้อมูลทดสอบ")).toBeInTheDocument();
    expect(screen.getByRole("region", { name: "รายงานจากประชาชน (ล่าสุด)" })).toHaveTextContent("ไม่สามารถโหลดรายงานจากประชาชนได้");
    fireEvent.click(screen.getByRole("button", { name: "ลองอีกครั้ง" }));
    expect(await screen.findByText("ข้อมูลทดสอบ")).toBeInTheDocument();
    expect(request.mock.calls.filter(([url]) => url.includes("/reports"))).toHaveLength(2);
  });

  it("keeps flood panels available when the independent weather panel fails", async () => {
    const request = vi.fn((url: string) => url === "/api/v1/weather/ongkharak"
      ? Promise.reject(new Error("weather offline"))
      : Promise.resolve({ ok: true, json: () => Promise.resolve({ items: url.includes("flood-events") ? [event()] : [], limit: 20, offset: 0 }) }));
    vi.stubGlobal("fetch", request);
    render(<AppShell><OverviewPage /></AppShell>);

    expect(await screen.findByText("ข้อมูลทดสอบ")).toBeInTheDocument();
    expect(screen.getByRole("region", { name: "สภาพอากาศองครักษ์" })).toHaveTextContent("ไม่สามารถโหลดข้อมูลสภาพอากาศได้");
    expect(request.mock.calls.filter(([url]) => url === "/api/v1/weather/ongkharak")).toHaveLength(1);
    expect(screen.getByRole("region", { name: "เหตุการณ์น้ำที่รายงาน" })).toHaveTextContent("ข้อมูลทดสอบ");
  });

  it("loads guest public endpoints and displays empty data", async () => {
    const request = mockCollections({});
    render(<OverviewPage />);
    expect(screen.getByRole("status", { name: "กำลังโหลดข้อมูล" })).toBeInTheDocument();
    expect(await screen.findByRole("region", { name: "เหตุการณ์น้ำที่รายงาน" })).toHaveTextContent("ยังไม่มีเหตุการณ์ในรายการ");
    expect(screen.getByRole("region", { name: "ระดับน้ำ (จุดสำคัญ)" })).toHaveTextContent("ยังไม่มีสถานีในรายการที่เรียกดู");
    expect(screen.getByRole("region", { name: "ประกาศล่าสุด" })).toHaveTextContent("ยังไม่มีประกาศที่เผยแพร่");
    expect(request.mock.calls.map(([url]) => url).filter((url) => !["/api/v1/weather/ongkharak", "/api/v1/external-data/ongkharak"].includes(url))).toEqual([
      "/api/v1/flood-events?limit=20", "/api/v1/risk-areas?limit=20", "/api/v1/roads?limit=20", "/api/v1/water-stations?limit=5",
      "/api/v1/alerts?limit=20&offset=0", "/api/v1/reports?limit=20&offset=0"
    ]);
    expect(request.mock.calls.filter(([url]) => url === "/api/v1/external-data/ongkharak")).toHaveLength(1);
  });

  it("discloses failed collections rather than treating them as empty", async () => {
    vi.stubGlobal("fetch", vi.fn(() => Promise.reject(new Error("offline"))));
    render(<OverviewPage />);
    expect(await screen.findByRole("alert")).toHaveTextContent("ไม่สามารถโหลดข้อมูลสถานการณ์ได้");
    expect(screen.queryByText("ยังไม่มีข้อมูลสถานการณ์ที่เผยแพร่")).not.toBeInTheDocument();
  });

  it("shows degraded partial data and distinct provenance", async () => {
    vi.stubGlobal("fetch", vi.fn((url: string) => url.includes("risk-areas")
      ? Promise.reject(new Error("offline"))
      : Promise.resolve({ ok: true, json: () => Promise.resolve({ items: url.includes("flood-events")
        ? [event(), event({ id: "community", title: "ชุมชนทดสอบ", source: source("COMMUNITY") }), event({ id: "ai", title: "AI ทดสอบ", source: source("AI") })]
        : [], limit: 20, offset: 0 }) })));
    render(<OverviewPage />);
    expect(await screen.findByText("ข้อมูลบางส่วนไม่พร้อมใช้งาน")).toBeInTheDocument();
    expect(screen.getByText("แหล่งทางการ")).toBeInTheDocument();
    expect(screen.getByText("รายงานชุมชน")).toBeInTheDocument();
    expect(screen.getByText("หลักฐาน AI")).toBeInTheDocument();
  });

  it("keeps stale and unknown evidence unknown, not safe", async () => {
    mockCollections({ "/api/v1/flood-events": [event({ freshness: "STALE", effectiveSeverity: "UNKNOWN", effectiveStatus: "UNKNOWN" })] });
    render(<OverviewPage />);
    expect(await screen.findByText("ข้อมูลทดสอบ")).toBeInTheDocument();
    expect(screen.getAllByText("ไม่ทราบสถานะ").length).toBeGreaterThan(0);
    expect(screen.getByText("ข้อมูลเก่า")).toBeInTheDocument();
    expect(screen.queryByText("ปลอดภัย")).not.toBeInTheDocument();
  });

  it("shows a verified current event with its reported status and observation time", async () => {
    mockCollections({ "/api/v1/flood-events": [event()] });
    render(<OverviewPage />);
    expect(await screen.findByText("ข้อมูลทดสอบ")).toBeInTheDocument();
    expect(screen.getByText("เตือนภัย")).toBeInTheDocument();
    expect(screen.getByText("ยังมีเหตุการณ์")).toBeInTheDocument();
    expect(screen.getByText(/หน่วยงานตัวอย่าง/)).toHaveTextContent("เวลาสังเกต:");
  });

  it("shows observed water readings with their source and freshness", async () => {
    mockCollections({
      "/api/v1/water-stations": [{ id: "station-1", name: "สถานีทดสอบ", source: source("OFFICIAL") }],
      "/api/v1/water-stations/station-1/readings": [{ source: source("COMMUNITY"), observedAt: "2026-10-02T00:00:00Z", waterLevelM: 1.2, flowRateM3s: null, rainfallMm: null, freshness: "STALE", verificationStatus: "UNVERIFIED" }]
    });
    render(<OverviewPage />);
    const water = screen.getByRole("region", { name: "ระดับน้ำ (จุดสำคัญ)" });
    expect(await within(water).findByText("สถานีทดสอบ")).toBeInTheDocument();
    expect(within(water).getByText("ระดับน้ำ 1.2 ม.")).toBeInTheDocument();
    expect(within(water).getByText("รายงานชุมชน")).toBeInTheDocument();
    expect(within(water).getByText("ข้อมูลเก่า")).toBeInTheDocument();
    expect(within(water).getByText("ไม่ทราบสถานะ")).toBeInTheDocument();
  });
});
