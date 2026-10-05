import { useEffect, useState } from "react";
import { formatNumber, useTranslation } from "../../i18n";
import th from "../../i18n/locales/th/announcements";
import en from "../../i18n/locales/en/announcements";
import { PageHeroArt } from "../../shared/components/PageHeroArt";
import { DataTimestamp, FreshnessBadge, OriginalContentNote, SourceBadge, StatusBadge } from "../../shared/components/DataStates";
import { useRealtimeRevision } from "../realtime/RealtimeProvider";
import { OfficialAnnouncementsPanel } from "../official-announcements/OfficialAnnouncementsPanel";
import { ManualAnnouncementsPanel } from "../manual-announcements/ManualAnnouncementsPanel";
import { FM91NewsPanel } from "../news-announcements/FM91NewsPanel";
import { listPublicAlerts } from "./api";
import type { PublicAlert } from "./api";
import "./AnnouncementsPage.css";

const alertEvents = ["alert.created"] as const;
const filters: Array<{ severity: PublicAlert["severity"] | "ALL"; key: keyof typeof th }> = [
  { severity: "ALL", key: "filter.all" },
  { severity: "CRITICAL", key: "filter.critical" },
  { severity: "WARNING", key: "filter.warning" },
  { severity: "WATCH", key: "filter.watch" },
  { severity: "INFO", key: "filter.info" }
];

function matchesFilter(alert: PublicAlert, filter: PublicAlert["severity"] | "ALL") {
  return filter === "ALL" || alert.severity === filter;
}

