import { act, fireEvent, render, screen } from "@testing-library/react";
import type { ReactNode } from "react";
import { useState } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ReportLocationPicker } from "./ReportLocationPicker";
import type { ReportLocation } from "./types";

const { map, mapEvents, circleMarkers, circles, markers } = vi.hoisted(() => ({
  map: {
    getCenter: vi.fn(() => ({ lat: 14.4, lng: 101.4 })),
    getZoom: vi.fn(() => 11),
    setView: vi.fn()
  },
  mapEvents: [] as Array<{ click?: (event: { latlng: { lng: number; lat: number } }) => void }>,
  circleMarkers: [] as Array<Record<string, unknown>>,
  circles: [] as Array<Record<string, unknown>>,
  markers: [] as Array<Record<string, unknown>>
}));

vi.mock("react-leaflet", () => ({
  Marker: (props: Record<string, unknown>) => {
    markers.push(props);
    return null;
  },
  Circle: (props: Record<string, unknown>) => {
    circles.push(props);
    return null;
  },
  CircleMarker: (props: Record<string, unknown>) => {
    circleMarkers.push(props);
    return null;
  },
  MapContainer: ({ children }: { children: ReactNode }) => <div data-testid="leaflet-container">{children}</div>,
  TileLayer: () => null,
  useMap: () => map,
  useMapEvents: (handlers: { click?: (event: { latlng: { lng: number; lat: number } }) => void }) => {
    mapEvents.push(handlers);
    return map;
  }
}));

type GeolocationCallbacks = {
  success: (position: GeolocationPosition) => void;
  failure: (error: GeolocationPositionError) => void;
};

function position(latitude: number, longitude: number, accuracy = 24): GeolocationPosition {
  return {
    coords: { latitude, longitude, accuracy, altitude: null, altitudeAccuracy: null, heading: null, speed: null, toJSON: () => ({}) },
    timestamp: Date.now(),
    toJSON: () => ({})
  };
}

function installGeolocation() {
  const callbacks: GeolocationCallbacks[] = [];
  const getCurrentPosition = vi.fn((success: PositionCallback, failure: PositionErrorCallback) => {
    callbacks.push({ success, failure });
  });
  vi.stubGlobal("navigator", { geolocation: { getCurrentPosition } });
  Object.defineProperty(window, "isSecureContext", { configurable: true, value: true });
  return { callbacks, getCurrentPosition };
}

function PickerHarness({ onSelect, initialLocation = null }: { onSelect: (location: ReportLocation | null) => void; initialLocation?: ReportLocation | null }) {
  const [location, setLocation] = useState<ReportLocation | null>(initialLocation);
  const change = (next: ReportLocation | null) => {
    setLocation(next);
    onSelect(next);
  };
  return <ReportLocationPicker location={location} onChange={change} resetKey="test" />;
}

afterEach(() => {
  vi.unstubAllGlobals();
  vi.clearAllMocks();
  mapEvents.length = 0;
  circleMarkers.length = 0;
  circles.length = 0;
  markers.length = 0;
});

