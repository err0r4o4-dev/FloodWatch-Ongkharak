import { useRealtimeRevision } from "../realtime/RealtimeProvider";

export function useReportAreaUpdatedRevision(_areaId?: string | null): number {
  return useRealtimeRevision(["report-area.updated"]);
}
