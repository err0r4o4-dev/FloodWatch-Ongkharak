import { useSyncExternalStore } from "react";

export const mapSettings = {
  initialCenter: [14.1, 101] as [number, number], initialZoom: 11,
  minZoom: 8, maxZoom: 18, locateZoom: 16, scaleMaxWidth: 150, positionMaxAge: 60_000
};

export type MapViewport = { center: [number, number]; zoom: number };
export type UserPosition = { latitude: number; longitude: number; accuracy: number; timestamp: number };
type MapSession = { viewport: MapViewport | null; position: UserPosition | null };

let session: MapSession = { viewport: null, position: null };
const listeners = new Set<() => void>();

export function getMapSession() { return session; }

function publish(next: MapSession) {
  session = next;
  listeners.forEach((listener) => listener());
}

export function setMapViewport(viewport: MapViewport) {
  const [latitude, longitude] = viewport.center;
  if (!Number.isFinite(latitude) || !Number.isFinite(longitude) || Math.abs(latitude) > 90 || Math.abs(longitude) > 180 || !Number.isFinite(viewport.zoom)) return;
  const next = { center: [latitude, longitude] as [number, number], zoom: Math.min(mapSettings.maxZoom, Math.max(mapSettings.minZoom, viewport.zoom)) };
  const previous = session.viewport;
  if (previous?.zoom === next.zoom && previous.center[0] === latitude && previous.center[1] === longitude) return;
  publish({ ...session, viewport: next });
}

export function positionIsFresh(position: UserPosition | null): position is UserPosition {
  if (!position) return false;
  const age = Date.now() - position.timestamp;
  return Number.isFinite(position.latitude) && Math.abs(position.latitude) <= 90 &&
    Number.isFinite(position.longitude) && Math.abs(position.longitude) <= 180 &&
    Number.isFinite(position.accuracy) && position.accuracy >= 0 && Number.isFinite(age) && age >= -1000 && age < mapSettings.positionMaxAge;
}

export function setUserPosition(position: UserPosition | null) {
  publish({ ...session, position: positionIsFresh(position) ? { ...position } : null });
}

export function resetMapSession() { publish({ viewport: null, position: null }); }

function subscribe(listener: () => void) {
  listeners.add(listener);
  return () => { listeners.delete(listener); };
}

export function useMapSession() { return useSyncExternalStore(subscribe, getMapSession, getMapSession); }
