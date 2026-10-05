import { fireEvent, render as renderView, screen, waitFor, within } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import type { ReactElement } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { MapPage } from "./MapPage";
import type { LayerItems, MapSelection } from "./types";

function render(view: ReactElement) {
  return renderView(<MemoryRouter>{view}</MemoryRouter>);
}

vi.mock("./components/SituationMap", async () => {
  const React = await import("react");
  return { SituationMap: ({ onBoundsChange, onSelect, items, openDetailsOnSelect }: { onBoundsChange: (bounds: { west: number; south: number; east: number; north: number }) => void; onSelect: (selection: MapSelection) => void; items: LayerItems; openDetailsOnSelect?: boolean }) => {
    const bounds = { west: 100.9, south: 14, east: 101.2, north: 14.3 };
    React.useEffect(() => { onBoundsChange(bounds); }, [onBoundsChange]);
    return <div data-direct-details={openDetailsOnSelect} aria-label="แผนที่สถานการณ์แบบโต้ตอบ"><button type="button" aria-label="เผยแพร่ขอบเขตเดิมอีกครั้ง" onClick={() => onBoundsChange(bounds)}>เผยแพร่ขอบเขตเดิมอีกครั้ง</button>{items.roads.map((feature) => <button key={feature.id} onClick={() => onSelect({ layer: "roads", feature })}>{feature.name}</button>)}{items.water.map((feature) => <button key={feature.id} onClick={() => onSelect({ layer: "water", feature })}>{feature.name}</button>)}{items.flood.map((feature) => <button key={feature.id} onClick={() => onSelect({ layer: "flood", feature })}>{feature.title}</button>)}</div>;
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
  it("opens the risk directory, filters stale evidence as UNKNOWN, and restores focus", async () => {
    const risk = { id: "risk-fresh", name: "พื้นที่เสี่ยงทดสอบ", description: "หลักฐานทดสอบ", source, riskLevel: "HIGH", effectiveRiskLevel: "HIGH", freshness: "FRESH", validFrom: null, validTo: null, geometry: { type: "MultiPolygon", coordinates: [[[[101, 14], [101.1, 14], [101.1, 14.1], [101, 14]]]] } };
    mockApi({ "/api/v1/risk-areas": [risk, { ...risk, id: "risk-stale", name: "พื้นที่ข้อมูลเก่า", freshness: "STALE" }] });
    render(<MapPage />);
    const trigger = screen.getByRole("button", { name: "พื้นที่เสี่ยง" });
    fireEvent.click(trigger);
    expect(await screen.findByRole("button", { name: /พื้นที่ข้อมูลเก่า/ })).toBeInTheDocument();
    expect(screen.getByText("พื้นที่ในบริเวณแผนที่ที่กำลังดู")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /กรอง: ความเสี่ยงสูง$/ }));
    expect(screen.getByRole("button", { name: /พื้นที่เสี่ยงทดสอบ/ })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /พื้นที่ข้อมูลเก่า/ })).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /กรอง: ไม่ทราบสถานะ/ }));
    expect(screen.getByRole("button", { name: /พื้นที่ข้อมูลเก่า/ })).toHaveTextContent("ไม่ทราบสถานะ");
    fireEvent.keyDown(screen.getByRole("button", { name: /พื้นที่ข้อมูลเก่า/ }), { key: "Escape" });
    expect(trigger).toHaveFocus();
    expect(trigger).toHaveAttribute("aria-expanded", "false");
  });

  it("keeps risk failures distinct from empty data and offers retry", async () => {
    mockApi({}, ["/risk-areas"]);
    render(<MapPage />);
    fireEvent.click(screen.getByRole("button", { name: "พื้นที่เสี่ยง" }));
    expect(await screen.findByText("ไม่สามารถโหลดพื้นที่เสี่ยงได้ · สถานะไม่ทราบ")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "ลองโหลดพื้นที่เสี่ยงอีกครั้ง" })).toBeInTheDocument();
    expect(screen.queryByText("ไม่มีพื้นที่เสี่ยงในข้อมูลที่โหลด ไม่ได้หมายความว่าปลอดภัย")).not.toBeInTheDocument();
  });

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
    expect(screen.getByLabelText("คำอธิบายสัญลักษณ์")).not.toHaveTextContent("สถานีวัดน้ำ");
  });

  it("selects a stale road from the map and keeps UNKNOWN distinct from passable", async () => {
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

  it("keeps a selected item when the map republishes identical bounds", async () => {
    const station = { id: "station-repeat", name: "สถานีขอบเขตเดิม", source, active: true, location: { type: "Point", coordinates: [101, 14.1] } };
    mockApi({ "/api/v1/water-stations": [station] });
    render(<MapPage />);
    fireEvent.click(await screen.findByRole("button", { name: /สถานีขอบเขตเดิม/ }));
    expect(screen.getByRole("region", { name: "ข้อมูลตำแหน่งที่เลือก" })).toHaveTextContent("สถานีขอบเขตเดิม");
    fireEvent.click(screen.getByRole("button", { name: "เผยแพร่ขอบเขตเดิมอีกครั้ง" }));
    expect(screen.getByRole("region", { name: "ข้อมูลตำแหน่งที่เลือก" })).toHaveTextContent("สถานีขอบเขตเดิม");
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

  it("opens map details directly and removes the bounds list button", async () => {
    mockApi({ "/api/v1/roads": [road] });
    render(<MapPage />);
    expect(screen.queryByRole("complementary", { name: "ข้อมูลแผนที่และเส้นทาง" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /รายการในขอบเขต/ })).not.toBeInTheDocument();
    expect(screen.getByLabelText("แผนที่สถานการณ์แบบโต้ตอบ")).toHaveAttribute("data-direct-details", "true");
    fireEvent.click(await screen.findByRole("button", { name: "ถนนทดสอบ" }));
    expect(screen.getByRole("complementary", { name: "ข้อมูลแผนที่และเส้นทาง" })).toBeInTheDocument();
    expect(screen.getByRole("region", { name: "ข้อมูลตำแหน่งที่เลือก" })).toHaveTextContent("ถนนทดสอบ");
  });

  it("searches only loaded stations and areas without claiming global place search", async () => {
    const station = { id: "station-search", name: "สถานีคลองทดสอบ", source, active: true, location: { type: "Point", coordinates: [101, 14.1] } };
    mockApi({ "/api/v1/water-stations": [station] });
    render(<MapPage />);
    const input = screen.getByRole("searchbox", { name: "ค้นหาสถานีหรือพื้นที่ในขอบเขตที่โหลด" });
    fireEvent.change(input, { target: { value: "คลองทดสอบ" } });
    const result = await within(screen.getByRole("search")).findByRole("button", { name: /สถานีคลองทดสอบ/ });
    expect(screen.getAllByText("ค้นหาเฉพาะสถานีและพื้นที่ในขอบเขตที่โหลด").length).toBeGreaterThan(0);
    fireEvent.click(result);
    expect(input).toHaveValue("");
    expect(screen.getByRole("region", { name: "ข้อมูลตำแหน่งที่เลือก" })).toHaveTextContent("สถานีคลองทดสอบ");
    fireEvent.change(input, { target: { value: "ไม่พบสถานี" } });
    expect(screen.getByText("ไม่พบสถานีหรือพื้นที่ในข้อมูลที่โหลด")).toBeInTheDocument();
    fireEvent.keyDown(input, { key: "Escape" });
    expect(input).toHaveValue("");
  });
});
