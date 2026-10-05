import { useEffect, useState } from "react";
import { useRealtimeRevision } from "../realtime/RealtimeProvider";
import { fetchFM91News, type FM91NewsResponse } from "./api";

export const FM91_NEWS_REFRESH_INTERVAL_MS = 60_000;
export const FM91_NEWS_REQUEST_TIMEOUT_MS = 15_000;

export type FM91NewsState = {
  data: FM91NewsResponse | null;
  loading: boolean;
  error: boolean;
};

function isVisible(): boolean {
  return typeof document === "undefined" || document.visibilityState === "visible";
}

export function useFM91News() {
  const realtimeRevision = useRealtimeRevision(["external_data.updated"]);
  const [retry, setRetry] = useState(0);
  const [visible, setVisible] = useState(isVisible);
  const [state, setState] = useState<FM91NewsState>({ data: null, loading: true, error: false });

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
      if (!disposed) setState({ data: null, loading: false, error: true });
    };

    const load = () => {
      controller?.abort();
      if (requestTimeout !== null) window.clearTimeout(requestTimeout);
      const requestController = new AbortController();
      controller = requestController;
      setState({ data: null, loading: true, error: false });
      const timeout = window.setTimeout(() => {
        if (disposed || requestController.signal.aborted) return;
        fail();
        requestController.abort();
      }, FM91_NEWS_REQUEST_TIMEOUT_MS);
      requestTimeout = timeout;
      void fetchFM91News(requestController.signal).then((data) => {
        if (disposed || requestController.signal.aborted) return;
        setState({ data, loading: false, error: false });
      }).catch(() => {
        if (disposed || requestController.signal.aborted) return;
        fail();
      }).finally(() => window.clearTimeout(timeout));
    };

    load();
    const timer = window.setInterval(load, FM91_NEWS_REFRESH_INTERVAL_MS);
    return () => {
      disposed = true;
      controller?.abort();
      if (requestTimeout !== null) window.clearTimeout(requestTimeout);
      window.clearInterval(timer);
    };
  }, [realtimeRevision, retry, visible]);

  return { state, setRetry };
}