export function AnnouncementsPage() {
  const { locale, t } = useTranslation({ th, en });
  const realtimeRevision = useRealtimeRevision(alertEvents);
  const [retry, setRetry] = useState(0);
  const [filter, setFilter] = useState<PublicAlert["severity"] | "ALL">("ALL");
  const [result, setResult] = useState<{ state: "loading" | "ready" | "error"; items: PublicAlert[] }>({ state: "loading", items: [] });

  useEffect(() => {
    const controller = new AbortController();
    setResult((current) => ({ ...current, state: "loading" }));
    void listPublicAlerts(controller.signal).then((items) => {
      if (!controller.signal.aborted) setResult({ state: "ready", items });
    }).catch(() => {
      if (!controller.signal.aborted) setResult({ state: "error", items: [] });
    });
    return () => controller.abort();
  }, [retry, realtimeRevision]);

  const visibleAlerts = result.items.filter((alert) => matchesFilter(alert, filter));
  const featuredAlert = visibleAlerts[0];
  const hasAlerts = result.items.length > 0;
  const showError = result.state === "error";
  const showLoading = result.state === "loading";
  const summaryState = showError ? t("summary.unavailable") : showLoading ? t("summary.loading") : !hasAlerts ? t("summary.empty") : null;
  const sourceCounts = result.items.reduce<Record<PublicAlert["sourceType"], number>>((counts, alert) => {
    counts[alert.sourceType] += 1;
    return counts;
  }, { OFFICIAL: 0, COMMUNITY: 0, AI: 0 });

  return <section className="feature-page announcements-page">
    <header className="page-hero">
      <div className="page-hero__copy">
        <p className="eyebrow">{t("eyebrow")}</p>
        <h1>{t("title")}</h1>
        <p>{t("description")}</p>
      </div>
      <PageHeroArt kind="announcement" />
    </header>
    <OfficialAnnouncementsPanel />
    <FM91NewsPanel />
    <ManualAnnouncementsPanel />
    <div className="announcements-page__layout">
        <div className="announcements-page__main">
          <div className="announcement-filters" aria-label={t("filters.label")}>
            {filters.map((option) => {
              const count = result.items.filter((alert) => matchesFilter(alert, option.severity)).length;
              return <button key={option.severity} type="button" className="announcement-filter" aria-pressed={filter === option.severity} onClick={() => setFilter(option.severity)}>
                {t(option.key)}{showError || showLoading ? "" : ` (${formatNumber(count, locale)})`}
              </button>;
            })}
          </div>

          {showLoading ? <section className="announcement-state" role="status" aria-label={t("loading.title")}><h2>{t("loading.title")}</h2><p>{t("loading.description")}</p></section> :
            showError ? <section className="announcement-state announcement-state--error" role="alert"><h2>{t("error.title")}</h2><p>{t("error.description")}</p><button type="button" className="action-button" onClick={() => setRetry((value) => value + 1)}>{t("error.retry")}</button></section> :
            !hasAlerts ? <section className="announcement-state"><h2>{t("empty.title")}</h2><p>{t("empty.description")}</p></section> :
            featuredAlert ? <article className={`announcement-feature announcement-feature--${featuredAlert.severity.toLowerCase()}`}>
            <div className="announcement-feature__content">
              <div className="announcement-feature__heading"><StatusBadge status={featuredAlert.severity} /><span className="announcement-feature__label">{t("featured.label")}</span></div>
              <OriginalContentNote />
              <h2>{featuredAlert.title}</h2>
              <p>{featuredAlert.message}</p>
              <div className="announcement-feature__metadata"><DataTimestamp label={t("timestamp.triggered")} value={featuredAlert.triggeredAt} /><SourceBadge type={featuredAlert.sourceType} /></div>
            </div>
          </article> : <section className="announcement-state"><h2>{t("filteredEmpty.title")}</h2><p>{t("filteredEmpty.description")}</p></section>}

          {!showLoading && !showError && visibleAlerts.length > 0 && <section className="data-section announcements-list" aria-labelledby="published-alerts-heading">
            <div className="overview-section-header announcements-list__heading"><div><h2 id="published-alerts-heading">{t("list.title")}</h2><p className="section-caption">{t("list.description")}</p></div><span>{t("list.count", { count: formatNumber(visibleAlerts.length, locale) })}</span></div>
            <ul className="data-list">{visibleAlerts.map((alert) => <li key={alert.id}>
              <article className="data-card announcement-row">
                <div className="announcement-row__severity"><StatusBadge status={alert.severity} /></div>
                <div className="announcement-row__content"><OriginalContentNote /><h3>{alert.title}</h3><p className="section-caption">{alert.message}</p><div className="badge-row"><SourceBadge type={alert.sourceType} /><FreshnessBadge freshness="UNKNOWN" /><span className="badge">{alert.resourceType}</span></div></div>
                <div className="data-card__meta announcement-row__meta"><DataTimestamp label={t("timestamp.triggered")} value={alert.triggeredAt} /><span>{t("currentEvidence")}</span></div>
              </article>
            </li>)}</ul>
          </section>}
        </div>
        <aside className="announcements-page__sidebar" aria-label={t("summary.aria")}>
          <section className="data-section announcement-summary"><div className="overview-section-header announcement-summary__heading"><h2>{t("summary.title")}</h2><span>{t("summary.published")}</span></div><div className="announcement-summary__grid">
            {summaryState ? <p className="announcement-summary__state">{summaryState}</p> : <>
            <div><strong>{formatNumber(result.items.length, locale)}</strong><span>{t("summary.all")}</span></div>
            <div><strong>{formatNumber(result.items.filter((alert) => alert.severity === "CRITICAL" || alert.severity === "WARNING").length, locale)}</strong><span>{t("summary.attention")}</span></div>
            <div><strong>{formatNumber(result.items.filter((alert) => alert.severity === "WATCH").length, locale)}</strong><span>{t("summary.watch")}</span></div>
            <div><strong>{formatNumber(result.items.filter((alert) => alert.severity === "INFO").length, locale)}</strong><span>{t("summary.info")}</span></div>
            </>}
          </div></section>
          <section className="data-section announcement-sidebar-card"><h2>{t("sources.title")}</h2><p className="section-caption">{showError ? t("sources.unavailable") : showLoading ? t("sources.loading") : !hasAlerts ? t("sources.empty") : t("sources.description")}</p>
            {hasAlerts && !showError && !showLoading && <ul className="announcement-source-list">{(Object.entries(sourceCounts) as Array<[PublicAlert["sourceType"], number]>).map(([sourceType, count]) => <li key={sourceType}><SourceBadge type={sourceType} /><strong>{t("sources.count", { count: formatNumber(count, locale) })}</strong></li>)}</ul>}
          </section>
          <section className="data-section announcement-sidebar-card"><h2>{t("note.title")}</h2><p className="section-caption">{t("note.description")}</p></section>
        </aside>
      </div>
  </section>;
}
