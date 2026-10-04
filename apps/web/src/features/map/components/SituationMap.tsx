import { useEffect, useRef, useState } from "react";
import { geoJSON, latLngBounds } from "leaflet";
import type { Popup as LeafletPopup } from "leaflet";
import { CircleMarker, MapContainer, Popup, TileLayer, Tooltip, useMap, useMapEvents } from "react-leaflet";
import "leaflet/dist/leaflet.css";
import { FloodLayer } from "./layers/FloodLayer";
import { RiskAreaLayer } from "./layers/RiskAreaLayer";
import { RoadLayer } from "./layers/RoadLayer";
import { WaterStationLayer } from "./layers/WaterStationLayer";
import { CommunityReportLayer } from "./layers/CommunityReportLayer";
import { FeaturePopup } from "./FeaturePopup";
import type { LayerItems, LayerVisibility, MapBounds, MapSelection, Position } from "../types";
import { geometryBounds, mapSelections, selectionGeometry, validBounds } from "../utils";
import { parseRoutePoint } from "../utils";
import type { Picking, RoutePoints } from "./RoutePanel";
import type { MapMode } from "../types";
import { useTranslation } from "../../../i18n";
import th from "../../../i18n/locales/th/map";
import en from "../../../i18n/locales/en/map";
import { getMapSession, mapSettings } from "../../../shared/map/mapSession";
import { useUserLocation } from "../../../shared/map/useUserLocation";
import type { UserLocationState } from "../../../shared/map/useUserLocation";
import { MapLocationError, MapNavigation, SharedMapViewport, UserLocationMarker } from "../../../shared/map/MapNavigation";
import "./MapPopup.css";

const initialCenter = mapSettings.initialCenter;
const initialZoom = mapSettings.initialZoom;

function ViewportEvents({ onBoundsChange, onMapPoint, initialBounds }: {
  onBoundsChange: (bounds: MapBounds) => void;
  onMapPoint: (position: Position) => void;
  initialBounds?: MapBounds;
}) {
  const map = useMapEvents({
    moveend: () => publish(),
    zoomend: () => publish(),
    click: (event) => onMapPoint([event.latlng.lng, event.latlng.lat])
  });
  const publish = () => {
    const bounds = map.getBounds();
    const next = { west: bounds.getWest(), south: bounds.getSouth(), east: bounds.getEast(), north: bounds.getNorth() };
    if (validBounds(next)) onBoundsChange(next);
  };
  useEffect(() => {
    if (initialBounds) map.fitBounds([[initialBounds.south, initialBounds.west], [initialBounds.north, initialBounds.east]], { padding: [48, 48], maxZoom: 14, animate: false });
    publish();
    if (typeof ResizeObserver === "undefined") return;
    const observer = new ResizeObserver(() => { map.invalidateSize({ pan: true, animate: false }); publish(); });
    observer.observe(map.getContainer());
    return () => observer.disconnect();
  }, [map]);
  return null;
}

function MapControls({ items, compact = false, location }: { items: LayerItems; compact?: boolean; location: UserLocationState }) {
  const { t } = useTranslation({ th, en });
  const map = useMap();
  const fitFeatures = () => {
    const bounds = latLngBounds([]);
    items.water.forEach((station) => bounds.extend([station.location.coordinates[1], station.location.coordinates[0]]));
    items.reports.forEach((report) => bounds.extend([report.location.coordinates[1], report.location.coordinates[0]]));
    [...items.flood, ...items.risk, ...items.roads].forEach((item) => {
      const layer = geoJSON(item.geometry as Parameters<typeof geoJSON>[0]);
      bounds.extend(layer.getBounds());
    });
    if (bounds.isValid()) map.fitBounds(bounds, { padding: [24, 24], maxZoom: 14 });
  };
  const hasFeatures = Object.values(items).some((collection) => collection.length > 0);
  return <MapNavigation location={location}>
    <button type="button" aria-label={t("map.reset")} title={t("map.reset")} onClick={() => map.setView(initialCenter, initialZoom)}>{compact ? "⌖" : t("map.reset")}</button>
    <button type="button" aria-label={t("map.fit")} title={t("map.fit")} disabled={!hasFeatures} onClick={fitFeatures}>{compact ? "⤢" : t("map.fit")}</button>
  </MapNavigation>;
}

