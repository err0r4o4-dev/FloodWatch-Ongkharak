import { useEffect, useState, type ReactNode } from "react";
import { Link } from "react-router";
import { DataTimestamp, ErrorState, FreshnessBadge, LoadingState, OriginalContentNote, SourceBadge, StatusBadge } from "../../shared/components/DataStates";
import { PageHeroArt } from "../../shared/components/PageHeroArt";
import type { Freshness, SourceRef } from "../../shared/types/flood";
import { listPublicAlerts, type PublicAlert } from "../announcements/api";
import { reportsApi } from "../reports/api";
import { ReportMediaImage } from "../reports/ReportMediaImage";
import type { CommunityReport } from "../reports/types";
import { useRealtimeRevision } from "../realtime/RealtimeProvider";
import { formatDate, formatNumber, useTranslation } from "../../i18n";
import th from "../../i18n/locales/th/overview";
import en from "../../i18n/locales/en/overview";
import { loadCollection, type Collections, type CollectionKey, type WaterStation } from "./api";
import "./OverviewPage.css";

const overviewEvents = ["water_level.updated", "flood_event.created", "flood_event.updated", "road.status.changed", "alert.created", "report.verified"] as const;
const keys: CollectionKey[] = ["events", "risks", "roads", "water"];
type FeedKey = CollectionKey | "alerts" | "reports";
type OverviewCollections = Collections & { alerts: PublicAlert[]; reports: CommunityReport[] };
type Result = { collections: Partial<OverviewCollections>; failed: FeedKey[] };
const headingKeys: Record<FeedKey, keyof typeof th> = {
  events: "heading.events", risks: "heading.risks", roads: "heading.roads", water: "heading.water", alerts: "heading.alerts", reports: "heading.reports"
};
const reportLabelKeys: Record<CommunityReport["type"], keyof typeof th> = { FLOOD: "label.floodReport", ROAD: "label.roadReport", OTHER: "label.otherReport" };
type OverviewIconName = "water" | "rain" | "risk" | "road" | "announcement" | "people";

function OverviewIcon({ name }: { name: OverviewIconName }) {
  const paths: Record<OverviewIconName, ReactNode> = {
    water: <><path d="M3 7c3-4 6 4 9 0s6 4 9 0M3 12c3-4 6 4 9 0s6 4 9 0M3 17c3-4 6 4 9 0s6 4 9 0" /></>,
    rain: <><path d="M6 15a4 4 0 1 1 1-8 5 5 0 0 1 10 0 4 4 0 0 1 1 8M7 18l-1 3M12 18l-1 3M17 18l-1 3" /></>,
    risk: <><path d="m12 3 10 18H2L12 3Z" /><path d="M12 9v5M12 17h.01" /></>,
    road: <><path d="M7 3 3 21M17 3l4 18M12 3v3M12 10v4M12 18v3" /></>,
    announcement: <><path d="m3 9 16-5v15L3 14V9ZM19 9h2v5h-2M6 15l2 6h4l-2-5" /></>,
    people: <><circle cx="9" cy="8" r="3" /><path d="M3 21v-2a6 6 0 0 1 12 0v2M17 5a3 3 0 0 1 0 6M18 15a5 5 0 0 1 3 5" /></>
  };
  return <svg viewBox="0 0 24 24" width="24" height="24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">{paths[name]}</svg>;
}

function effectiveStatus(value: string, freshness: Freshness, verification?: string) {
  return freshness === "FRESH" && (!verification || verification === "VERIFIED") ? value : "UNKNOWN";
}

function latest<T>(items: T[] | undefined, timestamp: (item: T) => string | null, limit = 5) {
  const time = (item: T) => Date.parse(timestamp(item) ?? "") || 0;
  return [...(items ?? [])].sort((first, second) => time(second) - time(first)).slice(0, limit);
}

function OverviewPanel({ id, title, icon, to, caption, children }: {
  id: string; title: string; icon: OverviewIconName; to: string; caption?: string; children: ReactNode;
}) {
  const { t } = useTranslation({ th, en });
  return <section className="data-section overview-panel" aria-labelledby={id}>
    <div className="overview-section-header"><h2 className="overview-section-title" id={id}><span className="overview-section-icon" aria-hidden="true"><OverviewIcon name={icon} /></span>{title}</h2><Link className="overview-section-action" to={to} aria-label={t("section.viewAllAria", { title })}>{t("section.viewAll")}</Link></div>
    {caption && <p className="section-caption">{caption}</p>}
    {children}
  </section>;
}

