import { DataTimestamp, FreshnessBadge, StatusBadge } from "../../shared/components/DataStates";
import { formatNumber, useTranslation } from "../../i18n";
import th from "../../i18n/locales/th/map";
import en from "../../i18n/locales/en/map";
import { areaTrendSymbol, areaTrendTone, displayDepthBand } from "./presentation";
import type { ReportArea } from "./types";
import "./report-areas.css";

export function ReportAreaSummary({ area, compact = false }: { area: ReportArea; compact?: boolean }) {
  const { locale, t } = useTranslation({ th, en });
  const depthBand = displayDepthBand(area.depthBand, area.freshness);
  return <div className={`report-area-summary${compact ? " report-area-summary--compact" : ""}`}>
    <div className="badge-row"><StatusBadge status={depthBand} /><FreshnessBadge freshness={area.freshness} /></div>
    <dl className="report-area-summary__metrics">
      <div><dt>{t("area.trend")}</dt><dd><span className={`report-area-trend report-area-trend--${areaTrendTone(area.trend)}`} aria-hidden="true">{areaTrendSymbol(area.trend)}</span>{t(`area.trend.${area.trend}`)}</dd></div>
      <div><dt>{t("area.reportedTrend")}</dt><dd>{t(`area.trend.${area.reportedTrend}`)}</dd></div>
      <div><dt>{t("area.reason")}</dt><dd>{t(`area.reason.${area.summaryReason}`)}</dd></div>
      <div><dt>{t("area.reports")}</dt><dd>{formatNumber(area.reportCount, locale)} · {formatNumber(area.sampleCount, locale)} {t("area.samples")}</dd></div>
      <div><dt>{t("area.totalReports")}</dt><dd>{formatNumber(area.totalReportCount, locale)}</dd></div>
    </dl>
    <p className="report-area-summary__window"><DataTimestamp label={t("area.latestObserved")} value={area.latestObservedAt} /> · {t("area.computedAt")} <DataTimestamp value={area.computedAt} /></p>
    {depthBand === "UNKNOWN" && <p className="uncertainty">{t("area.unknownWarning")}</p>}
    <p className="section-caption">{t("legend.areaTrend")}</p>
  </div>;
}
