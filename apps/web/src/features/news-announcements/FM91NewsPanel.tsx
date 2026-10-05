import { useId } from "react";
import { formatDate, formatNumber, useTranslation } from "../../i18n";
import { isSafeFM91ImageUrl, type FM91NewsFreshness, type FM91NewsItem, type FM91NewsSourceStatus } from "./api";
import en from "./locales/en";
import th from "./locales/th";
import { useFM91News } from "./useFM91News";
import "./FM91NewsPanel.css";

const messages = { th, en };
type Translate = (key: keyof typeof th, params?: Record<string, string | number>) => string;

function freshnessLabel(value: FM91NewsFreshness, t: Translate): string {
  return t(value === "FRESH" ? "freshnessFresh" : value === "STALE" ? "freshnessStale" : "freshnessUnknown");
}

function sourceStatusLabel(value: FM91NewsSourceStatus, t: Translate): string {
  return t(value === "AVAILABLE" ? "sourceAvailable" : value === "DEGRADED" ? "sourceDegraded" : value === "UNAVAILABLE" ? "sourceUnavailable" : "sourceNotConfigured");
}

function excerpt(value: string, unknown: string): string {
  const text = value.trim();
  if (!text) return unknown;
  return text.length > 280 ? `${text.slice(0, 280).trimEnd()}…` : text;
}

function ArticleMetadata({ item, locale, t }: { item: FM91NewsItem; locale: "th" | "en"; t: Translate }) {
  const date = (value: string | null) => value ? formatDate(value, locale) : t("unknown");
  return <div className="fm91-news__metadata">
    <span>{t("category", { value: item.category || t("unknown") })}</span>
    <span>{t("publishedAt", { value: date(item.publishedAt) })}</span>
    {item.observedAt && <span>{t("observedAt", { value: date(item.observedAt) })}</span>}
    {item.locations.length > 0 ? <span>{t("location", { value: item.locations.join(", ") })}</span> : item.latitude !== null && item.longitude !== null ? <span>{t("coordinates")}</span> : <span>{t("noCoordinates")}</span>}
    {item.roads.length > 0 && <span>{t("road", { value: item.roads.join(", ") })}</span>}
    {item.eventTimes.length > 0 && <span>{t("eventTime", { value: item.eventTimes.join(", ") })}</span>}
    {item.waterLevels.length > 0 && <span>{t("waterLevel", { value: item.waterLevels.join(", ") })}</span>}
    {item.travelAdvice.length > 0 && <span>{t("travelAdvice", { value: item.travelAdvice.join(", ") })}</span>}
    {item.reason && <span>{t("reason", { value: item.reason })}</span>}
  </div>;
}

function NewsItem({ item, locale, t }: { item: FM91NewsItem; locale: "th" | "en"; t: Translate }) {
  const image = item.imageURLs.find(isSafeFM91ImageUrl);
  return <article className={`fm91-news__item fm91-news__item--${item.freshness.toLowerCase()}`}>
    <div className="fm91-news__item-heading"><div className="fm91-news__badges"><span className="fm91-news__badge fm91-news__badge--media">{t("mediaNotice")}</span><span className="fm91-news__badge fm91-news__badge--freshness">{freshnessLabel(item.freshness, t)}</span></div><span className="fm91-news__auto">{t("autoPublished")}</span></div>
    {image && <img className="fm91-news__image" src={image} alt="" loading="lazy" />}
    <h3>{item.title}</h3>
    <p className="fm91-news__excerpt">{excerpt(item.description, t("unknown"))}</p>
    <p className="fm91-news__original">{t("originalText")}</p>
    <ArticleMetadata item={item} locale={locale} t={t} />
    <a className="fm91-news__link" href={item.url} target="_blank" rel="noreferrer">{t("sourceLink")}</a>
  </article>;
}

function PanelHeading({ titleId, t }: { titleId: string; t: Translate }) {
  return <div className="fm91-news__heading"><div><p className="fm91-news__eyebrow">{t("eyebrow")}</p><h2 id={titleId}>{t("title")}</h2><p>{t("subtitle")}</p></div></div>;
}

export function FM91NewsPanel({ variant = "default" }: { variant?: "default" | "compact" }) {
  const { locale, t } = useTranslation(messages);
  const { state, setRetry } = useFM91News();
  const titleId = useId();
  const data = state.data;
  const unavailable = data?.source.status === "UNAVAILABLE";
  const disabled = data && (!data.source.enabled || data.source.status === "NOT_CONFIGURED");
  const stale = data && (data.source.freshness === "STALE" || data.source.status === "DEGRADED" || data.items.some((item) => item.freshness === "STALE"));

  if (state.loading && !data) return <section className={`fm91-news fm91-news--${variant}`} aria-labelledby={titleId}><PanelHeading titleId={titleId} t={t} /><div className="fm91-news__state" role="status" aria-label={t("loadingTitle")}><h3>{t("loadingTitle")}</h3><p>{t("loadingBody")}</p></div></section>;
  if (state.error || !data) return <section className={`fm91-news fm91-news--${variant}`} aria-labelledby={titleId}><PanelHeading titleId={titleId} t={t} /><div className="fm91-news__state fm91-news__state--error" role="status"><h3>{t("unavailableTitle")}</h3><p>{t("unavailableBody")}</p><button type="button" onClick={() => setRetry((value) => value + 1)}>{t("retry")}</button></div></section>;

  const items = variant === "compact" ? data.items.slice(0, 3) : data.items;
  return <section className={`fm91-news fm91-news--${variant}`} aria-labelledby={titleId} aria-busy={state.loading}>
    <PanelHeading titleId={titleId} t={t} />
    <div className="fm91-news__source-row"><div className="fm91-news__source-meta"><span className="fm91-news__source-code">{data.source.code}</span><span>{t("sourceMedia")}</span><span>{t("retrievedAt", { value: formatDate(data.retrievedAt, locale) })}</span>{data.source.lastSuccessAt && <span>{t("lastSuccess", { value: formatDate(data.source.lastSuccessAt, locale) })}</span>}</div><div className="fm91-news__source-actions"><span className={`fm91-news__source-status fm91-news__source-status--${data.source.status.toLowerCase()}`}>{sourceStatusLabel(data.source.status, t)}</span><a href={data.source.url} target="_blank" rel="noreferrer">{t("sourceLink")}</a></div></div>
    {stale && !disabled && !unavailable && <div className="fm91-news__notice" role="status"><strong>{t("staleTitle")}</strong><span>{t("staleBody")}</span></div>}
    {disabled ? <div className="fm91-news__state"><h3>{t("disabledTitle")}</h3><p>{t("disabledBody")}</p></div> : unavailable ? <div className="fm91-news__state fm91-news__state--error" role="status"><h3>{t("unavailableTitle")}</h3><p>{t("unavailableBody")}</p><button type="button" onClick={() => setRetry((value) => value + 1)}>{t("retry")}</button></div> : data.items.length === 0 ? <div className="fm91-news__state"><h3>{t("emptyTitle")}</h3><p>{t("emptyBody")}</p></div> : <div className="fm91-news__items" aria-label={t("more", { count: formatNumber(items.length, locale) })}>{items.map((item) => <NewsItem key={item.id} item={item} locale={locale} t={t} />)}</div>}
  </section>;
}
