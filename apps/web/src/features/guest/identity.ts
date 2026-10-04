import { useCallback, useSyncExternalStore } from "react";

export const DEFAULT_GUEST_DISPLAY_NAME = "Guest";
export const GUEST_DISPLAY_NAME_STORAGE_KEY = "floodwatch.guest.displayName";
const MAX_DISPLAY_NAME_CODE_POINTS = 80;

type Listener = () => void;

const listeners = new Set<Listener>();
let currentDisplayName = normalizeDisplayName(readStoredDisplayName() ?? DEFAULT_GUEST_DISPLAY_NAME);
let hasUnsavedDisplayName = false;

function getStorage(): Storage | null {
  try {
    return typeof window === "undefined" ? null : window.localStorage;
  } catch {
    return null;
  }
}

function readStoredDisplayName(): string | null | undefined {
  const storage = getStorage();
  if (!storage) return undefined;
  try {
    return storage.getItem(GUEST_DISPLAY_NAME_STORAGE_KEY);
  } catch {
    return undefined;
  }
}

function normalizeDisplayName(value: string): string {
  const trimmed = value.trim();
  return Array.from(trimmed).slice(0, MAX_DISPLAY_NAME_CODE_POINTS).join("") || DEFAULT_GUEST_DISPLAY_NAME;
}

function notify() {
  listeners.forEach((listener) => listener());
}

function syncFromStorage() {
  if (hasUnsavedDisplayName) return;
  const stored = readStoredDisplayName();
  if (stored === undefined) return;
  const next = normalizeDisplayName(stored ?? DEFAULT_GUEST_DISPLAY_NAME);
  if (next === currentDisplayName) return;
  currentDisplayName = next;
  notify();
}

export function setGuestDisplayName(value: string) {
  const next = normalizeDisplayName(value);
  currentDisplayName = next;
  hasUnsavedDisplayName = true;
  try {
    const storage = getStorage();
    if (storage) {
      storage.setItem(GUEST_DISPLAY_NAME_STORAGE_KEY, next);
      hasUnsavedDisplayName = false;
    }
  } catch {}
  notify();
}

function subscribe(listener: Listener) {
  listeners.add(listener);
  syncFromStorage();
  const onStorage = (event: StorageEvent) => {
    if (event.key !== null && event.key !== GUEST_DISPLAY_NAME_STORAGE_KEY) return;
    hasUnsavedDisplayName = false;
    const next = normalizeDisplayName(event.newValue ?? DEFAULT_GUEST_DISPLAY_NAME);
    if (next === currentDisplayName) return;
    currentDisplayName = next;
    notify();
  };
  window.addEventListener("storage", onStorage);
  return () => {
    listeners.delete(listener);
    window.removeEventListener("storage", onStorage);
  };
}

function getSnapshot() {
  return currentDisplayName;
}

export function useGuestIdentity() {
  const displayName = useSyncExternalStore(subscribe, getSnapshot, () => DEFAULT_GUEST_DISPLAY_NAME);
  const setDisplayName = useCallback((value: string) => setGuestDisplayName(value), []);
  return { displayName, setDisplayName };
}
