import { useEffect, useState } from "react";
import { useRealtimeRevision } from "../realtime/RealtimeProvider";
import { fetchOfficialAnnouncements, type OfficialAnnouncementsResponse } from "./api";

export const OFFICIAL_ANNOUNCEMENTS_REFRESH_INTERVAL_MS = 60_000;
export const OFFICIAL_ANNOUNCEMENTS_REQUEST_TIMEOUT_MS = 15_000;

export type OfficialAnnouncementsState = {
  data: OfficialAnnouncementsResponse | null;
  loading: boolean;
  error: boolean;
};

function isVisible(): boolean {
  return typeof document === "undefined" || document.visibilityState === "visible";
}

export function useOfficialAnnouncements() {
  const realtimeRevision = useRealtimeRevision(["external_data.updated"]);
  const [retry, setRetry] = useState(0);
  const [visible, setVisible] = useState(isVisible);
  const [state, setState] = useState<OfficialAnnouncementsState>({ data: null, loading: true, error: false });

  useEffect(() => {
    const onVisibilityChange = () => setVisible(isVisible());
    document.addEventListener("visibilitychange", onVisibilityChange);
    return () => document.removeEventListener("visibilitychange", onVisibilityChange);
  }, []);

  useEffect(() => {
    if (!visible) return;
    let disposed = false;
    let controller: AbortController | null = null;
    let requestTimeout: number | null = null;

    const fail = () => {
      if (disposed) return;
      setState({ data: null, loading: false, error: true });
    };

    const load = () => {
      controller?.abort();
      if (requestTimeout !== null) window.clearTimeout(requestTimeout);
      const requestController = new AbortController();
      controller = requestController;
      setState((previous) => ({ ...previous, loading: previous.data === null, error: false }));
      const timeout = window.setTimeout(() => {
        if (disposed || requestController.signal.aborted) return;
        fail();
        requestController.abort();
      }, OFFICIAL_ANNOUNCEMENTS_REQUEST_TIMEOUT_MS);
      requestTimeout = timeout;
      void fetchOfficialAnnouncements(requestController.signal).then((data) => {
        if (disposed || requestController.signal.aborted) return;
        setState({ data, loading: false, error: false });
      }).catch(() => {
        if (disposed || requestController.signal.aborted) return;
        fail();
      }).finally(() => {
        window.clearTimeout(timeout);
      });
    };

    load();
    const timer = window.setInterval(load, OFFICIAL_ANNOUNCEMENTS_REFRESH_INTERVAL_MS);
    return () => {
      disposed = true;
      controller?.abort();
      if (requestTimeout !== null) window.clearTimeout(requestTimeout);
      window.clearInterval(timer);
    };
  }, [realtimeRevision, retry, visible]);

  return { state, setRetry };
}
