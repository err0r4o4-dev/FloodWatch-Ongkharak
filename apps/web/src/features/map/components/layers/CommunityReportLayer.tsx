import { MapMarker } from "../MapMarker";
import type { CommunityReportFeature, MapSelectHandler } from "../../types";
import { Circle } from "react-leaflet";
import { Fragment } from "react";
import { areaDepthColor } from "../../../report-areas/presentation";

export function CommunityReportLayer({ items, onSelect }: { items: CommunityReportFeature[]; onSelect: MapSelectHandler }) {
  return <>{items.map((feature) => <Fragment key={feature.id}>
    <Circle center={[feature.location.coordinates[1], feature.location.coordinates[0]]} radius={feature.radiusM} pathOptions={{ color: areaDepthColor(feature.depthBand, feature.freshness), fillColor: areaDepthColor(feature.depthBand, feature.freshness), fillOpacity: 0.14, weight: 2 }} eventHandlers={{ click: () => onSelect({ layer: "reports", feature }, feature.location.coordinates) }} />
    <MapMarker selection={{ layer: "reports", feature }} position={feature.location.coordinates} onSelect={onSelect} />
  </Fragment>)}</>;
}