function PanelState({ result, feed, empty }: { result: Result | null; feed: FeedKey; empty: string }) {
  const { t } = useTranslation({ th, en });
  return <p className="overview-panel-state" role="status">{!result ? t("panel.loading") : result.failed.includes(feed) ? t("panel.failed", { heading: t(headingKeys[feed]) }) : empty}</p>;
}

function Evidence({ source, freshness }: { source: SourceRef | null; freshness: Freshness }) {
  const { t } = useTranslation({ th, en });
  return <div className="badge-row overview-evidence">{source ? <SourceBadge type={source.sourceType} /> : <span className="badge">{t("panel.noSource")}</span>}<FreshnessBadge freshness={freshness} /></div>;
}

function Observation({ source, time, label }: { source: SourceRef | null; time: string | null; label?: string }) {
  const { t } = useTranslation({ th, en });
  return <div className="overview-cell-meta">{source?.name ?? t("panel.sourceMissing")} · <DataTimestamp value={time} label={label} /></div>;
}

function CompactObservation({ value }: { value: string | null }) {
  const { locale, t } = useTranslation({ th, en });
  const date = value ? new Date(value) : null;
  if (!date || Number.isNaN(date.getTime())) return <span className="overview-cell-meta">{t("panel.timeMissing")}</span>;
  const full = formatDate(value, locale);
  const time = formatDate(value, locale, { hour: "2-digit", minute: "2-digit", hourCycle: "h23" });
  const day = formatDate(value, locale, { day: "numeric", month: "short", year: "2-digit" });
  return <time className="overview-reading-time" dateTime={value!} title={full} aria-label={t("value.observedAt", { value: full })}><strong>{time}</strong><small>{day}</small></time>;
}

function Table({ label, columns, children }: { label: string; columns: string[]; children: ReactNode }) {
  const { t } = useTranslation({ th, en });
  return <div className="overview-table-wrap" tabIndex={0} role="region" aria-label={t("table.scroll", { label })}><table className="overview-table"><caption className="sr-only">{label}</caption><thead><tr>{columns.map((column) => <th key={column} scope="col">{column}</th>)}</tr></thead><tbody>{children}</tbody></table></div>;
}

function ReadingTable({ stations, rainfall = false }: { stations: WaterStation[]; rainfall?: boolean }) {
  const { locale, t } = useTranslation({ th, en });
  const tableLabel = rainfall ? t("label.rainfall") : t("label.waterLevel");
  return <Table label={tableLabel} columns={[t("table.waterSource"), rainfall ? t("table.rainfall") : t("table.reportedValue"), t("table.status"), t("table.observed")] }>
    {stations.map((station) => {
      const reading = station.latestReading;
      const source = reading?.source ?? station.source;
      const value = rainfall ? reading?.rainfallMm : reading?.waterLevelM;
      const known = value !== null && value !== undefined;
      return <tr key={station.id}><th scope="row">{station.name}<div className="overview-cell-source"><SourceBadge type={source.sourceType} /><small>{source.name}</small></div></th><td>{known ? <strong>{rainfall ? t("value.rainfall", { value: formatNumber(value, locale, { maximumFractionDigits: 2 }) }) : t("value.waterLevel", { value: formatNumber(value, locale, { maximumFractionDigits: 2 }) })}</strong> : t("value.noObservation")}{!rainfall && reading?.flowRateM3s !== null && reading?.flowRateM3s !== undefined && <p>{t("value.flowRate", { value: formatNumber(reading.flowRateM3s, locale, { maximumFractionDigits: 2 }) })}</p>}</td>
        <td><StatusBadge status={known && reading ? effectiveStatus("INFO", reading.freshness, reading.verificationStatus) : "UNKNOWN"} /><FreshnessBadge freshness={reading?.freshness ?? "UNKNOWN"} /></td>
        <td><CompactObservation value={reading?.observedAt ?? null} /></td></tr>;
    })}
  </Table>;
}

