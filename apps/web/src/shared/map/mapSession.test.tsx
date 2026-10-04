import { act, renderHook } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { getMapSession, resetMapSession, setMapViewport, setUserPosition } from "./mapSession";
import { useUserLocation } from "./useUserLocation";

const userPosition = () => ({ latitude: 14.1, longitude: 101, accuracy: 24, timestamp: Date.now() });

function installGeolocation() {
  const requests: { success: PositionCallback; failure: PositionErrorCallback }[] = [];
  const getCurrentPosition = vi.fn((success: PositionCallback, failure: PositionErrorCallback) => requests.push({ success, failure }));
  vi.stubGlobal("navigator", { geolocation: { getCurrentPosition } });
  Object.defineProperty(window, "isSecureContext", { configurable: true, value: true });
  return { requests, getCurrentPosition };
}

function browserPosition(): GeolocationPosition {
  return { coords: { ...userPosition(), altitude: null, altitudeAccuracy: null, heading: null, speed: null, toJSON: () => ({}) }, timestamp: Date.now(), toJSON: () => ({}) };
}

afterEach(() => { act(() => resetMapSession()); vi.unstubAllGlobals(); vi.useRealTimers(); });

describe("shared map session", () => {
  it("shares valid camera values without persisting location", () => {
    const storage = vi.spyOn(Storage.prototype, "setItem");
    setMapViewport({ center: [14.2, 101.2], zoom: 18 });
    setUserPosition(userPosition());
    expect(getMapSession()).toMatchObject({ viewport: { center: [14.2, 101.2], zoom: 18 }, position: { longitude: 101 } });
    setMapViewport({ center: [Number.NaN, 101], zoom: 11 });
    expect(getMapSession().viewport?.center).toEqual([14.2, 101.2]);
    expect(storage).not.toHaveBeenCalled();
    storage.mockRestore();
  });

  it("does not request GPS on mount and reuses a fresh position across maps", () => {
    const { getCurrentPosition } = installGeolocation();
    const cached = userPosition();
    setUserPosition(cached);
    const first = renderHook(() => useUserLocation());
    const second = renderHook(() => useUserLocation());
    const onSuccess = vi.fn();
    act(() => second.result.current.locate({ onSuccess }));
    expect(getCurrentPosition).not.toHaveBeenCalled();
    expect(onSuccess).toHaveBeenCalledWith(cached);
    expect(first.result.current.position).toEqual(second.result.current.position);
  });

  it("requests a fresh fix explicitly and ignores canceled callbacks", () => {
    const { requests, getCurrentPosition } = installGeolocation();
    setUserPosition(userPosition());
    const hook = renderHook(() => useUserLocation());
    const onSuccess = vi.fn();
    act(() => hook.result.current.locate({ fresh: true, onSuccess }));
    expect(getCurrentPosition).toHaveBeenCalledWith(expect.any(Function), expect.any(Function), { enableHighAccuracy: true, timeout: 10000, maximumAge: 0 });
    act(() => hook.result.current.cancel());
    act(() => requests[0].success(browserPosition()));
    expect(onSuccess).not.toHaveBeenCalled();
    expect(getMapSession().position).toBeNull();
    act(() => hook.result.current.locate({ onSuccess }));
    act(() => requests[1].success(browserPosition()));
    expect(onSuccess).toHaveBeenCalledOnce();
    expect(getMapSession().position?.accuracy).toBe(24);
  });

  it("expires the cached position and reports permission errors without stale coordinates", () => {
    vi.useFakeTimers();
    const { requests } = installGeolocation();
    setUserPosition(userPosition());
    const hook = renderHook(() => useUserLocation());
    act(() => vi.advanceTimersByTime(60001));
    expect(hook.result.current.position).toBeNull();
    act(() => hook.result.current.locate());
    act(() => requests[0].failure({ code: 1 } as GeolocationPositionError));
    expect(hook.result.current.error).toBe("denied");
    expect(hook.result.current.locating).toBe(false);
  });

  it("ignores GPS responses after unmount", () => {
    const { requests } = installGeolocation();
    const hook = renderHook(() => useUserLocation());
    act(() => hook.result.current.locate());
    hook.unmount();
    act(() => requests[0].success(browserPosition()));
    expect(getMapSession().position).toBeNull();
  });
});
