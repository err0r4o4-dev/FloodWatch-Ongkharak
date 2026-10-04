import { useEffect, useId, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { formatNumber, useTranslation } from "../../i18n";
import { conditionForCode } from "./conditions";
import { displayFreshness, WeatherIcon, WeatherPanel } from "./WeatherPanel";
import { useWeather } from "./WeatherProvider";
import en from "./locales/en";
import th from "./locales/th";
import "./WeatherWidget.css";

const minimizedStorageKey = "floodwatch.weather.minimized";

function readMinimized() {
  try {
    return sessionStorage.getItem(minimizedStorageKey) === "true";
  } catch {
    return false;
  }
}

export function WeatherWidget({ hasBottomNavigation = false }: { hasBottomNavigation?: boolean }) {
  const { locale, t } = useTranslation({ th, en });
  const { state, now, setRetry } = useWeather();
  const [minimized, setMinimized] = useState(readMinimized);
  const [detailsOpen, setDetailsOpen] = useState(false);
  const dialog = useRef<HTMLDialogElement>(null);
  const detailsButton = useRef<HTMLButtonElement>(null);
  const toggleButton = useRef<HTMLButtonElement>(null);
  const previousMinimized = useRef(minimized);
  const previousDetailsOpen = useRef(false);
  const titleId = useId();
  const dialogTitleId = useId();
  const dialogId = useId();
  const data = state.data;
  const freshness = data ? displayFreshness(data, now, state.refreshFailed) : "UNKNOWN";
  const condition = data?.current ? conditionForCode(data.current.weatherCode) : null;
  const current = freshness !== "UNKNOWN" && condition ? data?.current : null;
  const status = t(freshness === "FRESH" ? "fresh" : freshness === "STALE" ? "stale" : "unknown");

  useEffect(() => {
    if (detailsOpen) dialog.current?.showModal();
    else if (previousDetailsOpen.current) {
      if (dialog.current?.open) dialog.current.close();
      detailsButton.current?.focus();
    }
    previousDetailsOpen.current = detailsOpen;
  }, [detailsOpen]);

  useEffect(() => {
    if (previousMinimized.current !== minimized) toggleButton.current?.focus();
    previousMinimized.current = minimized;
    try {
      sessionStorage.setItem(minimizedStorageKey, String(minimized));
    } catch {
      return;
    }
  }, [minimized]);

  return <>
    <div className={`weather-floating${hasBottomNavigation ? " weather-floating--with-navigation" : ""}${minimized ? " weather-floating--minimized" : ""}`} hidden={detailsOpen}>
    {minimized ? <button ref={toggleButton} type="button" className="weather-widget__reopen" aria-label={t("widget.reopen")} title={t("widget.reopen")} onClick={() => setMinimized(false)}><WeatherIcon condition="overcast" /></button> : <section className="weather-widget" aria-labelledby={titleId}>
      <header className="weather-widget__header"><h2 id={titleId}>{t("title")}</h2><button ref={toggleButton} type="button" className="weather-widget__icon-button" aria-label={t("widget.minimize")} onClick={() => setMinimized(true)}><span aria-hidden="true">×</span></button></header>
      <div className="weather-widget__summary">
        <span className="weather-widget__condition-icon">{current && condition ? <WeatherIcon condition={condition.key} /> : <span aria-hidden="true">☁</span>}</span>
        <span className="weather-widget__current"><strong>{current ? `${formatNumber(current.temperatureC, locale, { maximumFractionDigits: 1 })}°C` : "—"}</strong><span>{state.loading && !data ? t("loading.title") : current && condition ? t(`condition.${condition.key}`) : t("unavailable.title")}</span></span>
      </div>
      {current && <dl className="weather-widget__readings"><div><dt>{t("humidityLabel")}</dt><dd><svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 2S5 10 5 15a7 7 0 0 0 14 0c0-5-7-13-7-13Z" /></svg>{formatNumber(current.humidityPercent, locale, { maximumFractionDigits: 0 })}%</dd></div><div><dt>{t("windLabel")}</dt><dd><svg viewBox="0 0 24 24" aria-hidden="true"><path d="M3 8h12a3 3 0 1 0-3-3M3 12h15a3 3 0 1 1-3 3M3 16h5" /></svg>{t("widget.wind", { value: formatNumber(current.windSpeedKmh, locale, { maximumFractionDigits: 1 }) })}</dd></div></dl>}
      <div className="weather-widget__metadata"><span>{t("widget.model")}</span><span className={`weather-widget__status weather-widget__status--${freshness.toLowerCase()}`}>{status}</span></div>
      {current && <p className="weather-widget__time">{t("modelTime", { value: new Intl.DateTimeFormat(locale, { hour: "2-digit", minute: "2-digit" }).format(new Date(current.observedAt)) })}</p>}
      {data && <p className="weather-widget__time">{t("providerLabel")}: {data.source.name}</p>}
      {state.loading && !data && <p className="weather-widget__notice" role="status">{t("loading.body")}</p>}
      {state.error && <p className="weather-widget__notice" role="status">{t("error.title")}</p>}
      {freshness === "STALE" && <p className="weather-widget__notice">{t("widget.stale")}</p>}
      {!state.loading && (state.error || !current) && <button type="button" className="weather-widget__retry" onClick={() => setRetry((value) => value + 1)}>{t("retry")}</button>}
      <button ref={detailsButton} type="button" className="weather-widget__details-button" aria-haspopup="dialog" aria-controls={dialogId} onClick={() => setDetailsOpen(true)}>{t("widget.details")}<span aria-hidden="true">↗</span></button>
    </section>}
    </div>
    {createPortal(<dialog ref={dialog} id={dialogId} className="weather-dialog" aria-labelledby={dialogTitleId} onClose={() => setDetailsOpen(false)}>
      <header className="weather-dialog__header"><h2 id={dialogTitleId}>{t("widget.dialogTitle")}</h2><button type="button" className="weather-widget__icon-button" aria-label={t("widget.closeDetails")} onClick={() => setDetailsOpen(false)}><span aria-hidden="true">×</span></button></header>
      {detailsOpen && <WeatherPanel />}
    </dialog>, document.body)}
  </>;
}
