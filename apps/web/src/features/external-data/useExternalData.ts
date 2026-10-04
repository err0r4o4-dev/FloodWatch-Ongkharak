import { useEffect, useState } from "react";
import { useRealtimeRevision } from "../realtime/RealtimeProvider";
import { fetchExternalData, type ExternalDataResponse } from "./api";

export const EXTERNAL_DATA_REFRESH_INTERVAL_MS = 60_000;
export const EXTERNAL_DATA_REQUEST_TIMEOUT_MS = 15_000;

export type ExternalDataState = {
  data: ExternalDataResponse | null;
  loading: boolean;
  error: boolean;
};

function isVisible(): boolean {
  return typeof document === "undefined" || document.visibilityState === "visible";
}

function unavailableState(previous: ExternalDataState): ExternalDataState {
  return {
    data: previous.data ? {
      ...previous.data,
      sources: previous.data.sources.map((source) => !source.enabled ? source : ({
        ...source,
        status: "UNAVAILABLE",
        freshness: "UNKNOWN",
        records: source.records.map((record) => ({ ...record, value: null, freshness: "UNKNOWN", reason: "SNAPSHOT_UNAVAILABLE" }))
      }))
    } : null,
    loading: false,
    error: true
  };
}

export function useExternalData() {
  const realtimeRevision = useRealtimeRevision(["external_data.updated"]);
  const [retry, setRetry] = useState(0);
  const [visible, setVisible] = useState(isVisible);
  const [state, setState] = useState<ExternalDataState>({ data: null, loading: true, error: false });

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

    const load = () => {
      controller?.abort();
      if (requestTimeout !== null) window.clearTimeout(requestTimeout);
      const requestController = new AbortController();
      controller = requestController;
      setState((previous) => ({ ...previous, loading: previous.data === null, error: false }));
      const timeout = window.setTimeout(() => {
        if (disposed || requestController.signal.aborted) return;
        setState(unavailableState);
        requestController.abort();
      }, EXTERNAL_DATA_REQUEST_TIMEOUT_MS);
      requestTimeout = timeout;
      void fetchExternalData(requestController.signal).then((data) => {
        if (disposed || requestController.signal.aborted) return;
        setState({ data, loading: false, error: false });
      }).catch(() => {
        if (disposed || requestController.signal.aborted) return;
        setState(unavailableState);
      }).finally(() => {
        window.clearTimeout(timeout);
      });
    };

    load();
    const timer = window.setInterval(load, EXTERNAL_DATA_REFRESH_INTERVAL_MS);
    return () => {
      disposed = true;
      controller?.abort();
      if (requestTimeout !== null) window.clearTimeout(requestTimeout);
      window.clearInterval(timer);
    };
  }, [realtimeRevision, retry, visible]);

  return { state, setRetry };
}
