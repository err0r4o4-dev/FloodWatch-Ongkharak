import { DataTimestamp, FreshnessBadge, SourceBadge } from "../../shared/components/DataStates";
import { formatNumber, useTranslation } from "../../i18n";
import th from "../../i18n/locales/th/map";
import en from "../../i18n/locales/en/map";
import { areaTrendSymbol, areaTrendTone, displayDepthBand, displayReportAreaTrend, effectiveReportAreaFreshness } from "./presentation";
import type { ReportArea, ReportAreaDepthBand } from "./types";
import "./report-areas.css";

const depthRangeKeys: Record<ReportAreaDepthBand, keyof typeof th> = {
  UNKNOWN: "area.depthRange.UNKNOWN",
  LOW: "area.depthRange.LOW",
  MODERATE: "area.depthRange.MODERATE",
  HIGH: "area.depthRange.HIGH",
  VERY_HIGH: "area.depthRange.VERY_HIGH"
};

export function ReportAreaDepthBadge({ depthBand, freshness, latestObservedAt, now }: { depthBand: ReportAreaDepthBand; freshness: ReportArea["freshness"]; latestObservedAt: string | null; now?: number }) {
  const { t } = useTranslation({ th, en });
  const displayedBand = displayDepthBand(depthBand, freshness, latestObservedAt, now);
  return <span className={`report-area-depth-badge report-area-depth-badge--${displayedBand.toLowerCase()}`}>{t(depthRangeKeys[displayedBand])}</span>;
}

export function ReportAreaSummary({ area, compact = false }: { area: ReportArea; compact?: boolean }) {
  const { locale, t } = useTranslation({ th, en });
  const freshness = effectiveReportAreaFreshness(area.freshness, area.latestObservedAt);
  const depthBand = displayDepthBand(area.depthBand, freshness, area.latestObservedAt);
  const trend = displayReportAreaTrend(area.trend, area.freshness, area.latestObservedAt);
  const reportedTrend = displayReportAreaTrend(area.reportedTrend, area.freshness, area.latestObservedAt);
  return <div className={`report-area-summary${compact ? " report-area-summary--compact" : ""}`}>
    <h4 className="report-area-summary__heading">{t("area.communityDerived")}</h4>
    <div className="badge-row"><ReportAreaDepthBadge depthBand={area.depthBand} freshness={freshness} latestObservedAt={area.latestObservedAt} /><FreshnessBadge freshness={freshness} /><SourceBadge type="COMMUNITY" /></div>
    <dl className="report-area-summary__metrics">
      <div><dt>{t("area.currentDepthRange")}</dt><dd>{t(depthRangeKeys[depthBand])}</dd></div>
      <div><dt>{t("area.previousDepthRange")}</dt><dd>{t(depthRangeKeys[displayDepthBand(area.previousDepthBand, freshness, area.latestObservedAt)])}</dd></div>
      <div><dt>{t("area.trend")}</dt><dd><span className={`report-area-trend report-area-trend--${areaTrendTone(trend)}`} aria-hidden="true">{areaTrendSymbol(trend)}</span>{t(`area.trend.${trend}`)}</dd></div>
      <div><dt>{t("area.reportedTrend")}</dt><dd>{t(`area.trend.${reportedTrend}`)}</dd></div>
      <div><dt>{t("area.reason")}</dt><dd>{t(`area.reason.${area.summaryReason}`)}</dd></div>
      <div><dt>{t("area.reports")}</dt><dd>{formatNumber(area.reportCount, locale)} · {formatNumber(area.sampleCount, locale)} {t("area.samples")}</dd></div>
      <div><dt>{t("area.totalReports")}</dt><dd>{formatNumber(area.totalReportCount, locale)}</dd></div>
    </dl>
    <p className="report-area-summary__window"><strong>{t("area.window")}</strong> <DataTimestamp value={area.windowStart} /> → <DataTimestamp value={area.windowEnd} /></p>
    <p className="report-area-summary__window"><DataTimestamp label={t("area.latestObserved")} value={area.latestObservedAt} /> · {t("area.computedAt")} <DataTimestamp value={area.computedAt} /></p>
    {depthBand === "UNKNOWN" && <p className="uncertainty">{t("area.unknownWarning")}</p>}
    <p className="section-caption">{t("legend.areaTrend")}</p>
  </div>;
}
