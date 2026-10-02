import { render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { OverviewPage } from "./OverviewPage";

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
  it("loads guest public endpoints and displays empty data", async () => {
    const request = mockCollections({});
    render(<OverviewPage />);
    expect(screen.getByRole("status", { name: "กำลังโหลดข้อมูล" })).toBeInTheDocument();
    expect(await screen.findByText("ยังไม่มีข้อมูลสถานการณ์ที่เผยแพร่")).toBeInTheDocument();
    expect(request.mock.calls.map(([url]) => url)).toEqual([
      "/api/v1/flood-events?limit=20", "/api/v1/risk-areas?limit=20", "/api/v1/roads?limit=20", "/api/v1/water-stations?limit=5"
    ]);
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
    expect(await screen.findByText("สถานีทดสอบ")).toBeInTheDocument();
    expect(screen.getByText("ระดับน้ำ 1.2 ม.")).toBeInTheDocument();
    expect(screen.getByText("รายงานชุมชน")).toBeInTheDocument();
    expect(screen.getByText("ข้อมูลเก่า")).toBeInTheDocument();
    expect(screen.getByText("ไม่ทราบสถานะ")).toBeInTheDocument();
  });
});
