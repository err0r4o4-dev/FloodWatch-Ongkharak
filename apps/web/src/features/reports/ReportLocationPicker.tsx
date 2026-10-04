import { useCallback, useEffect, useRef, useState } from "react";
import type { Map as LeafletMap } from "leaflet";
import { divIcon } from "leaflet";
import { Circle, CircleMarker, MapContainer, Marker, TileLayer, useMap, useMapEvents } from "react-leaflet";
import "leaflet/dist/leaflet.css";
import { useTranslation } from "../../i18n";
import th from "../../i18n/locales/th/reports";
import en from "../../i18n/locales/en/reports";
import type { ReportLocation } from "./types";
import "./ReportLocationPicker.css";

const initialCenter: [number, number] = [14.1, 101.0];
const incidentPinPath = "M16 1C8.3 1 2 7.3 2 15c0 10 14 24 14 24s14-14 14-24C30 7.3 23.7 1 16 1Z";
const incidentPinIcon = divIcon({
  className: "report-incident-pin",
  html: `<svg viewBox="0 0 32 40" aria-hidden="true"><path d="${incidentPinPath}"/><circle class="report-incident-pin__center" cx="16" cy="15" r="5"/></svg>`,
  iconSize: [32, 40],
  iconAnchor: [16, 39]
});
const geolocationOptions: PositionOptions = { enableHighAccuracy: true, timeout: 10_000, maximumAge: 0 };
type GeolocationIntent = "locate" | "select";
type LocationErrorKey = "location.denied" | "location.timeout" | "location.unavailable" | "location.unsupported";
type UserPosition = { latitude: number; longitude: number; accuracy: number };

function reportLocation(longitude: number, latitude: number): ReportLocation | null {
  if (!Number.isFinite(longitude) || !Number.isFinite(latitude) || Math.abs(longitude) > 180 || Math.abs(latitude) > 90) return null;
  return { type: "Point", coordinates: [longitude, latitude] };
}

function validUserPosition(position: GeolocationPosition): UserPosition | null {
  const { latitude, longitude, accuracy } = position.coords;
  return Number.isFinite(latitude) && Number.isFinite(longitude) && Number.isFinite(accuracy) && accuracy >= 0 &&
    Math.abs(latitude) <= 90 && Math.abs(longitude) <= 180
    ? { latitude, longitude, accuracy }
    : null;
}

function LocationEvents({ onSelect }: { onSelect: (location: ReportLocation) => void }) {
  useMapEvents({
    click: (event) => {
      const next = reportLocation(event.latlng.lng, event.latlng.lat);
      if (next) onSelect(next);
    }
  });
  return null;
}

function MapBridge({ userPosition, onReady }: { userPosition: UserPosition | null; onReady: (map: LeafletMap | null) => void }) {
  const map = useMap();

  useEffect(() => {
    onReady(map);
    return () => onReady(null);
  }, [map, onReady]);

  useEffect(() => {
    if (userPosition) map.setView([userPosition.latitude, userPosition.longitude], Math.max(map.getZoom(), 16), { animate: false });
  }, [map, userPosition]);

  return null;
}

function MapLocationControl({ locating, label, loadingLabel, onLocate }: { locating: boolean; label: string; loadingLabel: string; onLocate: () => void }) {
  return <button type="button" className="report-location-map__locate" title={locating ? loadingLabel : label} aria-busy={locating} disabled={locating} onClick={onLocate}>
    <span aria-hidden="true">◎</span><span className="report-location-picker__visually-hidden">{locating ? loadingLabel : label}</span>
  </button>;
}

