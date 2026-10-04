import { createContext, useContext, useEffect, useState } from "react";
import type { ReactNode } from "react";
import { fetchWeather } from "./api";
import type { WeatherResponse } from "./api";

export const WEATHER_REFRESH_INTERVAL_MS = 10 * 60 * 1000;
export const WEATHER_AGE_REEVALUATION_INTERVAL_MS = 60 * 1000;

type WeatherState = {
  data: WeatherResponse | null;
  loading: boolean;
  error: boolean;
  refreshFailed: boolean;
};

function useWeatherLoader(enabled: boolean) {
  const [retry, setRetry] = useState(0);
  const [now, setNow] = useState(() => Date.now());
  const [state, setState] = useState<WeatherState>({ data: null, loading: true, error: false, refreshFailed: false });

  useEffect(() => {
    if (!enabled) return;
    let activeController: AbortController | null = null;
    let disposed = false;

    const load = () => {
      activeController?.abort();
      const controller = new AbortController();
      activeController = controller;
      setState((previous) => ({ ...previous, loading: previous.data === null, error: false }));
      void fetchWeather(controller.signal).then((data) => {
        if (disposed || controller.signal.aborted) return;
        setState({ data, loading: false, error: false, refreshFailed: false });
        setNow(Date.now());
      }).catch(() => {
        if (disposed || controller.signal.aborted) return;
        setState((previous) => ({ ...previous, loading: false, error: true, refreshFailed: true }));
      });
    };

    load();
    const refreshTimer = window.setInterval(load, WEATHER_REFRESH_INTERVAL_MS);
    const ageTimer = window.setInterval(() => setNow(Date.now()), WEATHER_AGE_REEVALUATION_INTERVAL_MS);
    return () => {
      disposed = true;
      activeController?.abort();
      window.clearInterval(refreshTimer);
      window.clearInterval(ageTimer);
    };
  }, [retry, enabled]);

  return { state, now, setRetry };
}

const WeatherContext = createContext<ReturnType<typeof useWeatherLoader> | null>(null);

export function WeatherProvider({ children }: { children: ReactNode }) {
  const weather = useWeatherLoader(true);
  return <WeatherContext.Provider value={weather}>{children}</WeatherContext.Provider>;
}

export function useWeather() {
  const shared = useContext(WeatherContext);
  const local = useWeatherLoader(!shared);
  return shared ?? local;
}
