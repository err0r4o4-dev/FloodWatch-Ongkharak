import { act, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { MapNavigation, SharedMapViewport, UserLocationMarker } from "./MapNavigation";
import { getMapSession, resetMapSession, setUserPosition } from "./mapSession";
import { useUserLocation } from "./useUserLocation";

const { map, events, scaleProps } = vi.hoisted(() => ({
  map: {
    getCenter: vi.fn(() => ({ lat: 14.1, lng: 101 })), getZoom: vi.fn(() => 18),
    getMinZoom: () => 8, getMaxZoom: () => 18, zoomIn: vi.fn(), zoomOut: vi.fn(), setView: vi.fn(),
    getSize: () => ({ x: 400, y: 300 }), latLngToContainerPoint: vi.fn(() => ({ x: 200, y: 150 })),
    getContainer: () => document.createElement("div"), invalidateSize: vi.fn()
  },
  events: [] as Record<string, () => void>[], scaleProps: [] as Record<string, unknown>[]
}));

vi.mock("react-leaflet", () => ({
  useMap: () => map,
  useMapEvents: (handlers: Record<string, () => void>) => { events.push(handlers); return map; },
  Circle: () => null, CircleMarker: () => null,
  ScaleControl: (props: Record<string, unknown>) => { scaleProps.push(props); return null; }
}));

function Harness() {
  const location = useUserLocation();
  return <><SharedMapViewport /><MapNavigation location={location} /><UserLocationMarker position={location.position} /></>;
}

afterEach(() => { act(() => resetMapSession()); events.length = 0; scaleProps.length = 0; vi.clearAllMocks(); });

describe("shared map navigation", () => {
  it("changes centered status when panning away and returns using the cached fix", () => {
    setUserPosition({ latitude: 14.1, longitude: 101, accuracy: 20, timestamp: Date.now() });
    render(<Harness />);
    const locate = screen.getByRole("button", { name: "ตำแหน่งปัจจุบัน" });
    expect(locate).toHaveAttribute("aria-pressed", "true");
    act(() => { map.latLngToContainerPoint.mockReturnValue({ x: 250, y: 150 }); events.forEach((handlers) => handlers.move?.()); });
    expect(locate).toHaveAttribute("aria-pressed", "false");
    fireEvent.click(locate);
    expect(map.setView).toHaveBeenCalledWith([14.1, 101], 18, { animate: false });
    expect(locate.querySelector("svg")).toBeInTheDocument();
  });

  it("uses the same metric scale, disables zoom at limits, and publishes the camera", () => {
    render(<Harness />);
    expect(scaleProps.at(-1)).toMatchObject({ imperial: false, maxWidth: 150 });
    expect(screen.getByRole("button", { name: "ซูมเข้า" })).toBeDisabled();
    act(() => { map.getCenter.mockReturnValue({ lat: 14.3, lng: 101.3 }); events.forEach((handlers) => handlers.moveend?.()); });
    expect(getMapSession().viewport).toEqual({ center: [14.3, 101.3], zoom: 18 });
  });
});
