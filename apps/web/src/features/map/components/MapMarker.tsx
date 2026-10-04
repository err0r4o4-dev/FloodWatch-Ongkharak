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
  const icon = divIcon({
    className: `map-marker map-marker--${selection.layer} map-marker--${tone}${areaClass}`,
    html: area
      ? `<span class="map-marker__depth" aria-hidden="true">${markerSymbols.reports}</span><span class="map-marker__trend map-marker__trend--${areaTrendTone(trend)}" aria-hidden="true">${areaTrendSymbol(trend)}</span>`
      : `<span aria-hidden="true">${markerSymbols[selection.layer]}</span>`,
    iconSize: [36, 36], iconAnchor: [18, 18], popupAnchor: [0, -18]
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
