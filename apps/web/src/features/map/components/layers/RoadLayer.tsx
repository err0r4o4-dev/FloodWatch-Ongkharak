import { GeoJSON } from "react-leaflet";
import type { MapSelection, RoadFeature } from "../../types";
import { effectiveRoadStatus } from "../../utils";

export function RoadLayer({ items, onSelect }: { items: RoadFeature[]; onSelect: (selection: MapSelection) => void }) {
  return <>{items.map((feature) => {
    const status = effectiveRoadStatus(feature.currentCondition);
    const color = status === "CLOSED" ? "var(--color-status-danger)" : status === "WATCH" || status === "SMALL_VEHICLE_NOT_RECOMMENDED" ? "var(--color-status-warning)" : status === "PASSABLE" ? "var(--color-status-success)" : "var(--color-ink-muted)";
    return <GeoJSON key={feature.id} data={feature.geometry as Parameters<typeof GeoJSON>[0]["data"]} style={{ color, weight: 5, opacity: 0.85, dashArray: status === "UNKNOWN" ? "5 7" : undefined }} eventHandlers={{ click: () => onSelect({ layer: "roads", feature }) }} />;
  })}</>;
}
