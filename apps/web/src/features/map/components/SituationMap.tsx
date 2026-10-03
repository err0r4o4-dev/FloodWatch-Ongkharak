import { useEffect, useRef, useState } from "react";
import { DomEvent, geoJSON, latLngBounds } from "leaflet";
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
import "./MapPopup.css";

const initialCenter: [number, number] = [14.1, 101.0];
const initialZoom = 11;

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
    const observer = new ResizeObserver(() => { map.invalidateSize({ pan: false }); publish(); });
    observer.observe(map.getContainer());
    return () => observer.disconnect();
  }, [map]);
  return null;
}

function MapControls({ items, compact = false }: { items: LayerItems; compact?: boolean }) {
  const map = useMap();
  const controls = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (controls.current) {
      DomEvent.disableClickPropagation(controls.current);
      DomEvent.disableScrollPropagation(controls.current);
    }
  }, []);
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
  return <div ref={controls} className="map-zoom-controls" role="group" aria-label="ควบคุมแผนที่">
    <button type="button" aria-label="ขยายแผนที่" onClick={() => map.zoomIn()}>+</button>
    <button type="button" aria-label="ย่อแผนที่" onClick={() => map.zoomOut()}>−</button>
    <button type="button" aria-label="กลับมุมมองเริ่มต้น" title="กลับมุมมองเริ่มต้น" onClick={() => map.setView(initialCenter, initialZoom)}>{compact ? "⌖" : "กลับมุมมองเริ่มต้น"}</button>
    <button type="button" aria-label="ปรับให้พอดีกับข้อมูล" title="ปรับให้พอดีกับข้อมูล" disabled={!hasFeatures} onClick={fitFeatures}>{compact ? "⤢" : "ปรับให้พอดีกับข้อมูล"}</button>
  </div>;
}

function MapFeaturePopup({ selection, position, onClose, onDetails }: {
  selection: MapSelection;
  position: Position;
  onClose: () => void;
  onDetails: (selection: MapSelection) => void;
}) {
  const popup = useRef<LeafletPopup>(null);
  const map = useMap();
  useEffect(() => {
    const content = popup.current?.getElement()?.querySelector(".leaflet-popup-content");
    if (!content || typeof ResizeObserver === "undefined") return;
    const observer = new ResizeObserver(() => popup.current?.update());
    observer.observe(content);
    return () => observer.disconnect();
  }, []);
  return <Popup ref={popup} position={[position[1], position[0]]} className="feature-map-popup" closeButton={false} closeOnClick={false} autoPanPaddingTopLeft={[16, 80]} autoPanPaddingBottomRight={[16, 80]} minWidth={240} maxWidth={300}>
    <FeaturePopup selection={selection} onClose={onClose} onDetails={onDetails} restoreFocus={() => map.getContainer().focus({ preventScroll: true })} />
  </Popup>;
}

function InteractiveLayers({ items, visible, mode, enablePopups, focusSelection, initialRiskId, onSelect }: {
  items: LayerItems;
  visible: LayerVisibility;
  mode: MapMode;
  enablePopups: boolean;
  focusSelection?: MapSelection | null;
  initialRiskId?: string | null;
  onSelect: (selection: MapSelection) => void;
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

export function SituationMap({ items, visible, mode, points, picking, onBoundsChange, onMapPoint, onSelect, enablePopups = false, focusSelection, initialBounds, initialRiskId }: {
  items: LayerItems;
  visible: LayerVisibility;
  mode: MapMode;
  points: RoutePoints;
  picking: Picking;
  onBoundsChange: (bounds: MapBounds) => void;
  onMapPoint: (position: Position) => void;
  onSelect: (selection: MapSelection) => void;
  enablePopups?: boolean;
  focusSelection?: MapSelection | null;
  initialBounds?: MapBounds;
  initialRiskId?: string | null;
}) {
  return <div className={`situation-map${picking ? " is-picking" : ""}`} aria-label="แผนที่สถานการณ์แบบโต้ตอบ">
    <MapContainer center={initialCenter} zoom={initialZoom} minZoom={8} maxZoom={16} zoomControl={false} scrollWheelZoom={false} className="leaflet-map">
      <TileLayer url="https://tile.openstreetmap.org/{z}/{x}/{y}.png" attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors' />
      <ViewportEvents onBoundsChange={onBoundsChange} onMapPoint={onMapPoint} initialBounds={initialBounds} />
      <InteractiveLayers items={items} visible={visible} mode={mode} enablePopups={enablePopups} focusSelection={focusSelection} initialRiskId={initialRiskId} onSelect={onSelect} />
      {mode === "ROUTE" && (["origin", "destination"] as const).map((key) => {
        const position = parseRoutePoint(points[key]);
        return position ? <CircleMarker key={key} center={[position[1], position[0]]} radius={9} pathOptions={{ color: "var(--color-primary-strong)", fillColor: "var(--color-surface)", fillOpacity: 1, weight: 3 }}><Tooltip permanent direction="top">{key === "origin" ? "จุดเริ่มต้น" : "จุดหมาย"}</Tooltip></CircleMarker> : null;
      })}
      <MapControls items={items} compact={enablePopups} />
    </MapContainer>
  </div>;
}
