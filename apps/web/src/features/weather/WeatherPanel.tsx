import { useEffect, useState } from "react";
import { formatDate, formatNumber, useTranslation } from "../../i18n";
import { conditionForCode, type WeatherConditionKey } from "./conditions";
import { fetchWeather, isSafeProviderUrl, type WeatherFreshness, type WeatherResponse } from "./api";
import en from "./locales/en";
import th from "./locales/th";
import "./WeatherPanel.css";

export const WEATHER_REFRESH_INTERVAL_MS = 10 * 60 * 1000;
export const WEATHER_AGE_REEVALUATION_INTERVAL_MS = 60 * 1000;
const FRESH_MAX_AGE_MS = 60 * 60 * 1000;
const STALE_MAX_AGE_MS = 6 * 60 * 60 * 1000;
const reasonCodeKeys = {
  UPSTREAM_UNAVAILABLE: "reason.UPSTREAM_UNAVAILABLE",
  NOT_CONFIGURED: "reason.NOT_CONFIGURED",
  DATA_STALE: "reason.DATA_STALE",
  REQUEST_CANCELED: "reason.REQUEST_CANCELED"
} as const;

type WeatherState = {
  data: WeatherResponse | null;
  loading: boolean;
  error: boolean;
  refreshFailed: boolean;
};

export function displayFreshness(data: WeatherResponse, now: number, refreshFailed: boolean): WeatherFreshness {
  if (!data.current || data.freshness === "UNKNOWN") return "UNKNOWN";
  const observedAt = Date.parse(data.current.observedAt);
  if (Number.isNaN(observedAt)) return "UNKNOWN";
  const age = Math.max(0, now - observedAt);
  if (!refreshFailed && data.freshness === "FRESH" && age <= FRESH_MAX_AGE_MS) return "FRESH";
  if (age <= STALE_MAX_AGE_MS) return "STALE";
  return "UNKNOWN";
}

function formatWeatherTime(value: string | null, locale: "th" | "en", unknown: string): string {
  return value ? formatDate(value, locale) : unknown;
}

function WeatherIcon({ condition }: { condition: WeatherConditionKey }) {
  const path = condition === "clear" || condition === "mainlyClear" ? <><circle cx="12" cy="12" r="4" /><path d="M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4" /></> : condition === "fog" || condition === "rimeFog" ? <><path d="M4 9h16M3 13h18M5 17h14" /></> : condition === "partlyCloudy" || condition === "overcast" ? <path d="M5 15a5 5 0 1 1 2-9 6 6 0 0 1 11 3 4 4 0 0 1-1 8H5" /> : condition.includes("thunderstorm") ? <><path d="M6 15a5 5 0 1 1 2-9 6 6 0 0 1 11 3 4 4 0 0 1-1 8H6" /><path d="m12 13-2 4h3l-2 4 5-6h-3l2-2" /></> : <><path d="M5 15a5 5 0 1 1 2-9 6 6 0 0 1 11 3 4 4 0 0 1-1 8H5" /><path d="M8 19l-1 2M13 19l-1 2M18 19l-1 2" /></>;
  return <svg className="weather-panel__icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">{path}</svg>;
}

