import { useId } from "react";
import { formatDate, formatNumber, useTranslation } from "../../i18n";
import type { OfficialAnnouncementEffectiveStatus, OfficialAnnouncementFreshness, OfficialAnnouncementItem, OfficialAnnouncementSourceStatus } from "./api";
import en from "./locales/en";
import th from "./locales/th";
import { useOfficialAnnouncements } from "./useOfficialAnnouncements";
import "./OfficialAnnouncementsPanel.css";

const messages = { th, en };

function statusLabel(status: OfficialAnnouncementEffectiveStatus, translate: (key: keyof typeof th) => string): string {
  const keys: Record<OfficialAnnouncementEffectiveStatus, keyof typeof th> = {
    ACTIVE: "statusActive",
    UPDATED: "statusUpdated",
    CANCELLED: "statusCancelled",
    EXPIRED: "statusExpired",
    UNKNOWN: "statusUnknown"
  };
  return translate(keys[status]);
}

function freshnessLabel(freshness: OfficialAnnouncementFreshness, translate: (key: keyof typeof th) => string): string {
  const keys: Record<OfficialAnnouncementFreshness, keyof typeof th> = {
    FRESH: "freshnessFresh",
    STALE: "freshnessStale",
    UNKNOWN: "freshnessUnknown"
  };
  return translate(keys[freshness]);
}

function sourceStatusLabel(status: OfficialAnnouncementSourceStatus, translate: (key: keyof typeof th) => string): string {
  const keys: Record<OfficialAnnouncementSourceStatus, keyof typeof th> = {
    AVAILABLE: "sourceAvailable",
    DEGRADED: "sourceDegraded",
    UNAVAILABLE: "sourceUnavailable",
    NOT_CONFIGURED: "sourceNotConfigured"
  };
  return translate(keys[status]);
}

function reasonLabel(reason: string, translate: (key: keyof typeof th, params?: Record<string, string | number>) => string): string {
  const keys: Partial<Record<string, keyof typeof th>> = {
    GEOGRAPHIC_SCOPE_UNCONFIRMED: "reasonGeography",
    SOURCE_UNAVAILABLE: "reasonSourceUnavailable",
    MISSING_EXPIRY: "reasonMissingExpiry",
    NOT_YET_EFFECTIVE: "reasonNotYetEffective"
  };
  const key = keys[reason];
  return key ? translate(key) : translate("reason", { value: reason });
}

function effectiveStatus(item: OfficialAnnouncementItem): OfficialAnnouncementEffectiveStatus {
  if (item.status === "CANCELLED" || item.status === "EXPIRED") return item.effectiveStatus;
  return item.freshness === "FRESH" ? item.effectiveStatus : "UNKNOWN";
}

function AnnouncementStatus({ status, translate }: { status: OfficialAnnouncementEffectiveStatus; translate: (key: keyof typeof th) => string }) {
  return <span className={`official-announcements__status official-announcements__status--${status.toLowerCase()}`}>{statusLabel(status, translate)}</span>;
}

function FreshnessStatus({ freshness, translate }: { freshness: OfficialAnnouncementFreshness; translate: (key: keyof typeof th) => string }) {
  return <span className={`official-announcements__freshness official-announcements__freshness--${freshness.toLowerCase()}`}>{freshnessLabel(freshness, translate)}</span>;
}

function AnnouncementItem({ item, locale, translate }: { item: OfficialAnnouncementItem; locale: "th" | "en"; translate: (key: keyof typeof th, params?: Record<string, string | number>) => string }) {
  const status = effectiveStatus(item);
  return <article className="official-announcements__item">
    <div className="official-announcements__item-heading">
      <div className="official-announcements__badges"><AnnouncementStatus status={status} translate={translate} /><FreshnessStatus freshness={item.freshness} translate={translate} /></div>
      <span className="official-announcements__identifier">{item.identifier}</span>
    </div>
    <h3>{item.headline}</h3>
    <p>{item.description}</p>
    <div className="official-announcements__metadata">
      <time dateTime={item.issuedAt}>{translate("issuedAt", { value: formatDate(item.issuedAt, locale) })}</time>
      {item.expiresAt && <time dateTime={item.expiresAt}>{translate("expiresAt", { value: formatDate(item.expiresAt, locale) })}</time>}
      {item.areaCodes.length > 0 && <span>{translate("areas", { value: item.areaCodes.join(", ") })}</span>}
      {item.reason && <span>{reasonLabel(item.reason, translate)}</span>}
    </div>
    {item.historyTotal > 0 && <details className="official-announcements__history">
      <summary>{translate("history", { count: formatNumber(item.historyTotal, locale) })}</summary>
      <ol>{item.history.map((entry) => <li key={entry.evidenceId}>
        <strong>{translate("historyEvidence", { value: entry.identifier })}</strong>
        <span>{translate("messageType", { value: entry.messageType })}</span>
        <time dateTime={entry.issuedAt}>{translate("issuedAt", { value: formatDate(entry.issuedAt, locale) })}</time>
        <time dateTime={entry.retrievedAt}>{translate("historyRetrievedAt", { value: formatDate(entry.retrievedAt, locale) })}</time>
        <span>{translate("expiresAt", { value: entry.expiresAt ? formatDate(entry.expiresAt, locale) : translate("unknownValue") })}</span>
        <span>{translate("areas", { value: entry.areaCodes.length > 0 ? entry.areaCodes.join(", ") : translate("unknownValue") })}</span>
        <p>{entry.headline}</p>
        <p>{entry.description}</p>
        {entry.reason && <p>{reasonLabel(entry.reason, translate)}</p>}
      </li>)}</ol>
    </details>}
  </article>;
}

