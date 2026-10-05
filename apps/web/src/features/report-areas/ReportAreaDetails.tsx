import { useState } from "react";
import { Link } from "react-router";
import { DataTimestamp, ErrorState, LoadingState } from "../../shared/components/DataStates";
import { formatNumber, useTranslation } from "../../i18n";
import th from "../../i18n/locales/th/map";
import en from "../../i18n/locales/en/map";
import { useRealtimeRevision } from "../realtime/RealtimeProvider";
import { findTimelineGaps } from "./timeline";
import { useReportAreaDetail, useReportAreaReports } from "./hooks";
import { useReportAreaUpdatedRevision } from "./realtime";
import { ReportAreaDepthBadge, ReportAreaSummary } from "./ReportAreaSummary";
import { displayReportAreaTrend, maskReportArea } from "./presentation";
import { ReportCard } from "../reports/ReportCard";
import { isRoadSegmentReportArea } from "./types";
import type { ReportArea } from "./types";

const reportEvents = ["report.verified", "report.updated"] as const;

function updateReportHref(area: ReportArea): string {
  const query = new URLSearchParams({ areaId: area.id, lng: String(area.location.coordinates[0]), lat: String(area.location.coordinates[1]) });
  if (area.name) query.set("locationName", area.name);
  if (isRoadSegmentReportArea(area)) query.set("roadSegmentId", area.roadSegmentId);
  return `/reports/new?${query.toString()}`;
}

export function ReportAreaDetailsContent({ area }: { area: ReportArea }) {
  const { locale, t } = useTranslation({ th, en });
  const areaRevision = useReportAreaUpdatedRevision(area.id);
  const reportRevision = useRealtimeRevision(reportEvents);
  const [retry, setRetry] = useState(0);
  const detail = useReportAreaDetail(area.id, areaRevision + reportRevision + retry);
  const reports = useReportAreaReports(area.id, areaRevision + reportRevision + retry);
  const currentArea = detail.state === "ready" && detail.area ? detail.area : maskReportArea(area);
  const gaps = findTimelineGaps(detail.timeline);
  return <div className="report-area-detail">
    <h3 className="report-area-detail__name">{currentArea.name || t("details.communityArea")}</h3>
    <ReportAreaSummary area={currentArea} />
    <Link className="report-area-detail__update" to={updateReportHref(currentArea)}>{t("area.update")}</Link>
    <section aria-labelledby="report-area-timeline-title" className="report-area-detail__section">
      <h4 id="report-area-timeline-title">{currentArea.areaType === "ROAD_SEGMENT" ? t("area.roadTimeline") : t("area.timeline")}</h4>
      {detail.state === "loading" && <p role="status">{t("area.timelineLoading")}</p>}
      {detail.state === "error" && <div role="alert"><p>{t("area.timelineError")}</p><button type="button" className="action-button" onClick={() => setRetry((current) => current + 1)}>{t("mapStatus.retry")}</button></div>}
      {detail.state === "ready" && <>
        {gaps.length > 0 && <p className="report-area-detail__gap" role="status">{t("area.timelineGaps", { count: gaps.length })}</p>}
        {detail.timeline.length === 0 ? <p>{t("area.timelineEmpty")}</p> : <ol className="report-area-timeline">{detail.timeline.map((window) => <li key={`${window.windowStart}-${window.windowEnd}`}>
          <div className="report-area-timeline__heading"><strong><DataTimestamp value={window.windowStart} /> → <DataTimestamp value={window.windowEnd} /></strong><ReportAreaDepthBadge depthBand={window.depthBand} freshness={window.freshness} latestObservedAt={window.latestObservedAt} now={Date.parse(window.windowEnd)} /></div>
          <p>{t(`area.trend.${displayReportAreaTrend(window.trend, window.freshness, window.latestObservedAt, Date.parse(window.windowEnd))}`)} · {t(`area.reason.${window.summaryReason}`)} · {formatNumber(window.reportCount, locale)} {t("area.reportsUnit")}</p>
        </li>)}</ol>}
      </>}
    </section>
    <section aria-labelledby="report-area-original-title" className="report-area-detail__section">
      <h4 id="report-area-original-title">{t("area.originalReports")}</h4>
      {reports.state === "loading" && <LoadingState />}
      {reports.state === "error" && <ErrorState onRetry={reports.loadMore} />}
      {reports.state === "ready" && reports.items.length === 0 && <p>{t("area.originalReportsEmpty")}</p>}
      {reports.items.length > 0 && <ul className="report-area-original-reports">{reports.items.map((report) => <li key={report.id}><ReportCard report={report} /></li>)}</ul>}
      {reports.state === "ready" && reports.hasMore && <button type="button" className="action-button report-area-detail__load-more" onClick={reports.loadMore}>{t("area.loadMore")}</button>}
    </section>
  </div>;
}