export function ReportLocationPicker({ location, onChange, resetKey }: {
  location: ReportLocation | null;
  onChange: (location: ReportLocation | null) => void;
  resetKey: string | number;
}) {
  const { t } = useTranslation({ th, en });
  const mapRef = useRef<LeafletMap | null>(null);
  const mountedRef = useRef(false);
  const requestIdRef = useRef(0);
  const [userPosition, setUserPosition] = useState<UserPosition | null>(null);
  const [locating, setLocating] = useState(false);
  const [locationError, setLocationError] = useState<LocationErrorKey | null>(null);
  const [incidentChanged, setIncidentChanged] = useState(false);

  useEffect(() => {
    mountedRef.current = true;
    return () => { mountedRef.current = false; };
  }, []);

  useEffect(() => {
    requestIdRef.current += 1;
    setUserPosition(null);
    setLocating(false);
    setLocationError(null);
    setIncidentChanged(false);
  }, [resetKey]);

  const onMapReady = useCallback((map: LeafletMap | null) => {
    mapRef.current = map;
  }, []);

  const selectIncident = useCallback((next: ReportLocation) => {
    requestIdRef.current += 1;
    setLocating(false);
    if (location && (location.coordinates[0] !== next.coordinates[0] || location.coordinates[1] !== next.coordinates[1])) setIncidentChanged(true);
    setLocationError(null);
    onChange(next);
  }, [location, onChange]);

  const setError = (error: LocationErrorKey) => {
    if (!mountedRef.current) return;
    setUserPosition(null);
    setLocating(false);
    setLocationError(error);
  };

  const requestPosition = (intent: GeolocationIntent) => {
    const requestId = ++requestIdRef.current;
    const isCurrent = () => mountedRef.current && requestIdRef.current === requestId;
    const unsupported = typeof navigator === "undefined" || !navigator.geolocation || (typeof window !== "undefined" && window.isSecureContext === false);
    if (unsupported) {
      setError("location.unsupported");
      return;
    }

    setLocating(true);
    setLocationError(null);
    setUserPosition(null);
    try {
      navigator.geolocation.getCurrentPosition((position) => {
        if (!isCurrent()) return;
        const next = validUserPosition(position);
        if (!next) {
          setError("location.unavailable");
          return;
        }
        setUserPosition(next);
        setLocating(false);
        setLocationError(null);
        if (intent === "select") {
          const incident = reportLocation(next.longitude, next.latitude);
          if (incident) selectIncident(incident);
        }
      }, (error) => {
        if (!isCurrent()) return;
        setError(error.code === 1 ? "location.denied" : error.code === 3 ? "location.timeout" : "location.unavailable");
      }, geolocationOptions);
    } catch (error) {
      if (!isCurrent()) return;
      const errorName = error instanceof DOMException ? error.name : "";
      setError(errorName === "NotAllowedError" || errorName === "PermissionDeniedError" ? "location.denied" : "location.unavailable");
    }
  };

  const selectMapCenter = () => {
    const center = mapRef.current?.getCenter();
    const next = center ? reportLocation(center.lng, center.lat) : null;
    if (next) selectIncident(next);
  };

  return <fieldset className="report-location-picker">
    <legend>{t("location.legend")}</legend>
    <p className="report-location-picker__description">{t("location.description")}</p>
    <div className="report-location-map" aria-label={t("location.mapLabel")}>
      <MapContainer center={location ? [location.coordinates[1], location.coordinates[0]] : initialCenter} zoom={11} minZoom={8} maxZoom={16} className="leaflet-map" scrollWheelZoom={false}>
        <TileLayer url="https://tile.openstreetmap.org/{z}/{x}/{y}.png" attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors' />
        <LocationEvents onSelect={selectIncident} />
        <MapBridge userPosition={userPosition} onReady={onMapReady} />
        {userPosition && <>
          <Circle center={[userPosition.latitude, userPosition.longitude]} radius={userPosition.accuracy} pathOptions={{ color: "var(--color-primary-strong)", fillColor: "var(--color-primary-strong)", fillOpacity: 0.12, weight: 2 }} />
          <CircleMarker center={[userPosition.latitude, userPosition.longitude]} radius={7} pathOptions={{ color: "var(--color-surface)", fillColor: "var(--color-primary-strong)", fillOpacity: 1, weight: 3 }} />
        </>}
        {location && <Marker position={[location.coordinates[1], location.coordinates[0]]} icon={incidentPinIcon} title={t("location.incidentLegend")} interactive={false} keyboard={false} />}
      </MapContainer>
      <MapLocationControl locating={locating} label={t("location.locate")} loadingLabel={t("location.locating")} onLocate={() => requestPosition("locate")} />
    </div>
    <div className="report-location-picker__actions">
      <button type="button" className="report-location-picker__action" disabled={locating} onClick={() => requestPosition("select")}>
        <span aria-hidden="true">●</span>{locating ? t("location.locating") : t("location.useCurrent")}
      </button>
      <button type="button" className="report-location-picker__action report-location-picker__action--secondary" onClick={selectMapCenter}>{t("location.selectCenter")}</button>
    </div>
    <p className="report-location-picker__status" role="status" aria-live="polite">
      {location ? t("location.selected") : t("location.notSelected")}{incidentChanged && <> {t("location.changed")}</>}
    </p>
    {locationError && <p className="report-location-picker__error" role="alert">{t(locationError)}</p>}
    {userPosition && <div className="report-location-picker__accuracy">
      <p>{t("location.accuracy", { meters: Math.round(userPosition.accuracy) })}</p>
      {userPosition.accuracy > 100 && <p className="report-location-picker__accuracy-warning">{t("location.lowAccuracy")}</p>}
    </div>}
    <div className="report-location-picker__legend" aria-label={t("location.legend")}>
      <span><i className="report-location-picker__shape report-location-picker__shape--user" aria-hidden="true" />{t("location.userLegend")}</span>
      <span><svg className="report-location-picker__incident-legend" viewBox="0 0 32 40" aria-hidden="true"><path d={incidentPinPath} /><circle className="report-incident-pin__center" cx="16" cy="15" r="5" /></svg>{t("location.incidentLegend")}</span>
    </div>
  </fieldset>;
}
