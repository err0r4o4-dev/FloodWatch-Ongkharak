import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { MapPage } from "./MapPage";

vi.mock("./components/SituationMap", async () => {
  const React = await import("react");
  return { SituationMap: ({ onBoundsChange }: { onBoundsChange: (bounds: { west: number; south: number; east: number; north: number }) => void }) => {
    React.useEffect(() => { onBoundsChange({ west: 100.9, south: 14, east: 101.2, north: 14.3 }); }, [onBoundsChange]);
    return <div aria-label="แผนที่สถานการณ์แบบโต้ตอบ" />;
  } };
});

const source = { id: "source-1", code: "sample", name: "หน่วยงานทดสอบ", sourceType: "COMMUNITY" };
const road = {
  id: "road-1", name: "ถนนทดสอบ", geometry: { type: "MultiLineString", coordinates: [[[101, 14], [101.1, 14.1]]] },
  currentCondition: { source, status: "UNKNOWN", reportedStatus: "PASSABLE", observedAt: "2026-10-02T00:00:00Z", freshness: "STALE", verificationStatus: "VERIFIED", confidence: 0.5, description: "ข้อมูลเก่า", validUntil: null }
};

function mockApi(collections: Record<string, unknown[]> = {}, failed: string[] = []) {
  const request = vi.fn((url: string) => failed.some((path) => url.includes(path))
    ? Promise.reject(new Error("offline"))
    : Promise.resolve({ ok: true, json: () => Promise.resolve({ items: collections[new URL(url, "http://localhost").pathname] ?? [], limit: 100, offset: 0 }) }));
  vi.stubGlobal("fetch", request);
  return request;
}

afterEach(() => { vi.unstubAllGlobals(); vi.restoreAllMocks(); });