export function OfficialAnnouncementsPanel() {
  const { locale, t } = useTranslation(messages);
  const { state, setRetry } = useOfficialAnnouncements();
  const titleId = useId();
  const data = state.data;

  if (state.loading && !data) return <section className="official-announcements" aria-labelledby={titleId}><PanelHeading titleId={titleId} translate={t} /><div className="official-announcements__state" role="status" aria-label={t("loadingTitle")}><h3>{t("loadingTitle")}</h3><p>{t("loadingBody")}</p></div></section>;
  if (state.error || !data) return <section className="official-announcements" aria-labelledby={titleId}><PanelHeading titleId={titleId} translate={t} /><div className="official-announcements__state official-announcements__state--error" role="status"><h3>{t("unavailableTitle")}</h3><p>{t("unavailableBody")}</p><button type="button" onClick={() => setRetry((value) => value + 1)}>{t("retry")}</button></div></section>;

  const source = data.source;
  const disabled = !source.enabled || source.status === "NOT_CONFIGURED";
  const unavailable = source.status === "UNAVAILABLE";
  const stale = source.status === "DEGRADED" || source.freshness === "STALE" || data.items.some((item) => item.freshness === "STALE");
  const sourceMessage = disabled ? <div className="official-announcements__state"><h3>{t("disabledTitle")}</h3><p>{t("disabledBody")}</p></div> : unavailable ? <div className="official-announcements__state official-announcements__state--error" role="status"><h3>{t("unavailableTitle")}</h3><p>{t("unavailableBody")}</p><button type="button" onClick={() => setRetry((value) => value + 1)}>{t("retry")}</button></div> : data.items.length === 0 ? <div className="official-announcements__state"><h3>{t("emptyTitle")}</h3><p>{t("emptyBody")}</p></div> : null;

  return <section className="official-announcements" aria-labelledby={titleId} aria-busy={state.loading}>
    <PanelHeading titleId={titleId} translate={t} />
    <div className="official-announcements__source-row">
      <div className="official-announcements__source-meta"><span className="official-announcements__source-code">{source.code}</span><span>{source.name}</span><span>{t("retrievedAt", { value: formatDate(data.retrievedAt, locale) })}</span>{source.lastSuccessAt && <span>{t("lastSuccess", { value: formatDate(source.lastSuccessAt, locale) })}</span>}</div>
      <div className="official-announcements__source-actions"><span className={`official-announcements__source-status official-announcements__source-status--${source.status.toLowerCase()}`}>{sourceStatusLabel(source.status, t)}</span>{source.url === "https://www.tmd.go.th" && <a href={source.url} target="_blank" rel="noreferrer">{t("officialLink")}</a>}</div>
    </div>
    {stale && !disabled && !unavailable && <div className="official-announcements__notice" role="status"><strong>{t("staleTitle")}</strong><span>{t("staleBody")}</span></div>}
    {sourceMessage}
    {!disabled && !unavailable && data.items.length > 0 && <div className="official-announcements__items" aria-label={t("itemCount", { count: formatNumber(data.items.length, locale) })}>{data.items.map((item) => <AnnouncementItem key={item.id} item={item} locale={locale} translate={t} />)}</div>}
  </section>;
}

function PanelHeading({ titleId, translate }: { titleId: string; translate: (key: keyof typeof th, params?: Record<string, string | number>) => string }) {
  return <div className="official-announcements__heading"><div><p className="official-announcements__eyebrow">{translate("eyebrow")}</p><h2 id={titleId}>{translate("title")}</h2><p>{translate("subtitle")}</p></div></div>;
}
