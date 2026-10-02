import { CircleMarker, GeoJSON } from "react-leaflet";
import type { FloodFeature, MapSelection } from "../../types";
import { effectiveFloodSeverity } from "../../utils";

export function FloodLayer({ items, onSelect }: { items: FloodFeature[]; onSelect: (selection: MapSelection) => void }) {
  return <>{items.map((feature) => {
    const selected = () => onSelect({ layer: "flood", feature });
    const severity = effectiveFloodSeverity(feature);
    const color = severity === "CRITICAL" || severity === "WARNING" ? "var(--color-status-danger)" : severity === "WATCH" ? "var(--color-status-warning)" : "var(--color-ink-muted)";
    return feature.geometry.type === "Point"
      ? <CircleMarker key={feature.id} center={[feature.geometry.coordinates[1], feature.geometry.coordinates[0]]} radius={9} pathOptions={{ color, fillColor: color, fillOpacity: 0.75, weight: 2 }} eventHandlers={{ click: selected }} />
      : <GeoJSON key={feature.id} data={feature.geometry as Parameters<typeof GeoJSON>[0]["data"]} style={{ color, fillColor: color, fillOpacity: 0.2, weight: 2 }} eventHandlers={{ click: selected }} />;
  })}</>;
}