function MapFeaturePopup({ selection, position, onClose, onDetails }: {
  selection: MapSelection;
  position: Position;
  onClose: () => void;
  onDetails: (selection: MapSelection) => void;
}) {
  const popup = useRef<LeafletPopup>(null);
  const map = useMap();
  const narrow = typeof window !== "undefined" && window.innerWidth <= 880;
  const legendHeight = narrow && typeof document !== "undefined" ? document.querySelector<HTMLElement>(".map-page__legend")?.getBoundingClientRect().height ?? 0 : 0;
  const autoPanPadding = narrow ? { topLeft: [16, 80] as [number, number], bottomRight: [112, Math.max(120, Math.ceil(legendHeight) + 16)] as [number, number] } : { topLeft: [16, 80] as [number, number], bottomRight: [16, 80] as [number, number] };
  useEffect(() => {
    const content = popup.current?.getElement()?.querySelector(".leaflet-popup-content");
    if (!content || typeof ResizeObserver === "undefined") return;
    const observer = new ResizeObserver(() => popup.current?.update());
    observer.observe(content);
    return () => observer.disconnect();
  }, []);
  return <Popup ref={popup} position={[position[1], position[0]]} className="feature-map-popup" closeButton={false} closeOnClick={false} autoPanPaddingTopLeft={autoPanPadding.topLeft} autoPanPaddingBottomRight={autoPanPadding.bottomRight} minWidth={240} maxWidth={300}>
    <FeaturePopup selection={selection} onClose={onClose} onDetails={onDetails} restoreFocus={() => map.getContainer().focus({ preventScroll: true })} />
  </Popup>;
}

function InteractiveLayers({ items, visible, mode, enablePopups, focusSelection, initialRiskId, onSelect, onPreview }: {
  items: LayerItems;
  visible: LayerVisibility;
  mode: MapMode;
  enablePopups: boolean;
  focusSelection?: MapSelection | null;
  initialRiskId?: string | null;
  onSelect: (selection: MapSelection) => void;
  onPreview?: (selection: MapSelection | null) => void;
}) {
  const [popup, setPopup] = useState<{ selection: MapSelection; position: Position } | null>(null);
  const initialRiskOpened = useRef(false);
  const autoPanning = useRef(false);
  const previousMode = useRef(mode);
  const map = useMapEvents({
    autopanstart: () => { autoPanning.current = true; },
    movestart: () => { if (!autoPanning.current) setPopup(null); },
    moveend: () => { autoPanning.current = false; }
  });
  const preview = (selection: MapSelection, position?: Position) => {
    if (!enablePopups) { onSelect(selection); return; }
    if (typeof window.matchMedia === "function" && window.matchMedia("(max-width: 55rem)").matches) map.getContainer().scrollIntoView({ block: "center", behavior: "instant" });
    const bounds = geometryBounds(selectionGeometry(selection));
    setPopup({ selection, position: position ?? [(bounds.west + bounds.east) / 2, (bounds.south + bounds.north) / 2] });
    onPreview?.(selection);
  };
  useEffect(() => {
    if (popup && !visible[popup.selection.layer]) setPopup(null);
  }, [popup, visible]);
  useEffect(() => {
    if (previousMode.current !== mode) { setPopup(null); previousMode.current = mode; }
  }, [mode]);
  useEffect(() => {
    if (!focusSelection) return;
    const bounds = geometryBounds(selectionGeometry(focusSelection));
    map.fitBounds([[bounds.south, bounds.west], [bounds.north, bounds.east]], { padding: [72, 72], maxZoom: 14, animate: false });
    preview(focusSelection);
  }, [focusSelection, map]);
  useEffect(() => {
    if (!initialRiskId || initialRiskOpened.current) return;
    const feature = items.risk.find((item) => item.id === initialRiskId);
    if (feature) { initialRiskOpened.current = true; preview({ layer: "risk", feature }); }
  }, [initialRiskId, items.risk]);
  const current = popup && visible[popup.selection.layer] ? mapSelections(items).find((item) => item.layer === popup.selection.layer && item.feature.id === popup.selection.feature.id) : null;
  return <>
    {visible.risk && <RiskAreaLayer items={items.risk} onSelect={preview} />}
    {visible.flood && <FloodLayer items={items.flood} onSelect={preview} />}
    {visible.roads && <RoadLayer items={items.roads} onSelect={preview} />}
    {visible.water && <WaterStationLayer items={items.water} onSelect={preview} />}
    {visible.reports && <CommunityReportLayer items={items.reports} onSelect={preview} />}
    {popup && current && <MapFeaturePopup key={`${current.layer}-${current.feature.id}`} selection={current} position={popup.position} onClose={() => setPopup(null)} onDetails={(selection) => { setPopup(null); onSelect(selection); }} />}
  </>;
}

