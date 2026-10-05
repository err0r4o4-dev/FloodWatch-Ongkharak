import { divIcon } from "leaflet";
import { Marker } from "react-leaflet";
import type { MapSelectHandler, MapSelection, Position } from "../types";
import { selectionName } from "../utils";
import { useTranslation } from "../../../i18n";
import th from "../../../i18n/locales/th/map";
import en from "../../../i18n/locales/en/map";

const markerSymbols = { water: "≋", flood: "!" };

export function MapMarker({ selection, position, status = "UNKNOWN", onSelect }: {
  selection: MapSelection & { layer: "water" | "flood" | "reports" };
  position: Position;
  status?: string;
  onSelect: MapSelectHandler;
}) {
  const { t } = useTranslation({ th, en });
  const cluster = selection.layer === "reports" ? selection.feature : null;
  const tone = status === "WARNING" || status === "CRITICAL" ? "danger" : status === "WATCH" ? "warning" : "unknown";
  const title = cluster ? t("cluster.label", { name: cluster.name || t("cluster.name"), count: cluster.reports.length }) : selectionName(selection);
  const icon = cluster ? divIcon({
    className: "map-marker map-marker--reports map-marker--cluster",
    html: `<span class="map-marker__cluster-count" aria-hidden="true">${cluster.reports.length}</span>`,
    iconSize: [44, 44], iconAnchor: [22, 22], popupAnchor: [0, -22]
  }) : divIcon({
    className: `map-marker map-marker--${selection.layer} map-marker--${tone}`,
    html: `<span aria-hidden="true">${selection.layer === "water" ? markerSymbols.water : markerSymbols.flood}</span>`,
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
