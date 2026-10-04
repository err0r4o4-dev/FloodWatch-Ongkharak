import { createContext, useContext, useEffect, useState } from "react";
import { apiConfig } from "../../shared/config/api";

export const publicEventTypes = ["water_level.updated", "flood_event.created", "flood_event.updated", "road.status.changed", "report.verified", "report.updated", "alert.created", "external_data.updated"] as const;
export type PublicEventType = typeof publicEventTypes[number];
export type RealtimeStatus = "CONNECTED" | "RECONNECTING" | "DISCONNECTED";
const stableConnectionDelay = 15_000;

type RealtimeState = { status: RealtimeStatus; revisions: Record<PublicEventType, number> };
const initialRevisions = Object.fromEntries(publicEventTypes.map((type) => [type, 0])) as Record<PublicEventType, number>;
const RealtimeContext = createContext<RealtimeState>({ status: "DISCONNECTED", revisions: initialRevisions });

function isPublicEvent(value: unknown, expectedType: PublicEventType): boolean {
  if (!value || typeof value !== "object") return false;
  const event = value as Record<string, unknown>;
  const validSource = expectedType === "external_data.updated"
    ? event.resourceType === "EXTERNAL_DATA" && ["OFFICIAL", "MODEL"].includes(String(event.sourceType))
    : ["OFFICIAL", "COMMUNITY", "AI"].includes(String(event.sourceType));
  return event.eventType === expectedType && typeof event.eventId === "string" &&
    typeof event.resourceId === "string" && event.payloadVersion === 1 &&
    validSource;
}

export function RealtimeProvider({ children }: { children: React.ReactNode }) {
  const [state, setState] = useState<RealtimeState>({ status: "DISCONNECTED", revisions: initialRevisions });

  useEffect(() => {
    if (typeof EventSource === "undefined") return;
    let active = true;
    let connection: EventSource | null = null;
    let reconnectTimer: number | null = null;
    let healthyTimer: number | null = null;
    let attempts = 0;
    let cursor = "";
    const seen = new Set<string>();
    const refresh = (type?: PublicEventType) => setState((current) => ({
      ...current,
      revisions: Object.fromEntries(publicEventTypes.map((key) => [key, current.revisions[key] + (!type || key === type ? 1 : 0)])) as RealtimeState["revisions"]
    }));
    const markHealthy = () => {
      attempts = 0;
      if (healthyTimer !== null) {
        window.clearTimeout(healthyTimer);
        healthyTimer = null;
      }
    };

    const connect = () => {
      if (!active) return;
      reconnectTimer = null;
      const url = `${apiConfig.baseUrl}/events/stream${cursor ? `?cursor=${encodeURIComponent(cursor)}` : ""}`;
      const source = new EventSource(url);
      connection = source;
      source.onopen = () => {
        if (!active || connection !== source) return;
        refresh();
        if (healthyTimer !== null) window.clearTimeout(healthyTimer);
        healthyTimer = window.setTimeout(() => {
          if (active && connection === source) markHealthy();
        }, stableConnectionDelay);
        setState((current) => ({ ...current, status: "CONNECTED" }));
      };
      for (const type of publicEventTypes) {
        source.addEventListener(type, (message) => {
          if (!active || connection !== source || !(message instanceof MessageEvent)) return;
          try {
            const envelope: unknown = JSON.parse(message.data);
            if (!isPublicEvent(envelope, type)) return;
            const eventId = (envelope as { eventId: string }).eventId;
            if (message.lastEventId) cursor = message.lastEventId;
            markHealthy();
            if (seen.has(eventId)) return;
            seen.add(eventId);
            if (seen.size > 256) seen.delete(seen.values().next().value!);
            refresh(type);
          } catch {
            return;
          }
        });
      }
      source.addEventListener("replay.reset", () => {
        if (!active || connection !== source) return;
        cursor = "";
        seen.clear();
        refresh();
      });
      source.onerror = () => {
        if (!active || connection !== source) return;
        if (healthyTimer !== null) {
          window.clearTimeout(healthyTimer);
          healthyTimer = null;
        }
        source.close();
        connection = null;
        attempts++;
        setState((current) => ({ ...current, status: attempts > 5 ? "DISCONNECTED" : "RECONNECTING" }));
        const delay = Math.min(30_000, 1_000 * 2 ** Math.min(attempts - 1, 5));
        reconnectTimer = window.setTimeout(connect, delay);
      };
    };

    setState((current) => ({ ...current, status: "RECONNECTING" }));
    connect();
    return () => {
      active = false;
      connection?.close();
      if (reconnectTimer !== null) window.clearTimeout(reconnectTimer);
      if (healthyTimer !== null) window.clearTimeout(healthyTimer);
    };
  }, []);

  return <RealtimeContext.Provider value={state}>{children}</RealtimeContext.Provider>;
}

export function useRealtimeStatus(): RealtimeStatus {
  return useContext(RealtimeContext).status;
}

export function useRealtimeRevision(types: readonly PublicEventType[]): number {
  const { revisions } = useContext(RealtimeContext);
  return types.reduce((total, type) => total + revisions[type], 0);
}
