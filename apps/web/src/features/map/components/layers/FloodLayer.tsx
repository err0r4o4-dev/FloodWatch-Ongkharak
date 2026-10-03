import { GeoJSON } from "react-leaflet";
import { MapMarker } from "../MapMarker";
import type { FloodFeature, MapSelectHandler } from "../../types";
import { effectiveFloodSeverity } from "../../utils";

export function FloodLayer({ items, onSelect }: { items: FloodFeature[]; onSelect: MapSelectHandler }) {
  return <>{items.map((feature) => {
    const severity = effectiveFloodSeverity(feature);
    const color = severity === "CRITICAL" || severity === "WARNING" ? "var(--color-status-danger)" : severity === "WATCH" ? "var(--color-status-warning)" : "var(--color-ink-muted)";
    return feature.geometry.type === "Point"
      ? <MapMarker key={feature.id} selection={{ layer: "flood", feature }} position={feature.geometry.coordinates} status={severity} onSelect={onSelect} />
      : <GeoJSON key={feature.id} data={feature.geometry as Parameters<typeof GeoJSON>[0]["data"]} style={{ color, fillColor: color, fillOpacity: 0.2, weight: 2 }} eventHandlers={{ click: (event) => onSelect({ layer: "flood", feature }, [event.latlng.lng, event.latlng.lat]) }} />;
  })}</>;
}
