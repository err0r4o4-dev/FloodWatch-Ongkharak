import { useEffect, useRef } from "react";
import { DomEvent, geoJSON, latLngBounds } from "leaflet";
import { CircleMarker, MapContainer, TileLayer, Tooltip, useMap, useMapEvents } from "react-leaflet";
import "leaflet/dist/leaflet.css";
import { FloodLayer } from "./layers/FloodLayer";
import { RiskAreaLayer } from "./layers/RiskAreaLayer";
import { RoadLayer } from "./layers/RoadLayer";
import { WaterStationLayer } from "./layers/WaterStationLayer";
import { CommunityReportLayer } from "./layers/CommunityReportLayer";
import type { LayerItems, LayerVisibility, MapBounds, MapSelection, Position } from "../types";
import { validBounds } from "../utils";
import { parseRoutePoint } from "../utils";
import type { Picking, RoutePoints } from "./RoutePanel";
import type { MapMode } from "../types";

const initialCenter: [number, number] = [14.1, 101.0];
const initialZoom = 11;

function ViewportEvents({ onBoundsChange, onMapPoint }: {
  onBoundsChange: (bounds: MapBounds) => void;
  onMapPoint: (position: Position) => void;
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
  useEffect(() => { publish(); }, [map]);
  return null;
}

function MapControls({ items }: { items: LayerItems }) {
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
    <button type="button" onClick={() => map.setView(initialCenter, initialZoom)}>กลับมุมมองเริ่มต้น</button>
    <button type="button" disabled={!hasFeatures} onClick={fitFeatures}>ปรับให้พอดีกับข้อมูล</button>
  </div>;
}

export function SituationMap({ items, visible, mode, points, picking, onBoundsChange, onMapPoint, onSelect }: {
  items: LayerItems;
  visible: LayerVisibility;
  mode: MapMode;
  points: RoutePoints;
  picking: Picking;
  onBoundsChange: (bounds: MapBounds) => void;
  onMapPoint: (position: Position) => void;
  onSelect: (selection: MapSelection) => void;
}) {
  return <div className={`situation-map${picking ? " is-picking" : ""}`} aria-label="แผนที่สถานการณ์แบบโต้ตอบ">
    <MapContainer center={initialCenter} zoom={initialZoom} minZoom={8} maxZoom={16} zoomControl={false} scrollWheelZoom={false} className="leaflet-map">
      <TileLayer url="https://tile.openstreetmap.org/{z}/{x}/{y}.png" attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors' />
      <ViewportEvents onBoundsChange={onBoundsChange} onMapPoint={onMapPoint} />
      {visible.risk && <RiskAreaLayer items={items.risk} onSelect={onSelect} />}
      {visible.flood && <FloodLayer items={items.flood} onSelect={onSelect} />}
      {visible.roads && <RoadLayer items={items.roads} onSelect={onSelect} />}
      {visible.water && <WaterStationLayer items={items.water} onSelect={onSelect} />}
      {visible.reports && <CommunityReportLayer items={items.reports} onSelect={onSelect} />}
      {mode === "ROUTE" && (["origin", "destination"] as const).map((key) => {
        const position = parseRoutePoint(points[key]);
        return position ? <CircleMarker key={key} center={[position[1], position[0]]} radius={9} pathOptions={{ color: "var(--color-primary-strong)", fillColor: "var(--color-surface)", fillOpacity: 1, weight: 3 }}><Tooltip permanent direction="top">{key === "origin" ? "จุดเริ่มต้น" : "จุดหมาย"}</Tooltip></CircleMarker> : null;
      })}
      <MapControls items={items} />
    </MapContainer>
  </div>;
}