describe("MapPage", () => {
  it("loads five visible collections and shows an honest empty state", async () => {
    const request = mockApi();
    render(<MapPage />);
    expect(screen.getByText("กำลังโหลดข้อมูลตามขอบเขตแผนที่")).toBeInTheDocument();
    expect(await screen.findByText(/ไม่มีรายการจากชั้นข้อมูลที่โหลดสำเร็จ/)).toBeInTheDocument();
    expect(request).toHaveBeenCalledTimes(5);
    expect(request.mock.calls.every(([url]) => new URL(url, "http://localhost").searchParams.get("bbox") === "100.9,14,101.2,14.3")).toBe(true);
  });

  it("distinguishes full failure from partial degradation", async () => {
    mockApi({}, ["/risk-areas"]);
    const { unmount } = render(<MapPage />);
    expect(await screen.findByText(/ข้อมูลบางส่วนไม่พร้อมใช้งาน: พื้นที่เสี่ยง/)).toBeInTheDocument();
    unmount();
    mockApi({}, ["/api/v1/"]);
    render(<MapPage />);
    expect(await screen.findByRole("alert")).toHaveTextContent("ไม่สามารถโหลดข้อมูลแผนที่ได้");
  });

  it("toggles layers without requesting the disabled layer", async () => {
    const request = mockApi();
    render(<MapPage />);
    await screen.findByText(/ไม่มีรายการจากชั้นข้อมูลที่โหลดสำเร็จ/);
    request.mockClear();
    fireEvent.click(screen.getByRole("checkbox", { name: "สถานีวัดน้ำ" }));
    await waitFor(() => expect(request).toHaveBeenCalledTimes(4));
    expect(request.mock.calls.some(([url]) => url.includes("water-stations"))).toBe(false);
    expect(screen.getByText(/คำอธิบาย:/)).not.toHaveTextContent("◯ สถานีวัดน้ำ");
  });

  it("selects a stale road via the keyboard alternative and keeps UNKNOWN distinct from passable", async () => {
    mockApi({ "/api/v1/roads": [road] });
    render(<MapPage />);
    fireEvent.click(await screen.findByRole("button", { name: /ถนนทดสอบ/ }));
    expect(screen.getByRole("region", { name: "ข้อมูลตำแหน่งที่เลือก" })).toHaveTextContent("หน่วยงานทดสอบ");
    expect(screen.getAllByText("ไม่ทราบสถานะ").length).toBeGreaterThan(0);
    expect(screen.getByText(/ไม่สามารถสรุปว่าปลอดภัยหรือผ่านได้/)).toBeInTheDocument();
    expect(screen.getAllByText("ข้อมูลเก่า").length).toBeGreaterThan(0);
    expect(screen.getByRole("button", { name: "ปิดข้อมูลตำแหน่ง" })).toHaveFocus();
    fireEvent.keyDown(screen.getByRole("button", { name: "ปิดข้อมูลตำแหน่ง" }), { key: "Escape" });
    expect(screen.queryByRole("region", { name: "ข้อมูลตำแหน่งที่เลือก" })).not.toBeInTheDocument();
  });

  it("offers route inputs without calculating or fabricating a route", async () => {
    mockApi();
    render(<MapPage />);
    fireEvent.click(screen.getByRole("button", { name: "เส้นทาง" }));
    expect(screen.getByRole("heading", { name: "วางแผนเส้นทาง" })).toBeInTheDocument();
    expect(screen.getByText(/ยังไม่มีบริการคำนวณเส้นทาง/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "เลือกจุดเริ่มต้นบนแผนที่" })).toHaveAttribute("aria-pressed", "false");
    fireEvent.click(screen.getByRole("button", { name: "เลือกจุดเริ่มต้นบนแผนที่" }));
    expect(screen.getByRole("button", { name: "เลือกจุดเริ่มต้นบนแผนที่" })).toHaveAttribute("aria-pressed", "true");
    expect(screen.queryByText("เส้นทางแนะนำ")).not.toBeInTheDocument();
  });

  it("keeps AI event provenance and observation evidence visible on selection", async () => {
    const flood = {
      id: "event-ai", title: "เหตุการณ์ทดสอบ", description: "หลักฐานทดสอบ", source: { ...source, sourceType: "AI" },
      geometry: { type: "Point", coordinates: [101, 14.1] }, severity: "WATCH", effectiveSeverity: "WATCH",
      status: "ACTIVE", effectiveStatus: "ACTIVE", observedAt: "2026-10-02T00:00:00Z",
      freshness: "FRESH", verificationStatus: "VERIFIED", confidence: 0.8
    };
    mockApi({ "/api/v1/flood-events": [flood] });
    render(<MapPage />);
    fireEvent.click(await screen.findByRole("button", { name: /เหตุการณ์ทดสอบ/ }));
    const details = screen.getByRole("region", { name: "ข้อมูลตำแหน่งที่เลือก" });
    expect(details).toHaveTextContent("หลักฐาน AI");
    expect(details).toHaveTextContent("เวลาสังเกต");
    expect(details).toHaveTextContent("ตรวจสอบแล้ว");
    expect(details).toHaveTextContent("80%");
  });

  it("keeps station source separate from the latest reading source", async () => {
    const station = { id: "station-1", name: "สถานีทดสอบ", source: { ...source, sourceType: "OFFICIAL" }, active: true, location: { type: "Point", coordinates: [101, 14.1] } };
    const reading = { source, observedAt: "2026-10-02T00:00:00Z", freshness: "STALE", verificationStatus: "UNVERIFIED", confidence: null, waterLevelM: 1.2, flowRateM3s: null, rainfallMm: null };
    mockApi({ "/api/v1/water-stations": [station], "/api/v1/water-stations/station-1/readings": [reading] });
    render(<MapPage />);
    fireEvent.click(await screen.findByRole("button", { name: /สถานีทดสอบ/ }));
    const details = screen.getByRole("region", { name: "ข้อมูลตำแหน่งที่เลือก" });
    expect(details).toHaveTextContent("แหล่งทางการ");
    await waitFor(() => expect(details).toHaveTextContent("รายงานชุมชน"));
    expect(details).toHaveTextContent("ข้อมูลเก่า");
    expect(details).toHaveTextContent("ยังไม่ตรวจสอบ");
    expect(details).toHaveTextContent("ไม่ทราบสถานะ");
  });
});