export function WeatherPanel() {
  const { locale, t: translate } = useTranslation({ th, en });
  const [retry, setRetry] = useState(0);
  const [now, setNow] = useState(() => Date.now());
  const [state, setState] = useState<WeatherState>({ data: null, loading: true, error: false, refreshFailed: false });

  useEffect(() => {
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
  }, [retry]);

  const data = state.data;
  const freshness = data ? displayFreshness(data, now, state.refreshFailed) : "UNKNOWN";
  const condition = data?.current ? conditionForCode(data.current.weatherCode) : null;
  const usableCurrent = Boolean(data?.current && condition && freshness !== "UNKNOWN");
  const providerUrl = data && isSafeProviderUrl(data.source.url) ? data.source.url : null;
  const reasonKey = data?.reason ? reasonCodeKeys[data.reason as keyof typeof reasonCodeKeys] : undefined;
  const reasonText = data?.reason ? reasonKey ? translate(reasonKey) : /^[A-Z][A-Z0-9_]+$/.test(data.reason) ? translate("reason.unknown") : data.reason : null;

  if (state.loading && !data) return <section className="weather-panel" aria-labelledby="weather-title"><div className="weather-panel__heading"><div><h2 id="weather-title">{translate("title")}</h2><p>{translate("location")}</p></div></div><div className="weather-panel__state" role="status" aria-label={translate("loading.aria")}><h3>{translate("loading.title")}</h3><p>{translate("loading.body")}</p></div></section>;

  if (state.error && !data) return <section className="weather-panel" aria-labelledby="weather-title"><div className="weather-panel__heading"><div><h2 id="weather-title">{translate("title")}</h2><p>{translate("location")}</p></div></div><div className="weather-panel__state weather-panel__state--error" role="status" aria-label={translate("error.aria")}><h3>{translate("error.title")}</h3><p>{translate("error.body")}</p><button type="button" onClick={() => setRetry((value) => value + 1)}>{translate("retry")}</button></div></section>;

  if (!data || !data.current || !usableCurrent || !condition) return <section className="weather-panel" aria-labelledby="weather-title"><div className="weather-panel__header"><div className="weather-panel__heading"><div><h2 id="weather-title">{translate("title")}</h2><p>{translate("location")}</p></div></div><span className="weather-panel__freshness weather-panel__freshness--unknown">{translate("unknown")}</span></div><div className="weather-panel__state"><h3>{translate("unavailable.title")}</h3><p>{reasonText ?? translate("unavailable.body")}</p><button type="button" onClick={() => setRetry((value) => value + 1)}>{translate("retry")}</button></div><p className="weather-panel__note">{translate("floodCaveat")}</p></section>;

  const current = data.current;
  const conditionLabel = translate(`condition.${condition.key}`);
  const intervalMinutes = formatNumber(current.intervalSeconds / 60, locale, { maximumFractionDigits: 1 });
  const observedAt = formatWeatherTime(current.observedAt, locale, translate("unknown"));
  const retrievedAt = formatWeatherTime(data.retrievedAt, locale, translate("unknown"));

  return <section className="weather-panel" aria-labelledby="weather-title">
    <div className="weather-panel__header">
      <div className="weather-panel__heading"><div><h2 id="weather-title">{translate("title")}</h2><p>{translate("location")}</p></div></div>
      <span className={`weather-panel__freshness weather-panel__freshness--${freshness.toLowerCase()}`}>{translate(freshness === "FRESH" ? "fresh" : freshness === "STALE" ? "stale" : "unknown")}</span>
    </div>
    <div className="weather-panel__summary">
      <div className="weather-panel__condition"><WeatherIcon condition={condition.key} /><strong>{conditionLabel}</strong><span className="weather-panel__temperature">{formatNumber(current.temperatureC, locale, { maximumFractionDigits: 1 })}°C</span><span>{translate("apparentTemperature", { value: formatNumber(current.apparentTemperatureC, locale, { maximumFractionDigits: 1 }) })}</span></div>
      <dl className="weather-panel__reading-list">
        <div className="weather-panel__reading"><dt>{translate("humidityLabel")}</dt><dd>{formatNumber(current.humidityPercent, locale, { maximumFractionDigits: 0 })}%</dd></div>
        <div className="weather-panel__reading"><dt>{translate("precipitationLabel")}</dt><dd>{translate("precipitation", { value: formatNumber(current.precipitationMm, locale, { maximumFractionDigits: 1 }), minutes: intervalMinutes })}</dd></div>
        <div className="weather-panel__reading"><dt>{translate("windLabel")}</dt><dd>{translate("wind", { speed: formatNumber(current.windSpeedKmh, locale, { maximumFractionDigits: 1 }), direction: formatNumber(current.windDirectionDegrees, locale, { maximumFractionDigits: 0 }) })}</dd></div>
        <div className="weather-panel__reading"><dt>{current.isDay ? translate("day") : translate("night")}</dt><dd>{conditionLabel}</dd></div>
      </dl>
    </div>
    {freshness === "STALE" && <p className="weather-panel__note weather-panel__note--stale">{translate("staleNotice")}</p>}
    {state.error && <div className="weather-panel__state weather-panel__state--error" role="status"><h3>{translate("error.title")}</h3><p>{translate("error.body")}</p><button type="button" onClick={() => setRetry((value) => value + 1)}>{translate("retry")}</button></div>}
    <div className="weather-panel__meta"><span>{translate("modelTime", { value: observedAt })}</span><span>{translate("retrievedAt", { value: retrievedAt })}</span></div>
    <div className="weather-panel__provider"><span>{translate("model")}</span>{providerUrl && <span>{translate("providerLabel")}: <a href={providerUrl} target="_blank" rel="noreferrer">{data.source.name}</a></span>}</div>
    {reasonText && <p className="weather-panel__note">{translate("reason", { value: reasonText })}</p>}
    <p className="weather-panel__note">{translate("floodCaveat")}</p>
  </section>;
}