export function SituationMap({ items, visible, mode, points, picking, onBoundsChange, onMapPoint, onSelect, onPreview, enablePopups = false, compactControls = enablePopups, focusSelection, initialBounds, initialRiskId }: {
  items: LayerItems;
  visible: LayerVisibility;
  mode: MapMode;
  points: RoutePoints;
  picking: Picking;
  onBoundsChange: (bounds: MapBounds) => void;
  onMapPoint: (position: Position) => void;
  onSelect: (selection: MapSelection) => void;
  onPreview?: (selection: MapSelection | null) => void;
  enablePopups?: boolean;
  compactControls?: boolean;
  focusSelection?: MapSelection | null;
  initialBounds?: MapBounds;
  initialRiskId?: string | null;
}) {
  const { t } = useTranslation({ th, en });
  const location = useUserLocation();
  const [viewport] = useState(() => enablePopups && !initialBounds ? getMapSession().viewport : null);

  return <div className={`situation-map${picking ? " is-picking" : ""}`} aria-label={t("map.label")}>
    <MapContainer center={viewport?.center ?? initialCenter} zoom={viewport?.zoom ?? initialZoom} minZoom={mapSettings.minZoom} maxZoom={mapSettings.maxZoom} zoomControl={false} scrollWheelZoom={true} className="leaflet-map">
      <SharedMapViewport enabled={enablePopups} observeResize={false} />
      <TileLayer url="https://tile.openstreetmap.org/{z}/{x}/{y}.png" attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors' maxZoom={mapSettings.maxZoom} />
      <ViewportEvents onBoundsChange={onBoundsChange} onMapPoint={onMapPoint} initialBounds={initialBounds} />
      <InteractiveLayers items={items} visible={visible} mode={mode} enablePopups={enablePopups} focusSelection={focusSelection} initialRiskId={initialRiskId} onSelect={onSelect} onPreview={onPreview} />
      <UserLocationMarker position={location.position} />
      {mode === "ROUTE" && (["origin", "destination"] as const).map((key) => {
        const position = parseRoutePoint(points[key]);
        return position ? <CircleMarker key={key} center={[position[1], position[0]]} radius={9} pathOptions={{ color: "var(--color-primary-strong)", fillColor: "var(--color-surface)", fillOpacity: 1, weight: 3 }}><Tooltip permanent direction="top">{t(key === "origin" ? "map.origin" : "map.destination")}</Tooltip></CircleMarker> : null;
      })}
      <MapControls items={items} compact={compactControls} location={location} />
    </MapContainer>
    <MapLocationError error={location.error} />
  </div>;
}
