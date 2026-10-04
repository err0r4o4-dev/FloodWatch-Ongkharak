import { useCallback, useEffect, useId, useRef, useState } from "react";
import type { Map as LeafletMap } from "leaflet";
import { divIcon } from "leaflet";
import { MapContainer, Marker, TileLayer, useMap, useMapEvents } from "react-leaflet";
import "leaflet/dist/leaflet.css";
import { useTranslation } from "../../i18n";
import th from "../../i18n/locales/th/reports";
import en from "../../i18n/locales/en/reports";
import type { ReportLocation } from "./types";
import { getMapSession, mapSettings } from "../../shared/map/mapSession";
import { useUserLocation } from "../../shared/map/useUserLocation";
import { centerOnUser, MapNavigation, SharedMapViewport, UserLocationMarker } from "../../shared/map/MapNavigation";
import controlTh from "../../i18n/locales/th/mapControls";
import controlEn from "../../i18n/locales/en/mapControls";
import "./ReportLocationPicker.css";

const incidentPinPath = "M16 1C8.3 1 2 7.3 2 15c0 10 14 24 14 24s14-14 14-24C30 7.3 23.7 1 16 1Z";
const incidentPinIcon = divIcon({
  className: "report-incident-pin",
  html: `<svg viewBox="0 0 32 40" aria-hidden="true"><path d="${incidentPinPath}"/><circle class="report-incident-pin__center" cx="16" cy="15" r="5"/></svg>`,
  iconSize: [32, 40],
  iconAnchor: [16, 39]
});

