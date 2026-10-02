import { useEffect, useState } from "react";
import { CircleMarker, MapContainer, TileLayer, useMapEvents } from "react-leaflet";
import "leaflet/dist/leaflet.css";
import type { ReportLocation } from "./types";

const initialCenter: [number, number] = [14.1, 101.0];

function LocationEvents({ onSelect }: { onSelect: (location: ReportLocation) => void }) {
  useMapEvents({
    click: (event) => onSelect({ type: "Point", coordinates: [event.latlng.lng, event.latlng.lat] })
  });
  return null;
}

export function ReportLocationPicker({ location, onChange, resetKey }: {
  location: ReportLocation | null;
  onChange: (location: ReportLocation | null) => void;
  resetKey: string | number;
}) {
  const [longitudeDraft, setLongitudeDraft] = useState(location?.coordinates[0].toString() ?? "");
  const [latitudeDraft, setLatitudeDraft] = useState(location?.coordinates[1].toString() ?? "");

  useEffect(() => {
    setLongitudeDraft(location?.coordinates[0].toString() ?? "");
    setLatitudeDraft(location?.coordinates[1].toString() ?? "");
  }, [resetKey]);

  const selectMapLocation = (next: ReportLocation) => {
    setLongitudeDraft(next.coordinates[0].toString());
    setLatitudeDraft(next.coordinates[1].toString());
    onChange(next);
  };

  const changeCoordinate = (index: 0 | 1, value: string) => {
    const nextLongitude = index === 0 ? value : longitudeDraft;
    const nextLatitude = index === 1 ? value : latitudeDraft;
    if (index === 0) setLongitudeDraft(value); else setLatitudeDraft(value);
    const longitude = Number(nextLongitude);
    const latitude = Number(nextLatitude);
    onChange(nextLongitude !== "" && nextLatitude !== "" && Number.isFinite(longitude) && Number.isFinite(latitude) && Math.abs(longitude) <= 180 && Math.abs(latitude) <= 90
      ? { type: "Point", coordinates: [longitude, latitude] }
      : null);
  };

  return <fieldset className="report-location-picker">
    <legend>ตำแหน่งเหตุการณ์</legend>
    <p>แตะแผนที่เพื่อเลือกตำแหน่ง หรือกรอกพิกัดด้วยตนเอง ข้อมูลตำแหน่งจะถูกส่งพร้อมรายงาน</p>
    <div className="report-location-map" aria-label="แผนที่เลือกตำแหน่งรายงาน">
      <MapContainer center={location ? [location.coordinates[1], location.coordinates[0]] : initialCenter} zoom={11} minZoom={8} maxZoom={16} className="leaflet-map" scrollWheelZoom={false}>
        <TileLayer url="https://tile.openstreetmap.org/{z}/{x}/{y}.png" attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors' />
        <LocationEvents onSelect={selectMapLocation} />
        {location && <CircleMarker center={[location.coordinates[1], location.coordinates[0]]} radius={9} pathOptions={{ color: "#2563eb", fillColor: "#ffffff", fillOpacity: 1, weight: 3 }} />}
      </MapContainer>
    </div>
    <div className="report-coordinate-fields">
      <label>ลองจิจูด<input type="number" inputMode="decimal" step="any" value={longitudeDraft} onChange={(event) => changeCoordinate(0, event.target.value)} /></label>
      <label>ละติจูด<input type="number" inputMode="decimal" step="any" value={latitudeDraft} onChange={(event) => changeCoordinate(1, event.target.value)} /></label>
    </div>
  </fieldset>;
}
