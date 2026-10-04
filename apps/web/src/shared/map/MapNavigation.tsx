import { useCallback, useEffect, useId, useRef, useState } from "react";
import type { ReactNode } from "react";
import { DomEvent } from "leaflet";
import type { Map as LeafletMap } from "leaflet";
import { Circle, CircleMarker, ScaleControl, useMap, useMapEvents } from "react-leaflet";
import { useTranslation } from "../../i18n";
import th from "../../i18n/locales/th/mapControls";
import en from "../../i18n/locales/en/mapControls";
import { mapSettings, setMapViewport } from "./mapSession";
import type { UserPosition } from "./mapSession";
import type { LocationError, UserLocationState } from "./useUserLocation";
import "./MapNavigation.css";

export function centerOnUser(map: LeafletMap, position: UserPosition) {
  map.setView([position.latitude, position.longitude], Math.max(map.getZoom(), mapSettings.locateZoom), { animate: false });
}

export function SharedMapViewport({ enabled = true, observeResize = true }: { enabled?: boolean; observeResize?: boolean }) {
  const map = useMap();
  const publish = useCallback(() => {
    if (!enabled) return;
    const center = map.getCenter();
    setMapViewport({ center: [center.lat, center.lng], zoom: map.getZoom() });
  }, [map, enabled]);
  useMapEvents({ moveend: publish, zoomend: publish });
  useEffect(() => {
    publish();
    if (!observeResize || typeof ResizeObserver === "undefined") return;
    const observer = new ResizeObserver(() => { map.invalidateSize({ pan: true, animate: false }); });
    observer.observe(map.getContainer());
    return () => observer.disconnect();
  }, [map, publish, observeResize]);
  return <ScaleControl position="bottomleft" imperial={false} maxWidth={mapSettings.scaleMaxWidth} />;
}

export function UserLocationMarker({ position }: { position: UserPosition | null }) {
  if (!position) return null;
  const center: [number, number] = [position.latitude, position.longitude];
  return <>
    <Circle center={center} radius={position.accuracy} pathOptions={{ color: "var(--color-primary-strong)", fillColor: "var(--color-primary-strong)", fillOpacity: 0.12, weight: 2 }} />
    <CircleMarker center={center} radius={7} pathOptions={{ color: "var(--color-surface)", fillColor: "var(--color-primary-strong)", fillOpacity: 1, weight: 2 }} />
  </>;
}

export function MapLocationError({ error }: { error: LocationError | null }) {
  const { t } = useTranslation({ th, en });
  return error ? <p className="shared-map-error" role="alert">{t(error)}</p> : null;
}

export function MapNavigation({ location, locationLabel, children }: { location: UserLocationState; locationLabel?: string; children?: ReactNode }) {
  const { t } = useTranslation({ th, en });
  const map = useMap();
  const controls = useRef<HTMLDivElement>(null);
  const descriptionId = useId();
  const [zoom, setZoom] = useState(map.getZoom());
  const [centered, setCentered] = useState(false);
  const { position, locating, locate } = location;
  const updateCenter = useCallback(() => {
    if (!position) { setCentered(false); return; }
    const point = map.latLngToContainerPoint([position.latitude, position.longitude]);
    const size = map.getSize();
    setCentered(Math.hypot(point.x - size.x / 2, point.y - size.y / 2) <= 12);
  }, [map, position]);
  useMapEvents({
    move: updateCenter, moveend: updateCenter,
    zoom: () => { setZoom(map.getZoom()); updateCenter(); },
    zoomend: () => { setZoom(map.getZoom()); updateCenter(); }
  });
  useEffect(updateCenter, [updateCenter]);
  useEffect(() => {
    if (!controls.current) return;
    DomEvent.disableClickPropagation(controls.current);
    DomEvent.disableScrollPropagation(controls.current);
  }, []);
  const status = t(locating ? "locating" : centered ? "centered" : position ? "away" : "unknown");
  return <div ref={controls} className="map-zoom-controls shared-map-controls" role="group" aria-label={t("controls")}>
    <button type="button" aria-label={t("zoomIn")} title={t("zoomIn")} disabled={zoom >= map.getMaxZoom()} onClick={() => map.zoomIn()}>+</button>
    <button type="button" aria-label={t("zoomOut")} title={t("zoomOut")} disabled={zoom <= map.getMinZoom()} onClick={() => map.zoomOut()}>−</button>
    <button type="button" className={`map-location-control${centered ? " is-centered" : ""}`} aria-label={locationLabel ?? t("location")} aria-describedby={descriptionId} title={status} aria-pressed={centered} aria-busy={locating} disabled={locating} onClick={() => locate({ onSuccess: (next) => centerOnUser(map, next) })}>
      <svg width="1.2em" height="1.2em" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true"><circle cx="12" cy="12" r="6"/><circle cx="12" cy="12" r="2.5" fill="currentColor"/><line x1="12" y1="2" x2="12" y2="5"/><line x1="12" y1="19" x2="12" y2="22"/><line x1="2" y1="12" x2="5" y2="12"/><line x1="19" y1="12" x2="22" y2="12"/></svg>
      <span id={descriptionId} className="sr-only">{status}</span>
    </button>
    {children}
  </div>;
}
