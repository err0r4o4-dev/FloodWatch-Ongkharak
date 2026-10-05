import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { SituationMap } from "./SituationMap";
import type { LayerItems, LayerVisibility } from "../types";
import type { CommunityReport } from "../../../features/reports/types";
import { mapSettings, resetMapSession, setMapViewport } from "../../../shared/map/mapSession";

const { map, mapEvents, containers, popupPositions } = vi.hoisted(() => ({
  popupPositions: [] as Array<[number, number]>,
  containers: [] as Record<string, unknown>[],
  mapEvents: [] as Array<Record<string, () => void>>,
  map: {
    getBounds: () => ({ getWest: () => 100.9, getEast: () => 101.2, getSouth: () => 14, getNorth: () => 14.3 }),
    getContainer: () => document.createElement("div"), invalidateSize: vi.fn(), fitBounds: vi.fn(), setView: vi.fn(), zoomIn: vi.fn(), zoomOut: vi.fn(),
    getCenter: () => ({ lat: 14.1, lng: 101 }), getZoom: () => 11, getMinZoom: () => 8, getMaxZoom: () => 18,
    getSize: () => ({ x: 400, y: 300 }), latLngToContainerPoint: () => ({ x: 200, y: 150 })
  }
}));

vi.mock("react-leaflet", () => ({
  MapContainer: ({ children, ...props }: { children: ReactNode }) => { containers.push(props); return <div>{children}</div>; },
  TileLayer: () => null,
  ScaleControl: () => null,
  Circle: () => null,
  Popup: ({ children, position }: { children: ReactNode; position: [number, number] }) => { popupPositions.push(position); return <div>{children}</div>; },
  Tooltip: ({ children }: { children: ReactNode }) => <span>{children}</span>,
  Marker: ({ title, eventHandlers }: { title: string; eventHandlers: { click: () => void; keydown?: (event: { originalEvent: KeyboardEvent }) => void } }) => <button type="button" onClick={eventHandlers.click} onKeyDown={(event) => eventHandlers.keydown?.({ originalEvent: event.nativeEvent })}>{title}</button>,
  CircleMarker: () => null,
  GeoJSON: ({ eventHandlers }: { eventHandlers: { click: (event: unknown) => void } }) => <button type="button" onClick={() => eventHandlers.click({ latlng: { lng: 101.05, lat: 14.05 } })}>เลือกพื้นที่</button>,
  useMap: () => map,
  useMapEvents: (events: Record<string, () => void>) => { mapEvents.push(events); return map; }
}));

const visible: LayerVisibility = { flood: true, water: true, risk: true, roads: true, reports: true };
const items: LayerItems = {
  flood: [], risk: [], roads: [], reports: [], water: [{ id: "station-1", name: "สถานีทดสอบ", active: true, source: { id: "source", code: "test", name: "แหล่งทดสอบ", sourceType: "OFFICIAL" }, location: { type: "Point", coordinates: [101, 14.1] } }]
};
const points = { origin: { longitude: "", latitude: "" }, destination: { longitude: "", latitude: "" } };

const clusterReport: CommunityReport = {
  id: "report-map-1", type: "FLOOD", description: "รายงานน้ำท่วม", locationName: "พื้นที่แผนที่ทดสอบ", location: { type: "Point", coordinates: [101.02, 14.1] },
  estimatedWaterDepthM: 0.2, observedAt: "2026-10-02T01:00:00Z", waterTrend: "RISING", status: "VERIFIED", verificationStatus: "VERIFIED", publicationStatus: "PUBLISHED",
  sourceType: "COMMUNITY", createdAt: "2026-10-02T01:00:00Z", updatedAt: "2026-10-02T01:00:00Z", verifiedAt: "2026-10-02T01:00:00Z", media: []
};

afterEach(() => { act(() => resetMapSession()); vi.unstubAllGlobals(); mapEvents.length = 0; containers.length = 0; popupPositions.length = 0; vi.clearAllMocks(); });

