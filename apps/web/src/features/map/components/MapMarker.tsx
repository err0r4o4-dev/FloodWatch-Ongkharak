import { divIcon } from "leaflet";
import { Marker } from "react-leaflet";
import type { MapSelectHandler, MapSelection, Position } from "../types";
import { selectionName } from "../utils";
import { areaMarkerLabel, areaTrendSymbol, areaTrendTone, displayDepthBand, displayReportAreaTrend } from "../../report-areas/presentation";

const markerSymbols = { water: "≋", flood: "!", reports: "●" };

export function MapMarker({ selection, position, status = "UNKNOWN", onSelect }: {
  selection: MapSelection & { layer: "water" | "flood" | "reports" };
  position: Position;
  status?: string;
  onSelect: MapSelectHandler;
}) {
  const area = selection.layer === "reports" ? selection.feature : null;
  const depthBand = area ? displayDepthBand(area.depthBand, area.freshness, area.latestObservedAt) : "UNKNOWN";
  const trend = area ? displayReportAreaTrend(area.trend, area.freshness, area.latestObservedAt) : "UNKNOWN";
  const tone = status === "WARNING" || status === "CRITICAL" ? "danger" : status === "WATCH" ? "warning" : "unknown";
  const areaClass = area ? ` map-marker--depth-${depthBand.toLowerCase()}` : "";
  const title = area ? areaMarkerLabel(area) : selectionName(selection);
  const markerSize = area ? 28 : 36;
  const icon = divIcon({
    className: `map-marker map-marker--${selection.layer} map-marker--${tone}${areaClass}`,
    html: area
      ? `<span class="map-marker__pin" aria-hidden="true"><span class="map-marker__dot"></span></span><span class="map-marker__trend map-marker__trend--${areaTrendTone(trend)}" aria-hidden="true">${areaTrendSymbol(trend)}</span>`
      : `<span aria-hidden="true">${markerSymbols[selection.layer]}</span>`,
    iconSize: [markerSize, area ? 36 : markerSize], iconAnchor: [markerSize / 2, area ? 34 : markerSize / 2], popupAnchor: [0, area ? -34 : -markerSize / 2]
  });
  return <Marker position={[position[1], position[0]]} icon={icon} title={title} alt={title} eventHandlers={{
    click: () => onSelect(selection, position),
    keydown: (event) => {
      if (event.originalEvent.key === "Enter" || event.originalEvent.key === " ") {
        event.originalEvent.preventDefault();
        event.originalEvent.stopPropagation();
        onSelect(selection, position);
      }
    }
  }} />;
}
