import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { SituationMap } from "./SituationMap";
import type { LayerItems, LayerVisibility } from "../types";

const { map, mapEvents } = vi.hoisted(() => ({
  mapEvents: [] as Array<Record<string, () => void>>,
  map: {
    getBounds: () => ({ getWest: () => 100.9, getEast: () => 101.2, getSouth: () => 14, getNorth: () => 14.3 }),
    getContainer: () => document.createElement("div"), invalidateSize: vi.fn(), fitBounds: vi.fn(), setView: vi.fn(), zoomIn: vi.fn(), zoomOut: vi.fn()
  }
}));

vi.mock("react-leaflet", () => ({
  MapContainer: ({ children }: { children: ReactNode }) => <div>{children}</div>,
  TileLayer: () => null,
  Popup: ({ children }: { children: ReactNode }) => <div>{children}</div>,
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

afterEach(() => { vi.unstubAllGlobals(); mapEvents.length = 0; vi.clearAllMocks(); });

function prepare() {
  vi.stubGlobal("ResizeObserver", class { observe() {} disconnect() {} });
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: true, json: async () => ({ items: [] }) }));
  const onSelect = vi.fn();
  const props = { items, visible, mode: "SITUATION" as const, points, picking: null, onBoundsChange: vi.fn(), onMapPoint: vi.fn(), onSelect, enablePopups: true };
  return { props, onSelect };
}

describe("SituationMap popup flow", () => {
  it("still renders when ResizeObserver is unavailable", () => {
    const { props } = prepare();
    vi.stubGlobal("ResizeObserver", undefined);
    render(<SituationMap {...props} />);
    expect(screen.getByRole("button", { name: "สถานีทดสอบ" })).toBeInTheDocument();
    expect(props.onBoundsChange).toHaveBeenCalled();
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
