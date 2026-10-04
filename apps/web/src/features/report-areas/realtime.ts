import { useEffect, useState } from "react";
import { apiConfig } from "../../shared/config/api";

function validAreaEvent(value: unknown): boolean {
  if (!value || typeof value !== "object" || Array.isArray(value)) return false;
  const event = value as Record<string, unknown>;
  return event.eventType === "report-area.updated" && typeof event.eventId === "string" && event.eventId.length > 0 &&
    typeof event.resourceId === "string" && event.resourceId.length > 0 && event.payloadVersion === 1 && event.sourceType === "COMMUNITY";
}

export function useReportAreaUpdatedRevision(areaId?: string | null): number {
  const [revision, setRevision] = useState(0);
  useEffect(() => {
    if (typeof EventSource === "undefined") return;
    const source = new EventSource(`${apiConfig.baseUrl}/events/stream`);
    const onUpdate = (message: Event) => {
      const eventData = message && typeof message === "object" ? (message as MessageEvent).data : undefined;
      if (typeof eventData !== "string") return;
      try {
        const value: unknown = JSON.parse(eventData);
        if (!validAreaEvent(value)) return;
        if (areaId && (value as { resourceId: string }).resourceId !== areaId) return;
        setRevision((current) => current + 1);
      } catch { return; }
    };
    source.addEventListener("report-area.updated", onUpdate);
    return () => { source.removeEventListener("report-area.updated", onUpdate); source.close(); };
  }, [areaId]);
  return revision;
}
