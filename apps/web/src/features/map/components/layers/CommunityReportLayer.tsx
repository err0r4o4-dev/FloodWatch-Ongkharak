import { CircleMarker } from "react-leaflet";
import type { CommunityReportFeature, MapSelection } from "../../types";

export function CommunityReportLayer({ items, onSelect }: { items: CommunityReportFeature[]; onSelect: (selection: MapSelection) => void }) {
  return <>{items.map((feature) => <CircleMarker key={feature.id} center={[feature.location.coordinates[1], feature.location.coordinates[0]]} radius={8} pathOptions={{ color: "var(--color-source-community-ink)", fillColor: "var(--color-source-community)", fillOpacity: 0.95, weight: 3 }} eventHandlers={{ click: () => onSelect({ layer: "reports", feature }) }} />)}</>;
}
