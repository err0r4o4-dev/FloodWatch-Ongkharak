import { useId } from "react";
import { formatDate, formatNumber, useTranslation } from "../../i18n";
import { isSafeProviderUrl, type ExternalDataFreshness, type ExternalDataRecord, type ExternalDataResponse, type ExternalDataSource } from "./api";
import en from "./locales/en";
import th from "./locales/th";
import { useExternalData } from "./useExternalData";
import "./ExternalDataPanel.css";

const messages = { th, en };
const recordLimit = 3;

function dateText(value: string | null, locale: "th" | "en", unknown: string): string {
  return value ? formatDate(value, locale) : unknown;
}

function freshnessClass(value: ExternalDataFreshness): string {
  return value.toLowerCase();
}

type Translate = (key: keyof typeof th, params?: Record<string, string | number>) => string;

function localReason(reason: string | null, translate: Translate): string | null {
  if (!reason) return null;
  if (reason === "SNAPSHOT_UNAVAILABLE") return translate("reasonSnapshot");
  if (reason === "GEOGRAPHIC_SCOPE_UNCONFIRMED") return translate("reasonGeography");
  if (["SOURCE_EXPIRED", "OBSERVATION_EXPIRED", "FORECAST_EXPIRED", "ALERT_EXPIRED"].includes(reason)) return translate("reasonExpired");
  if (reason === "MISSING_MEASUREMENT") return translate("reasonMissing");
  if (reason === "CANCELLED") return translate("reasonCancelled");
  if (reason === "NOT_YET_EFFECTIVE") return translate("reasonNotEffective");
  if (reason.toLowerCase().includes("hii") || reason.toLowerCase().includes("accumulation window")) return translate("reasonHii");
  if (/^[A-Z][A-Z0-9_]+$/.test(reason)) return translate("reasonUnknown");
  return reason;
}

function sourceStatus(source: ExternalDataSource, translate: Translate): string {
  if (!source.enabled || source.status === "NOT_CONFIGURED") return translate("notConfigured");
  if (source.freshness === "STALE" || source.status === "DEGRADED") return translate("degraded");
  if (source.status === "AVAILABLE" && source.freshness === "FRESH") return translate("available");
  return translate(source.status === "UNAVAILABLE" ? "unavailable" : "sourceUnknown");
}

function sourceHealthText(source: ExternalDataSource, translate: Translate): string {
  if (source.freshness === "FRESH") return translate("sourceFresh");
  if (source.freshness === "STALE") return translate("sourceStale");
  return translate("sourceUnknown");
}

function metricLabel(record: ExternalDataRecord, translate: Translate): string {
  if (record.metric === "rain_1h") return translate("rain1h");
  if (record.metric === "rain_24h") return translate("rain24h");
  if (record.metric === "precipitation") return translate("precipitation");
  if (record.metric === "river_discharge") return translate("riverDischarge");
  if (record.metric === "waterlevel_msl") return translate("waterMsl");
  if (record.metric === "waterlevel_m") return translate("waterLevel");
  if (record.metric === "gate_in") return translate("gateIn");
  if (record.metric === "gate_out") return translate("gateOut");
  if (record.metric?.startsWith("gate_")) return translate("gate");
  return record.metric ?? translate("unknownValue");
}

function valueText(record: ExternalDataRecord, locale: "th" | "en", unknown: string): string {
  if (record.value === null) return unknown;
  return `${formatNumber(record.value, locale, { maximumFractionDigits: 2 })}${record.unit ? ` ${record.unit}` : ""}`;
}

function recordTime(record: ExternalDataRecord, locale: "th" | "en", unknown: string, translate: Translate): string {
  if (record.kind === "OBSERVATION") return translate("observedAt", { value: dateText(record.observedAt, locale, unknown) });
  if (record.kind === "FORECAST" && (record.validFrom || record.validTo)) return translate("forecastWindow", { from: dateText(record.validFrom, locale, unknown), to: dateText(record.validTo, locale, unknown) });
  if (record.kind === "OFFICIAL_ALERT") return translate("issuedAt", { value: dateText(record.issuedAt, locale, unknown) });
  return translate("observedAt", { value: unknown });
}

function RecordCard({ record, locale, translate }: { record: ExternalDataRecord; locale: "th" | "en"; translate: Translate }) {
  const unknown = translate("unknownValue");
  const reason = localReason(record.reason, translate);
  const effectiveFreshness = record.kind !== "OFFICIAL_ALERT" && record.value === null ? "UNKNOWN" : record.freshness;
  const recordStatus = effectiveFreshness === "FRESH" ? translate("recordFresh") : effectiveFreshness === "STALE" ? translate("recordStale") : translate("recordUnknown");
  const sourceKind = record.kind === "FORECAST" ? translate("model") : record.kind === "OFFICIAL_ALERT" ? translate("officialAlert") : translate("observation");

  return <article className={`external-data__record external-data__record--${freshnessClass(effectiveFreshness)}`}>
    <div className="external-data__record-head"><strong>{metricLabel(record, translate)}</strong><span className={`external-data__badge external-data__badge--${freshnessClass(effectiveFreshness)}`}>{recordStatus}</span></div>
    <p className="external-data__record-kind">{sourceKind}</p>
    {record.kind === "OFFICIAL_ALERT" ? <>
      {record.headline && <h4>{record.headline}</h4>}
      {record.description && <p>{record.description}</p>}
      <p className="external-data__record-meta">{translate("alertType", { value: record.alertMessageType ?? unknown })} · {translate("issuedAt", { value: dateText(record.issuedAt, locale, unknown) })}{record.validFrom && ` · ${translate("validFrom", { value: dateText(record.validFrom, locale, unknown) })}`}{record.validTo && ` · ${translate("expiresAt", { value: dateText(record.validTo, locale, unknown) })}`}</p>
    </> : <>
      <p className="external-data__record-value">{valueText(record, locale, unknown)}</p>
      <p className="external-data__record-meta">{recordTime(record, locale, unknown, translate)}{record.accumulationSeconds && record.metric === "precipitation" ? ` · ${record.accumulationSeconds / 3600} h` : ""}{record.station ? ` · ${record.station.name}` : ""}</p>
    </>}
    {reason && <p className="external-data__reason">{translate("reason", { value: reason })}</p>}
  </article>;
}

