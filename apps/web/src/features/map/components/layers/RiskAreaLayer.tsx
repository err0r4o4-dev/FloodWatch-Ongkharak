import { GeoJSON } from "react-leaflet";
import type { MapSelectHandler, RiskFeature } from "../../types";
import { effectiveRiskLevel } from "../../utils";

export function RiskAreaLayer({ items, onSelect }: { items: RiskFeature[]; onSelect: MapSelectHandler }) {
  return <>{items.map((feature) => {
    const level = effectiveRiskLevel(feature);
    const color = level === "HIGH" || level === "VERY_HIGH" ? "var(--color-status-danger)" : level === "MODERATE" ? "var(--color-status-warning)" : "var(--color-ink-muted)";
    return <GeoJSON key={feature.id} data={feature.geometry as Parameters<typeof GeoJSON>[0]["data"]} style={{ color, fillColor: color, fillOpacity: 0.16, weight: 2 }} eventHandlers={{ click: (event) => onSelect({ layer: "risk", feature }, [event.latlng.lng, event.latlng.lat]) }} />;
  })}</>;
}