function prepare() {
  vi.stubGlobal("ResizeObserver", class { observe() {} disconnect() {} });
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: true, json: async () => ({ items: [] }) }));
  const onSelect = vi.fn();
  const props = { items, visible, mode: "SITUATION" as const, points, picking: null, onBoundsChange: vi.fn(), onMapPoint: vi.fn(), onSelect, enablePopups: true };
  return { props, onSelect };
}

describe("SituationMap popup flow", () => {
  it("opens side-panel details directly without a marker popup when requested", () => {
    const { props, onSelect } = prepare();
    render(<SituationMap {...props} openDetailsOnSelect />);
    fireEvent.click(screen.getByRole("button", { name: "สถานีทดสอบ" }));
    expect(onSelect).toHaveBeenCalledWith({ layer: "water", feature: items.water[0] });
    expect(screen.queryByRole("region", { name: "ข้อมูลสรุปบนแผนที่" })).not.toBeInTheDocument();
  });
  it("keeps the popup anchor stable during viewport and data updates", async () => {
    const { props } = prepare();
    const view = render(<SituationMap {...props} />);
    fireEvent.click(screen.getByRole("button", { name: "สถานีทดสอบ" }));
    await screen.findByText(/ยังไม่มีค่าที่สังเกตได้/);
    const anchor = popupPositions.at(-1);
    act(() => setMapViewport({ center: [14.2, 101.2], zoom: 12 }));
    view.rerender(<SituationMap {...props} items={{ ...items, water: [...items.water] }} />);
    expect(popupPositions.at(-1)).toBe(anchor);
    expect(screen.getByRole("region", { name: "ข้อมูลสรุปบนแผนที่" })).toBeInTheDocument();
  });

  it("restores a shared camera on the full map without applying it to previews", () => {
    const { props } = prepare();
    setMapViewport({ center: [14.6, 101.6], zoom: 18 });
    const view = render(<SituationMap {...props} />);
    expect(containers.at(-1)).toMatchObject({ center: [14.6, 101.6], zoom: 18, maxZoom: mapSettings.maxZoom });
    view.unmount();
    render(<SituationMap {...props} enablePopups={false} />);
    expect(containers.at(-1)).toMatchObject({ center: mapSettings.initialCenter, zoom: mapSettings.initialZoom });
  });

  it("prioritizes explicit bounds over the shared camera", () => {
    const { props } = prepare();
    setMapViewport({ center: [14.6, 101.6], zoom: 18 });
    render(<SituationMap {...props} initialBounds={{ west: 100, south: 14, east: 101, north: 15 }} />);
    expect(containers.at(-1)).toMatchObject({ center: mapSettings.initialCenter, zoom: mapSettings.initialZoom });
    expect(map.fitBounds).toHaveBeenCalledWith([[14, 100], [15, 101]], expect.objectContaining({ animate: false }));
  });

  it("does not request GPS automatically from the legacy tracking flag", () => {
    const { props } = prepare();
    const getCurrentPosition = vi.fn();
    vi.stubGlobal("navigator", { geolocation: { getCurrentPosition } });
    localStorage.setItem("kineguide:mapTracking", "true");
    render(<SituationMap {...props} />);
    expect(getCurrentPosition).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "ตำแหน่งปัจจุบัน" })).toHaveAttribute("aria-pressed", "false");
    localStorage.removeItem("kineguide:mapTracking");
  });

  it("still renders when ResizeObserver is unavailable", () => {
    const { props } = prepare();
    vi.stubGlobal("ResizeObserver", undefined);
    render(<SituationMap {...props} />);
    expect(screen.getByRole("button", { name: "สถานีทดสอบ" })).toBeInTheDocument();
    expect(props.onBoundsChange).toHaveBeenCalled();
  });
  it("supports compact controls without removing translated labels", () => {
    const { props } = prepare();
    render(<SituationMap {...props} compactControls />);
    expect(screen.getByRole("button", { name: "กลับมุมมองเริ่มต้น" })).toHaveTextContent("⌖");
    expect(screen.getByRole("button", { name: "ปรับให้พอดีกับข้อมูล" })).toHaveTextContent("⤢");
    expect(screen.getByRole("button", { name: "กลับมุมมองเริ่มต้น" })).toHaveAttribute("title", "กลับมุมมองเริ่มต้น");
  });
  it("opens marker popups with Enter and dismisses them with Escape", async () => {
    const { props } = prepare();
    render(<SituationMap {...props} />);
    fireEvent.keyDown(screen.getByRole("button", { name: "สถานีทดสอบ" }), { key: "Enter" });
    const popup = screen.getByRole("region", { name: "ข้อมูลสรุปบนแผนที่" });
    await screen.findByText(/ยังไม่มีค่าที่สังเกตได้/);
    fireEvent.keyDown(popup, { key: "Escape" });
    expect(screen.queryByRole("region", { name: "ข้อมูลสรุปบนแผนที่" })).not.toBeInTheDocument();
  });
  it("opens a popup on marker click and opens details only on request", async () => {
    const { props, onSelect } = prepare();
    render(<SituationMap {...props} />);
    fireEvent.click(screen.getByRole("button", { name: "สถานีทดสอบ" }));
    expect(screen.getByRole("region", { name: "ข้อมูลสรุปบนแผนที่" })).toHaveTextContent("สถานีทดสอบ");
    expect(onSelect).not.toHaveBeenCalled();
    await screen.findByText(/ยังไม่มีค่าที่สังเกตได้/);
    fireEvent.click(screen.getByRole("button", { name: "ดูรายละเอียด" }));
    expect(onSelect).toHaveBeenCalledWith({ layer: "water", feature: items.water[0] });
    expect(screen.queryByRole("region", { name: "ข้อมูลสรุปบนแผนที่" })).not.toBeInTheDocument();
  });

  it("opens a grouped community report marker with the keyboard", () => {
    const cluster = { id: "report-group:report-map-1", name: "", location: clusterReport.location, sourceType: "COMMUNITY" as const, reports: [clusterReport] };
    const { props, onSelect } = prepare();
    render(<SituationMap {...props} items={{ ...items, reports: [cluster] }} />);
    const marker = screen.getByRole("button", { name: /กลุ่มรายงานชุมชน · 1 รายงาน/ });
    fireEvent.keyDown(marker, { key: "Enter" });
    expect(screen.getByRole("region", { name: "ข้อมูลสรุปบนแผนที่" })).toHaveTextContent("กลุ่มรายงานชุมชน");
    expect(screen.getByText("1 รายงานในบริเวณนี้")).toBeInTheDocument();
    expect(screen.queryByText("แนวโน้มคำนวณ")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "ดูรายละเอียด" }));
    expect(onSelect).toHaveBeenCalledWith({ layer: "reports", feature: cluster });
  });

  it("dismisses the popup when moving or hiding its layer", async () => {
    const { props } = prepare();
    const view = render(<SituationMap {...props} />);
    fireEvent.click(screen.getByRole("button", { name: "สถานีทดสอบ" }));
    act(() => [...mapEvents].reverse().find((events) => events.movestart)?.movestart());
    expect(screen.queryByRole("region", { name: "ข้อมูลสรุปบนแผนที่" })).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "สถานีทดสอบ" }));
    view.rerender(<SituationMap {...props} visible={{ ...visible, water: false }} />);
    expect(screen.queryByRole("region", { name: "ข้อมูลสรุปบนแผนที่" })).not.toBeInTheDocument();
    view.rerender(<SituationMap {...props} />);
    expect(screen.queryByRole("region", { name: "ข้อมูลสรุปบนแผนที่" })).not.toBeInTheDocument();
    await waitFor(() => expect(screen.queryByText("กำลังโหลดค่าที่สังเกตล่าสุด")).not.toBeInTheDocument());
  });

  it("focuses a search result and previews it without opening the details panel", async () => {
    const { props, onSelect } = prepare();
    render(<SituationMap {...props} focusSelection={{ layer: "water", feature: items.water[0] }} />);
    expect(map.fitBounds).toHaveBeenCalled();
    expect(screen.getByRole("region", { name: "ข้อมูลสรุปบนแผนที่" })).toBeInTheDocument();
    expect(onSelect).not.toHaveBeenCalled();
    await screen.findByText(/ยังไม่มีค่าที่สังเกตได้/);
  });
});
