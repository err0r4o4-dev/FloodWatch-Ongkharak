import { useCallback, useEffect, useRef, useState } from "react";
import { getMapSession, mapSettings, positionIsFresh, setUserPosition, useMapSession } from "./mapSession";
import type { UserPosition } from "./mapSession";

export type LocationError = "denied" | "timeout" | "unavailable" | "unsupported";
type LocateOptions = { fresh?: boolean; onSuccess?: (position: UserPosition) => void };

export function useUserLocation() {
  const { position: storedPosition } = useMapSession();
  const position = positionIsFresh(storedPosition) ? storedPosition : null;
  const [locating, setLocating] = useState(false);
  const [error, setError] = useState<LocationError | null>(null);
  const requestId = useRef(0);
  const mounted = useRef(false);

  useEffect(() => {
    mounted.current = true;
    return () => { mounted.current = false; requestId.current += 1; };
  }, []);

  useEffect(() => {
    if (!storedPosition) return;
    const timer = window.setTimeout(() => {
      if (getMapSession().position === storedPosition) setUserPosition(null);
    }, Math.max(0, mapSettings.positionMaxAge - (Date.now() - storedPosition.timestamp)));
    return () => window.clearTimeout(timer);
  }, [storedPosition]);

  const cancel = useCallback(() => {
    requestId.current += 1;
    setLocating(false);
    setError(null);
  }, []);

  const locate = useCallback(({ fresh = false, onSuccess }: LocateOptions = {}) => {
    const currentRequest = ++requestId.current;
    const isCurrent = () => mounted.current && currentRequest === requestId.current;
    setError(null);
    const cached = getMapSession().position;
    if (!fresh && positionIsFresh(cached)) {
      setLocating(false);
      onSuccess?.(cached);
      return;
    }
    setUserPosition(null);
    const fail = (next: LocationError) => {
      if (!isCurrent()) return;
      setLocating(false);
      setError(next);
    };
    if (!navigator.geolocation || window.isSecureContext === false) {
      fail("unsupported");
      return;
    }
    setLocating(true);
    try {
      navigator.geolocation.getCurrentPosition((result) => {
        if (!isCurrent()) return;
        const { latitude, longitude, accuracy } = result.coords;
        const next = { latitude, longitude, accuracy, timestamp: result.timestamp };
        if (!positionIsFresh(next)) { fail("unavailable"); return; }
        setUserPosition(next);
        setLocating(false);
        onSuccess?.(next);
      }, (failure) => {
        fail(failure.code === 1 ? "denied" : failure.code === 3 ? "timeout" : "unavailable");
      }, { enableHighAccuracy: fresh, timeout: 10_000, maximumAge: fresh ? 0 : mapSettings.positionMaxAge });
    } catch (failure) {
      const name = failure instanceof DOMException ? failure.name : "";
      fail(name === "NotAllowedError" || name === "PermissionDeniedError" ? "denied" : "unavailable");
    }
  }, []);

  return { position, locating, error, locate, cancel };
}

export type UserLocationState = ReturnType<typeof useUserLocation>;
