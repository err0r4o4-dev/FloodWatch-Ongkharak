import { MapMarker } from "../MapMarker";
import type { CommunityReportFeature, MapSelectHandler } from "../../types";

export function CommunityReportLayer({ items, onSelect }: { items: CommunityReportFeature[]; onSelect: MapSelectHandler }) {
  return <>{items.map((feature) => <MapMarker key={feature.id} selection={{ layer: "reports", feature }} position={feature.location.coordinates} onSelect={onSelect} />)}</>;
}
