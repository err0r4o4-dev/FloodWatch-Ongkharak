import type { MapMode } from "../types";

export function MapToolbar({ mode, onModeChange }: { mode: MapMode; onModeChange: (mode: MapMode) => void }) {
  return <div className="map-toolbar" role="group" aria-label="โหมดแผนที่">
    <button type="button" aria-pressed={mode === "SITUATION"} onClick={() => onModeChange("SITUATION")}>สถานการณ์</button>
    <button type="button" aria-pressed={mode === "ROUTE"} onClick={() => onModeChange("ROUTE")}>เส้นทาง</button>
  </div>;
}