function reportLocation(longitude: number, latitude: number): ReportLocation | null {
  if (!Number.isFinite(longitude) || !Number.isFinite(latitude) || Math.abs(longitude) > 180 || Math.abs(latitude) > 90) return null;
  return { type: "Point", coordinates: [longitude, latitude] };
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

function MapBridge({ onReady }: { onReady: (map: LeafletMap | null) => void }) {
  const map = useMap();

  useEffect(() => {
    onReady(map);
    return () => onReady(null);
  }, [map, onReady]);

  return null;
}

export function ReportLocationPicker({ location, onChange, resetKey }: {
  location: ReportLocation | null;
  onChange: (location: ReportLocation | null) => void;
  resetKey: string | number;
}) {
  const { t } = useTranslation({ th, en });
  const { t: controlText } = useTranslation({ th: controlTh, en: controlEn });
  const mapRef = useRef<LeafletMap | null>(null);
  const workspace = useRef<HTMLDivElement>(null);
  const expandButton = useRef<HTMLButtonElement>(null);
  const titleId = useId();
  const userLocation = useUserLocation();
  const { position: userPosition, locating, error: locationError, cancel, locate } = userLocation;
  const [incidentChanged, setIncidentChanged] = useState(false);
  const [expanded, setExpanded] = useState(false);
  const [viewport] = useState(() => {
    const shared = getMapSession().viewport;
    return {
      center: location ? [location.coordinates[1], location.coordinates[0]] as [number, number] : shared?.center ?? mapSettings.initialCenter,
      zoom: location ? Math.max(shared?.zoom ?? mapSettings.initialZoom, mapSettings.locateZoom) : shared?.zoom ?? mapSettings.initialZoom
    };
  });

  useEffect(() => {
    cancel();
    setIncidentChanged(false);
  }, [resetKey, cancel]);

  useEffect(() => {
    if (!expanded) return;
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    expandButton.current?.focus();
    return () => { document.body.style.overflow = previousOverflow; expandButton.current?.focus(); };
  }, [expanded]);

  const onMapReady = useCallback((map: LeafletMap | null) => {
    mapRef.current = map;
  }, []);

  const selectIncident = useCallback((next: ReportLocation) => {
    cancel();
    if (location && (location.coordinates[0] !== next.coordinates[0] || location.coordinates[1] !== next.coordinates[1])) setIncidentChanged(true);
    onChange(next);
  }, [location, onChange, cancel]);

  const selectMapCenter = () => {
    const center = mapRef.current?.getCenter();
    const next = center ? reportLocation(center.lng, center.lat) : null;
    if (next) selectIncident(next);
  };

  return <div ref={workspace} className={`report-location-workspace${expanded ? " is-expanded" : ""}`} role={expanded ? "dialog" : undefined} aria-modal={expanded || undefined} aria-labelledby={expanded ? titleId : undefined} onKeyDown={(event) => {
    if (!expanded) return;
    if (event.key === "Escape") { event.preventDefault(); event.stopPropagation(); setExpanded(false); }
    if (event.key === "Tab") {
      const controls = Array.from(workspace.current?.querySelectorAll<HTMLElement>('button:not([disabled]), a[href], [tabindex="0"]') ?? []).filter((element) => element.getClientRects().length > 0);
      const first = controls[0];
      const last = controls[controls.length - 1];
      if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last?.focus(); }
      else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first?.focus(); }
    }
  }}><fieldset className="report-location-picker">
    <legend id={titleId}>{t("location.legend")}</legend>
    <p className="report-location-picker__description">{t("location.description")}</p>
    <div className="report-location-map" aria-label={t("location.mapLabel")}>
      <MapContainer center={viewport.center} zoom={viewport.zoom} minZoom={mapSettings.minZoom} maxZoom={mapSettings.maxZoom} zoomControl={false} className="leaflet-map" scrollWheelZoom={true}>
        <TileLayer url="https://tile.openstreetmap.org/{z}/{x}/{y}.png" attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors' maxZoom={mapSettings.maxZoom} />
        <SharedMapViewport />
        <MapBridge onReady={onMapReady} />
        <UserLocationMarker position={userPosition} />
        {location && <Marker position={[location.coordinates[1], location.coordinates[0]]} icon={incidentPinIcon} title={t("location.incidentLegend")} interactive={false} keyboard={false} />}
        <MapNavigation location={userLocation} locationLabel={t("location.locate")}>
          <button ref={expandButton} type="button" aria-label={controlText(expanded ? "collapse" : "expand")} title={controlText(expanded ? "collapse" : "expand")} aria-pressed={expanded} onClick={() => setExpanded((previous) => !previous)}><span aria-hidden="true">⛶</span></button>
        </MapNavigation>
        <LocationEvents onSelect={selectIncident} />
      </MapContainer>
    </div>
    <div className="report-location-picker__actions">
      <button type="button" className="report-location-picker__action" disabled={locating} onClick={() => locate({ fresh: true, onSuccess: (next) => {
        const incident = reportLocation(next.longitude, next.latitude);
        if (mapRef.current) centerOnUser(mapRef.current, next);
        if (incident) selectIncident(incident);
      } })}>
        <span aria-hidden="true">●</span>{locating ? t("location.locating") : t("location.useCurrent")}
      </button>
      <button type="button" className="report-location-picker__action report-location-picker__action--secondary" onClick={selectMapCenter}>{t("location.selectCenter")}</button>
    </div>
    <p className="report-location-picker__status" role="status" aria-live="polite">
      {location ? t("location.selected") : t("location.notSelected")}{incidentChanged && <> {t("location.changed")}</>}
    </p>
    {locationError && <p className="report-location-picker__error" role="alert">{t(`location.${locationError}`)}</p>}
    {userPosition && <div className="report-location-picker__accuracy">
      <p>{t("location.accuracy", { meters: Math.round(userPosition.accuracy) })}</p>
      {userPosition.accuracy > 100 && <p className="report-location-picker__accuracy-warning">{t("location.lowAccuracy")}</p>}
    </div>}
    <div className="report-location-picker__legend" aria-label={t("location.legend")}>
      <span><i className="report-location-picker__shape report-location-picker__shape--user" aria-hidden="true" />{t("location.userLegend")}</span>
      <span><svg className="report-location-picker__incident-legend" viewBox="0 0 32 40" aria-hidden="true"><path d={incidentPinPath} /><circle className="report-incident-pin__center" cx="16" cy="15" r="5" /></svg>{t("location.incidentLegend")}</span>
    </div>
  </fieldset></div>;
}
