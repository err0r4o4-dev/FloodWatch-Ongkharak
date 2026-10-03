import { divIcon } from "leaflet";
import { Marker } from "react-leaflet";
import type { MapSelectHandler, MapSelection, Position } from "../types";
import { selectionName } from "../utils";

const markerSymbols = { water: "≋", flood: "!", reports: "●" };

export function MapMarker({ selection, position, status = "UNKNOWN", onSelect }: {
  selection: MapSelection & { layer: "water" | "flood" | "reports" };
  position: Position;
  status?: string;
  onSelect: MapSelectHandler;
}) {
  const tone = status === "WARNING" || status === "CRITICAL" ? "danger" : status === "WATCH" ? "warning" : "unknown";
  const icon = divIcon({
    className: `map-marker map-marker--${selection.layer} map-marker--${tone}`,
    html: `<span aria-hidden="true">${markerSymbols[selection.layer]}</span>`,
    iconSize: [36, 36], iconAnchor: [18, 18], popupAnchor: [0, -18]
  });
  return <Marker position={[position[1], position[0]]} icon={icon} title={selectionName(selection)} alt={selectionName(selection)} eventHandlers={{
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
