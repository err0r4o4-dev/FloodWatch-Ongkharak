import { MapMarker } from "../MapMarker";
import type { MapSelectHandler, WaterFeature } from "../../types";

export function WaterStationLayer({ items, onSelect }: { items: WaterFeature[]; onSelect: MapSelectHandler }) {
  return <>{items.map((feature) => <MapMarker key={feature.id} selection={{ layer: "water", feature }} position={feature.location.coordinates} onSelect={onSelect} />)}</>;
}