function SourceList({ data, locale, translate }: { data: ExternalDataResponse; locale: "th" | "en"; translate: Translate }) {
  return <div className="external-data__source-list">
    {data.sources.map((source) => <article className="external-data__source" key={source.code}>
      <header className="external-data__source-head"><div><h3>{source.name}</h3><p>{source.code} · {source.kind === "FORECAST" ? translate("model") : source.kind === "OFFICIAL_ALERT" ? translate("officialAlert") : translate("observation")}</p></div><span className={`external-data__badge external-data__badge--${source.freshness.toLowerCase()}`}>{sourceStatus(source, translate)}</span></header>
      <div className="external-data__source-meta"><span>{sourceHealthText(source, translate)}</span><span>{translate("records", { count: source.recordCount })}</span><span>{translate("lastAttemptAt", { value: dateText(source.lastAttemptAt, locale, translate("unknownValue")) })}</span><span>{translate("lastSuccessAt", { value: dateText(source.lastSuccessAt, locale, translate("unknownValue")) })}</span></div>
      {source.kind === "OFFICIAL_ALERT" && source.records.length === 0 ? <p className="external-data__empty-source">{translate("noAlerts")}</p> : source.records.length === 0 ? <p className="external-data__empty-source">{translate("noDataTitle")}</p> : <div className="external-data__records">{source.records.slice(0, recordLimit).map((record) => <RecordCard key={record.key} record={record} locale={locale} translate={translate} />)}{source.records.length > recordLimit && <p className="external-data__more">{translate("moreRecords", { count: source.records.length - recordLimit })}</p>}</div>}
      {isSafeProviderUrl(source.url) && <p className="external-data__provider"><span>{translate("provider")}:</span> <a href={source.url} target="_blank" rel="noreferrer">{new URL(source.url).hostname}</a></p>}
      {source.errorCode && <p className="external-data__provider">{translate("errorCode")}</p>}
      {source.records.some((record) => record.validFrom === null && record.validTo === null && record.kind === "FORECAST") && <p className="external-data__provider">{translate("reasonHii")}</p>}
    </article>)}
  </div>;
}

export function ExternalDataPanel() {
  const { locale, t } = useTranslation(messages);
  const { state, setRetry } = useExternalData();
  const titleId = useId();
  const data = state.data;
  const sources = data?.sources ?? [];
  const allNotConfigured = sources.length > 0 && sources.every((source) => !source.enabled || source.status === "NOT_CONFIGURED");
  const allEmpty = sources.length > 0 && sources.every((source) => source.records.length === 0);
  const hasStaleSource = sources.some((source) => source.freshness === "STALE" || source.status === "DEGRADED");
  const hasUnavailableSource = sources.some((source) => source.status === "UNAVAILABLE");

  if (state.loading && !data) return <section className="external-data-panel" aria-labelledby={titleId}><div className="external-data__heading"><div><h2 id={titleId}>{t("title")}</h2><p>{t("subtitle")}</p></div></div><div className="external-data__state" role="status" aria-label={t("loadingAria")}><h3>{t("loadingTitle")}</h3><p>{t("loadingBody")}</p></div></section>;
  if (state.error && !data) return <section className="external-data-panel" aria-labelledby={titleId}><div className="external-data__heading"><div><h2 id={titleId}>{t("title")}</h2><p>{t("subtitle")}</p></div></div><div className="external-data__state external-data__state--error" role="status" aria-label={t("errorAria")}><h3>{t("errorTitle")}</h3><p>{t("errorBody")}</p><button type="button" onClick={() => setRetry((value) => value + 1)}>{t("retry")}</button></div></section>;

  return <section className="external-data-panel" aria-labelledby={titleId}>
    <div className="external-data__heading"><div><h2 id={titleId}>{t("title")}</h2><p>{t("subtitle")}</p></div><span className={`external-data__badge external-data__badge--${hasStaleSource ? "stale" : allNotConfigured || hasUnavailableSource ? "unknown" : "fresh"}`}>{hasStaleSource ? t("sourceStale") : allNotConfigured ? t("notConfigured") : hasUnavailableSource ? t("unavailable") : t("available")}</span></div>
    {data && <p className="external-data__retrieved">{t("retrievedAt", { value: formatDate(data.retrievedAt, locale) })}</p>}
    {state.error && <div className="external-data__state external-data__state--error" role="status"><h3>{t("errorTitle")}</h3><p>{t("errorBody")}</p><button type="button" onClick={() => setRetry((value) => value + 1)}>{t("retry")}</button></div>}
    {allNotConfigured ? <div className="external-data__state"><h3>{t("notConfiguredTitle")}</h3><p>{t("notConfiguredBody")}</p></div> : allEmpty ? <div className="external-data__state"><h3>{t("noDataTitle")}</h3><p>{t("noDataBody")}</p></div> : hasStaleSource && <p className="external-data__note">{t("sourceStale")}</p>}
    <details className="external-data__details">
      <summary>{t("showSources")} <span aria-hidden="true">· {t("sourceCount", { count: sources.length })}</span></summary>
      {data && <SourceList data={data} locale={locale} translate={t} />}
    </details>
  </section>;
}