export function OverviewPage() {
  const { t } = useTranslation({ th, en });
  const [result, setResult] = useState<Result | null>(null);
  const [revision, setRevision] = useState(0);
  const realtimeRevision = useRealtimeRevision(overviewEvents);

  useEffect(() => {
    const controller = new AbortController();
    setResult(null);
    const requests: Array<{ key: FeedKey; load: Promise<OverviewCollections[FeedKey]> }> = [
      ...keys.map((key) => ({ key, load: loadCollection(key, controller.signal) })),
      { key: "alerts", load: listPublicAlerts(controller.signal) },
      { key: "reports", load: reportsApi.listPublic({ signal: controller.signal }).then((page) => page.items.filter((report) => report.status === "VERIFIED" && report.verificationStatus === "VERIFIED" && report.sourceType === "COMMUNITY")) }
    ];
    void Promise.allSettled(requests.map((request) => request.load)).then((outcomes) => {
      if (controller.signal.aborted) return;
      const collections: Partial<OverviewCollections> = {};
      const failed: FeedKey[] = [];
      outcomes.forEach((outcome, index) => {
        const key = requests[index].key;
        if (outcome.status === "rejected") failed.push(key);
        else Object.assign(collections, { [key]: outcome.value });
      });
      setResult({ collections, failed });
    });
    return () => controller.abort();
  }, [revision, realtimeRevision]);

  const collections = result?.collections;
  const water = latest(collections?.water, (station) => station.latestReading?.observedAt ?? null);
  const risks = latest(collections?.risks, (risk) => risk.validFrom);
  const roads = latest(collections?.roads, (road) => road.currentCondition.observedAt);
  const events = latest(collections?.events, (event) => event.observedAt);
  const alerts = latest(collections?.alerts, (alert) => alert.triggeredAt, 3);
  const reports = latest(collections?.reports, (report) => report.createdAt, 3);
  const allFailed = result?.failed.length === keys.length + 2;
  const hasItems = collections && Object.values(collections).some((items) => items.length > 0);
  const metrics = [
    { key: "risks", label: t("metric.risks"), to: "/risk", icon: "risk", tone: "danger", caption: t("metric.risksCaption") },
    { key: "water", label: t("metric.water"), to: "/map", icon: "water", tone: "", caption: t("metric.waterCaption") },
    { key: "events", label: t("metric.events"), to: "/map", icon: "rain", tone: "sky", caption: t("metric.eventsCaption") },
    { key: "roads", label: t("metric.roads"), to: "/map", icon: "road", tone: "road", caption: t("metric.roadsCaption") }
  ] as const;

  return <div className="overview-page">
    <header className="page-hero"><div className="page-hero__copy"><p className="eyebrow">{t("hero.eyebrow")}</p><h1>{t("hero.title")}</h1><p>{t("hero.location")}</p></div><PageHeroArt kind="water" /></header>
    <div className="summary-metrics" aria-label={result ? t("summary.loaded") : t("summary.loading")}>
      {metrics.map((metric) => <Link key={metric.key} to={metric.to} className="summary-metric"><span className={`summary-metric__icon${metric.tone ? ` summary-metric__icon--${metric.tone}` : ""}`} aria-hidden="true"><OverviewIcon name={metric.icon} /></span><span>{metric.label}<strong>{collections?.[metric.key]?.length ?? "—"}</strong><small>{!result ? t("summary.loadingData") : result.failed.includes(metric.key) ? t("summary.unavailable") : metric.caption}</small></span><span className="summary-metric__arrow" aria-hidden="true">›</span></Link>)}
    </div>
    {!result && <LoadingState />}
    {allFailed ? <ErrorState onRetry={() => setRevision((previous) => previous + 1)} /> : result && result.failed.length > 0 && <div className="notice" role="status"><strong>{t("partial.title")}</strong><span>{t("partial.body", { feeds: result.failed.map((key) => t(headingKeys[key])).join(", ") })}</span><button type="button" onClick={() => setRevision((previous) => previous + 1)}>{t("retry")}</button></div>}
    <OriginalContentNote />
    {result && !allFailed && !hasItems && null}
    <div className="overview-dashboard"><div className="overview-primary">
      <OverviewPanel id="water-title" title={t("water.title")} icon="water" to="/map" caption={t("water.caption")}>
        {water.length ? <ReadingTable stations={water} /> : <PanelState result={result} feed="water" empty={t("panel.noWater")} />}
      </OverviewPanel>
      <OverviewPanel id="rainfall-title" title={t("rain.title")} icon="rain" to="/map" caption={t("rain.caption")}>
        {water.length ? <ReadingTable stations={water} rainfall /> : <PanelState result={result} feed="water" empty={t("panel.noRain")} />}
      </OverviewPanel>
      <OverviewPanel id="risks-title" title={t("risk.title")} icon="risk" to="/risk" caption={t("risk.caption")}>
        {risks.length ? <Table label={t("table.area")} columns={[t("table.area"), t("table.riskLevel"), t("table.detailsSource")]}>{risks.map((risk) => <tr key={risk.id}><th scope="row">{risk.name}</th><td><StatusBadge status={effectiveStatus(risk.effectiveRiskLevel, risk.freshness)} /><Evidence source={risk.source} freshness={risk.freshness} /></td><td>{risk.description}<Observation source={risk.source} time={risk.validFrom} label={t("label.effectiveFrom")} />{risk.validTo && <DataTimestamp value={risk.validTo} label={t("label.effectiveUntil")} />}</td></tr>)}</Table> : <PanelState result={result} feed="risks" empty={t("panel.noRisk")} />}
      </OverviewPanel>
      <OverviewPanel id="roads-title" title={t("road.title")} icon="road" to="/map" caption={t("road.caption")}>
        {roads.length ? <Table label={t("table.roadName")} columns={[t("table.roadName"), t("table.status"), t("table.sourceTime")]}>{roads.map((road) => <tr key={road.id}><th scope="row">{road.name}</th><td><StatusBadge status={effectiveStatus(road.currentCondition.status, road.currentCondition.freshness, road.currentCondition.verificationStatus)} /><Evidence source={road.currentCondition.source} freshness={road.currentCondition.freshness} /></td><td><Observation source={road.currentCondition.source} time={road.currentCondition.observedAt} /></td></tr>)}</Table> : <PanelState result={result} feed="roads" empty={t("panel.noRoad")} />}
      </OverviewPanel>
      <OverviewPanel id="events-title" title={t("event.title")} icon="rain" to="/map" caption={t("event.caption")}>
        {events.length ? <ul className="overview-bulletins">{events.map((event) => <li className={`overview-bulletin overview-bulletin--${effectiveStatus(event.effectiveSeverity, event.freshness, event.verificationStatus).toLowerCase()}`} key={event.id}><article><h3>{event.title}</h3><p>{event.description}</p><div className="badge-row"><StatusBadge status={effectiveStatus(event.effectiveSeverity, event.freshness, event.verificationStatus)} /><StatusBadge status={effectiveStatus(event.effectiveStatus, event.freshness, event.verificationStatus)} /></div><Evidence source={event.source} freshness={event.freshness} /><Observation source={event.source} time={event.observedAt} /></article></li>)}</ul> : <PanelState result={result} feed="events" empty={t("panel.noEvent")} />}
      </OverviewPanel>
    </div><aside className="overview-sidebar" aria-label={`${t("heading.alerts")} ${t("heading.reports")}`}>
      <OverviewPanel id="alerts-title" title={t("alert.title")} icon="announcement" to="/announcements" caption={t("alert.caption")}>
        {alerts.length ? <ul className="overview-bulletins">{alerts.map((alert) => <li className={`overview-bulletin overview-bulletin--${alert.severity === "WATCH" ? "warning" : alert.severity.toLowerCase()}`} key={alert.id}><article><div className="badge-row"><StatusBadge status={alert.severity} /><FreshnessBadge freshness="UNKNOWN" /><SourceBadge type={alert.sourceType} /></div><h3>{alert.title}</h3><p>{alert.message}</p><div className="overview-cell-meta"><DataTimestamp value={alert.triggeredAt} label={t("label.alertStarted")} /></div></article></li>)}</ul> : <PanelState result={result} feed="alerts" empty={t("panel.noAlert")} />}
      </OverviewPanel>
      <OverviewPanel id="reports-title" title={t("report.title")} icon="people" to="/reports" caption={t("report.caption")}>
        {reports.length ? <ul className="overview-report-list">{reports.map((report) => <li key={report.id}><article><div className="overview-report-thumbnail">{report.media[0] ? <ReportMediaImage media={report.media[0]} /> : <span>{t("report.noImage")}</span>}</div><div className="overview-report-copy"><Link className="overview-report-link" to={`/reports?report=${encodeURIComponent(report.id)}`}><div><h3>{t(reportLabelKeys[report.type])}</h3><p>{report.description}</p></div></Link><div className="badge-row"><SourceBadge type="COMMUNITY" /><span className="badge">{t("report.verified")}</span></div><small><DataTimestamp value={report.createdAt} label={t("label.reportSubmitted")} /></small></div></article></li>)}</ul> : <PanelState result={result} feed="reports" empty={t("panel.noReport")} />}
        <p className="overview-footer-note">{t("report.footer")}</p>
      </OverviewPanel>
    </aside></div>
    <p className="overview-footer-note">{t("footer")}</p>
  </div>;
}
