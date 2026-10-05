import { MapMarker } from "../MapMarker";
import type { CommunityReportFeature, MapSelectHandler } from "../../types";
import { Circle } from "react-leaflet";
import { Polyline } from "react-leaflet";
import { Fragment } from "react";
import { areaDepthColor } from "../../../report-areas/presentation";
import { isRoadSegmentReportArea } from "../../../report-areas/types";

export function CommunityReportLayer({ items, onSelect }: { items: CommunityReportFeature[]; onSelect: MapSelectHandler }) {
  return <>{items.map((feature) => <Fragment key={feature.id}>
    {isRoadSegmentReportArea(feature) ? <Polyline positions={feature.geometry.coordinates.map(([longitude, latitude]) => [latitude, longitude] as [number, number])} pathOptions={{ color: areaDepthColor(feature.depthBand, feature.freshness, feature.latestObservedAt), opacity: 0.95, weight: 7, dashArray: "12 8" }} eventHandlers={{ click: () => onSelect({ layer: "reports", feature }, feature.location.coordinates) }} /> : <Circle center={[feature.location.coordinates[1], feature.location.coordinates[0]]} radius={feature.radiusM} pathOptions={{ color: areaDepthColor(feature.depthBand, feature.freshness, feature.latestObservedAt), fillColor: areaDepthColor(feature.depthBand, feature.freshness, feature.latestObservedAt), fillOpacity: 0.14, weight: 2 }} eventHandlers={{ click: () => onSelect({ layer: "reports", feature }, feature.location.coordinates) }} />}
    <MapMarker selection={{ layer: "reports", feature }} position={feature.location.coordinates} onSelect={onSelect} />
  </Fragment>)}</>;
}