describe("ReportLocationPicker", () => {
  it("selects map clicks as GeoJSON longitude then latitude without coordinate inputs", () => {
    const onSelect = vi.fn();
    render(<PickerHarness onSelect={onSelect} />);

    expect(screen.queryAllByRole("spinbutton")).toHaveLength(0);
    act(() => mapEvents.at(-1)?.click?.({ latlng: { lng: 101.2, lat: 14.3 } }));

    expect(onSelect).toHaveBeenLastCalledWith({ type: "Point", coordinates: [101.2, 14.3] });
    expect(screen.getByRole("status")).toHaveTextContent("เลือก");
  });

  it("keeps the locate control compact with an accessible hidden label", () => {
    render(<PickerHarness onSelect={vi.fn()} />);

    const locate = screen.getByRole("button", { name: "ไปยังตำแหน่งของฉัน" });
    expect(locate.querySelector(".report-location-picker__visually-hidden")).toBeInTheDocument();
  });

  it("does not request permission on mount and locates without changing the incident pin", () => {
    const { callbacks, getCurrentPosition } = installGeolocation();
    const onSelect = vi.fn();
    const initialLocation: ReportLocation = { type: "Point", coordinates: [101.1, 14.2] };
    render(<PickerHarness onSelect={onSelect} initialLocation={initialLocation} />);

    expect(getCurrentPosition).not.toHaveBeenCalled();
    expect(screen.getByTestId("leaflet-container")).not.toContainElement(screen.getByRole("button", { name: "ไปยังตำแหน่งของฉัน" }));
    fireEvent.click(screen.getByRole("button", { name: "ไปยังตำแหน่งของฉัน" }));
    expect(getCurrentPosition).toHaveBeenCalledWith(expect.any(Function), expect.any(Function), { enableHighAccuracy: true, timeout: 10_000, maximumAge: 0 });

    act(() => callbacks[0].success(position(14.5, 101.5, 18)));

    expect(onSelect).not.toHaveBeenCalled();
    expect(map.setView).toHaveBeenCalledWith([14.5, 101.5], 16, { animate: false });
    expect(screen.getByText("ตำแหน่งของคุณอาจคลาดเคลื่อนประมาณ 18 เมตร")).toBeInTheDocument();
    expect(circleMarkers.some((props) => Array.isArray(props.center) && props.center[0] === 14.5 && props.center[1] === 101.5)).toBe(true);
  });

  it("always refreshes before explicitly using the current position as the incident", () => {
    const { callbacks, getCurrentPosition } = installGeolocation();
    const onSelect = vi.fn();
    render(<PickerHarness onSelect={onSelect} />);

    fireEvent.click(screen.getByRole("button", { name: "ไปยังตำแหน่งของฉัน" }));
    act(() => callbacks[0].success(position(14.5, 101.5)));
    expect(onSelect).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole("button", { name: "ใช้ตำแหน่งของฉันเป็นจุดเกิดเหตุ" }));
    expect(getCurrentPosition).toHaveBeenCalledTimes(2);
    act(() => callbacks[1].success(position(14.6, 101.6)));

    expect(onSelect).toHaveBeenLastCalledWith({ type: "Point", coordinates: [101.6, 14.6] });
  });

  it("keeps the incident pin when refreshing the user position", () => {
    const { callbacks } = installGeolocation();
    const onSelect = vi.fn();
    const initialLocation: ReportLocation = { type: "Point", coordinates: [101.1, 14.2] };
    render(<PickerHarness onSelect={onSelect} initialLocation={initialLocation} />);

    fireEvent.click(screen.getByRole("button", { name: "ไปยังตำแหน่งของฉัน" }));
    act(() => callbacks[0].success(position(14.5, 101.5)));

    expect(onSelect).not.toHaveBeenCalled();
    expect(markers.at(-1)).toMatchObject({ position: [14.2, 101.1], interactive: false });
    expect(markers.at(-1)?.icon).toMatchObject({ options: { className: "report-incident-pin", iconSize: [32, 40], iconAnchor: [16, 39] } });
    expect(circleMarkers.some((props) => Array.isArray(props.center) && props.center[0] === 14.2 && props.center[1] === 101.1)).toBe(false);
    expect(screen.getByRole("status")).toHaveTextContent("เลือกจุดเกิดเหตุแล้ว");
  });

  it("clears the previous user point while refreshing and after a failed refresh", () => {
    const { callbacks } = installGeolocation();
    render(<PickerHarness onSelect={vi.fn()} />);

    fireEvent.click(screen.getByRole("button", { name: "ไปยังตำแหน่งของฉัน" }));
    act(() => callbacks[0].success(position(14.5, 101.5, 18)));
    expect(screen.getByText("ตำแหน่งของคุณอาจคลาดเคลื่อนประมาณ 18 เมตร")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "ไปยังตำแหน่งของฉัน" }));
    expect(screen.queryByText("ตำแหน่งของคุณอาจคลาดเคลื่อนประมาณ 18 เมตร")).not.toBeInTheDocument();
    act(() => callbacks[1].failure({ code: 2, message: "unavailable" } as GeolocationPositionError));
    expect(screen.queryByText("ตำแหน่งของคุณอาจคลาดเคลื่อนประมาณ 18 เมตร")).not.toBeInTheDocument();
  });

  it("shows a permission error and retries with the same locate button without clearing the pin", () => {
    const { callbacks, getCurrentPosition } = installGeolocation();
    const onSelect = vi.fn();
    const initialLocation: ReportLocation = { type: "Point", coordinates: [101.1, 14.2] };
    render(<PickerHarness onSelect={onSelect} initialLocation={initialLocation} />);

    fireEvent.click(screen.getByRole("button", { name: "ไปยังตำแหน่งของฉัน" }));
    act(() => callbacks[0].failure({ code: 1, message: "denied" } as GeolocationPositionError));
    expect(screen.getByRole("alert")).toHaveTextContent("ไม่ได้รับอนุญาตให้ใช้ตำแหน่ง");
    expect(screen.getByRole("status")).toHaveTextContent("เลือกจุดเกิดเหตุแล้ว");

    fireEvent.click(screen.getByRole("button", { name: "ไปยังตำแหน่งของฉัน" }));
    expect(getCurrentPosition).toHaveBeenCalledTimes(2);
    act(() => callbacks[1].success(position(14.5, 101.5)));
    expect(onSelect).not.toHaveBeenCalled();
  });

  it("shows approximate accuracy and warns when accuracy is over 100 metres", () => {
    const { callbacks } = installGeolocation();
    render(<PickerHarness onSelect={vi.fn()} />);

    fireEvent.click(screen.getByRole("button", { name: "ไปยังตำแหน่งของฉัน" }));
    act(() => callbacks[0].success(position(14.5, 101.5, 101.4)));

    expect(screen.getByText("ตำแหน่งของคุณอาจคลาดเคลื่อนประมาณ 101 เมตร")).toBeInTheDocument();
    expect(screen.getByText("ตำแหน่งมีความคลาดเคลื่อนสูง กรุณาตรวจและปรับหมุดก่อนส่งรายงาน")).toBeInTheDocument();
    expect(circles.at(-1)).toMatchObject({ radius: 101.4 });
  });

  it("reports a synchronous geolocation throw and exits loading", () => {
    const { getCurrentPosition } = installGeolocation();
    getCurrentPosition.mockImplementationOnce(() => { throw new Error("geolocation unavailable"); });
    render(<PickerHarness onSelect={vi.fn()} />);

    fireEvent.click(screen.getByRole("button", { name: "ไปยังตำแหน่งของฉัน" }));

    expect(screen.getByRole("alert")).toHaveTextContent("ไม่พบตำแหน่งปัจจุบัน");
    expect(screen.getByRole("button", { name: "ไปยังตำแหน่งของฉัน" })).not.toBeDisabled();
  });

  it("uses the map center as a keyboard-accessible non-coordinate selection", () => {
    const onSelect = vi.fn();
    render(<PickerHarness onSelect={onSelect} />);

    fireEvent.click(screen.getByRole("button", { name: "เลือกจุดกึ่งกลางแผนที่" }));

    expect(onSelect).toHaveBeenLastCalledWith({ type: "Point", coordinates: [101.4, 14.4] });
  });

  it("warns after moving an existing incident and clears the incident on reset", () => {
    const onSelect = vi.fn();
    const initialLocation: ReportLocation = { type: "Point", coordinates: [101.1, 14.2] };
    const view = render(<ReportLocationPicker location={initialLocation} onChange={onSelect} resetKey={1} />);

    act(() => mapEvents.at(-1)?.click?.({ latlng: { lng: 101.2, lat: 14.3 } }));
    expect(screen.getByRole("status")).toHaveTextContent("เปลี่ยนจุดเกิดเหตุแล้ว");

    act(() => view.rerender(<ReportLocationPicker location={null} onChange={onSelect} resetKey={2} />));
    expect(screen.getByRole("status")).toHaveTextContent("ยังไม่ได้เลือกจุดเกิดเหตุ");
  });

  it("does not let a late explicit-use response overwrite a manual map selection", () => {
    const { callbacks, getCurrentPosition } = installGeolocation();
    const onSelect = vi.fn();
    render(<PickerHarness onSelect={onSelect} />);

    fireEvent.click(screen.getByRole("button", { name: "ใช้ตำแหน่งของฉันเป็นจุดเกิดเหตุ" }));
    act(() => mapEvents.at(-1)?.click?.({ latlng: { lng: 101.2, lat: 14.3 } }));
    expect(onSelect).toHaveBeenLastCalledWith({ type: "Point", coordinates: [101.2, 14.3] });
    expect(screen.getByRole("button", { name: "ไปยังตำแหน่งของฉัน" })).not.toBeDisabled();
    expect(screen.getByRole("button", { name: "ไปยังตำแหน่งของฉัน" })).toHaveAttribute("aria-busy", "false");
    expect(screen.getByRole("button", { name: "ใช้ตำแหน่งของฉันเป็นจุดเกิดเหตุ" })).not.toBeDisabled();

    fireEvent.click(screen.getByRole("button", { name: "ไปยังตำแหน่งของฉัน" }));
    expect(getCurrentPosition).toHaveBeenCalledTimes(2);

    act(() => callbacks[0].success(position(14.6, 101.6)));
    expect(onSelect).toHaveBeenCalledTimes(1);
  });

  it("ignores a pending geolocation response after the incident reset key changes", () => {
    const { callbacks } = installGeolocation();
    const onSelect = vi.fn();
    const view = render(<ReportLocationPicker location={null} onChange={onSelect} resetKey={1} />);

    fireEvent.click(screen.getByRole("button", { name: "ไปยังตำแหน่งของฉัน" }));
    act(() => view.rerender(<ReportLocationPicker location={null} onChange={onSelect} resetKey={2} />));
    act(() => callbacks[0].success(position(14.6, 101.6)));

    expect(onSelect).not.toHaveBeenCalled();
    expect(map.setView).not.toHaveBeenCalled();
  });

  it("ignores invalid and late geolocation callbacks after unmount", () => {
    const { callbacks } = installGeolocation();
    const onSelect = vi.fn();
    const view = render(<PickerHarness onSelect={onSelect} />);
    fireEvent.click(screen.getByRole("button", { name: "ไปยังตำแหน่งของฉัน" }));
    act(() => callbacks[0].success(position(14.5, Number.NaN)));
    expect(screen.getByRole("alert")).toHaveTextContent("ไม่พบตำแหน่งปัจจุบัน");

    fireEvent.click(screen.getByRole("button", { name: "ไปยังตำแหน่งของฉัน" }));
    view.unmount();
    act(() => callbacks[1].success(position(14.6, 101.6)));

    expect(onSelect).not.toHaveBeenCalled();
    expect(map.setView).not.toHaveBeenCalled();
  });
});
