import { CircleMarker } from "react-leaflet";
import type { MapSelection, WaterFeature } from "../../types";

export function WaterStationLayer({ items, onSelect }: { items: WaterFeature[]; onSelect: (selection: MapSelection) => void }) {
  return <>{items.map((feature) => <CircleMarker key={feature.id} center={[feature.location.coordinates[1], feature.location.coordinates[0]]} radius={7} pathOptions={{ color: "var(--color-primary-strong)", fillColor: "var(--color-surface)", fillOpacity: 0.95, weight: 3 }} eventHandlers={{ click: () => onSelect({ layer: "water", feature }) }} />)}</>;
}
