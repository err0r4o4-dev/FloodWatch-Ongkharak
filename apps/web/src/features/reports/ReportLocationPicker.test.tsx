import { act, fireEvent, render, screen } from "@testing-library/react";
import { useState } from "react";
import { describe, expect, it, vi } from "vitest";
import { ReportLocationPicker } from "./ReportLocationPicker";
import type { ReportLocation } from "./types";

const mapEvents = vi.hoisted(() => ({ click: null as null | ((event: { latlng: { lng: number; lat: number } }) => void) }));

vi.mock("react-leaflet", () => ({
  CircleMarker: () => null,
  MapContainer: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  TileLayer: () => null,
  useMapEvents: (handlers: { click: (event: { latlng: { lng: number; lat: number } }) => void }) => {
    mapEvents.click = handlers.click;
    return null;
  }
}));

function PickerHarness({ onSelect, initialLocation = null }: { onSelect: (location: ReportLocation | null) => void; initialLocation?: ReportLocation | null }) {
  const [location, setLocation] = useState<ReportLocation | null>(initialLocation);
  const change = (next: ReportLocation | null) => {
    setLocation(next);
    onSelect(next);
  };
  return <ReportLocationPicker location={location} onChange={change} resetKey="test" />;
}

describe("ReportLocationPicker", () => {
  it("uses the clicked Leaflet location as longitude then latitude", () => {
    const onSelect = vi.fn();
    render(<PickerHarness onSelect={onSelect} />);

    act(() => mapEvents.click?.({ latlng: { lng: 101.2, lat: 14.3 } }));

    expect(onSelect).toHaveBeenLastCalledWith({ type: "Point", coordinates: [101.2, 14.3] });
    expect(screen.getByLabelText("ลองจิจูด")).toHaveValue(101.2);
    expect(screen.getByLabelText("ละติจูด")).toHaveValue(14.3);
  });

  it("keeps coordinate drafts until both values form a valid point", () => {
    const onSelect = vi.fn();
    render(<PickerHarness onSelect={onSelect} />);

    fireEvent.change(screen.getByLabelText("ลองจิจูด"), { target: { value: "101.1" } });
    expect(screen.getByLabelText("ลองจิจูด")).toHaveValue(101.1);
    expect(onSelect).toHaveBeenLastCalledWith(null);
    fireEvent.change(screen.getByLabelText("ละติจูด"), { target: { value: "14.2" } });

    expect(screen.getByLabelText("ลองจิจูด")).toHaveValue(101.1);
    expect(screen.getByLabelText("ละติจูด")).toHaveValue(14.2);
    expect(onSelect).toHaveBeenLastCalledWith({ type: "Point", coordinates: [101.1, 14.2] });
  });

  it("retains the companion draft when an existing location becomes temporarily invalid", () => {
    const onSelect = vi.fn();
    render(<PickerHarness onSelect={onSelect} initialLocation={{ type: "Point", coordinates: [101.1, 14.2] }} />);

    fireEvent.change(screen.getByLabelText("ลองจิจูด"), { target: { value: "" } });
    expect(screen.getByLabelText("ละติจูด")).toHaveValue(14.2);
    expect(onSelect).toHaveBeenLastCalledWith(null);
    fireEvent.change(screen.getByLabelText("ลองจิจูด"), { target: { value: "101.25" } });

    expect(onSelect).toHaveBeenLastCalledWith({ type: "Point", coordinates: [101.25, 14.2] });
  });
});
