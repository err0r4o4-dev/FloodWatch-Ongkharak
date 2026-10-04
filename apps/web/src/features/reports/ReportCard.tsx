import { formatNumber, useTranslation } from "../../i18n";
import th from "../../i18n/locales/th/reports";
import en from "../../i18n/locales/en/reports";
import { DataTimestamp, OriginalContentNote, SourceBadge } from "../../shared/components/DataStates";
import { ReportMediaImage } from "./ReportMediaImage";
import type { CommunityReport, ReportType } from "./types";

const typeKeys: Record<ReportType, keyof typeof th> = {
  FLOOD: "typeLabel.FLOOD",
  ROAD: "typeLabel.ROAD",
  OTHER: "typeLabel.OTHER",
};
const statusKeys: Record<CommunityReport["status"], keyof typeof th> = {
  PENDING: "status.pending",
  VERIFIED: "status.verified",
  REJECTED: "status.rejected",
};

export function ReportStatusBadge({ status }: { status: CommunityReport["status"] }) {
  const { t } = useTranslation({ th, en });
  return <span className={`badge badge--report-${status.toLowerCase()}`}>{t(statusKeys[status])}</span>;
}

export function ReportCard({ report, showOwnerControls = false, onEdit, showDetailsControl = false, isSelected, onShowDetails }: {
  report: CommunityReport;
  showOwnerControls?: boolean;
  onEdit?: (report: CommunityReport) => void;
  showDetailsControl?: boolean;
  isSelected?: boolean;
  onShowDetails?: (reportId: string) => void;
}) {
  const { locale, t } = useTranslation({ th, en });
  const typeLabel = t(typeKeys[report.type]);
  return <article className={`data-card report-card${report.media.length > 0 ? " report-card--with-media" : ""}${isSelected ? " report-card--selected" : ""}`}>
    <div className="data-card__heading"><h3>{typeLabel}</h3><ReportStatusBadge status={report.status} /></div>
    <OriginalContentNote />
    <p>{report.description}</p>
    <p className="data-card__meta">{t("detail.reportedBy")}: {report.displayName || t("detail.reporterNotProvided")}</p>
    <div className="badge-row"><SourceBadge type="COMMUNITY" /><span className="badge">{t("detail.location")}: {report.locationName || t("detail.locationUnknown")}</span></div>
    {report.locationNote && <p className="data-card__meta">{t("detail.locationNote")}: {report.locationNote}</p>}
    {report.estimatedWaterDepthM !== null && <p className="reading-values">{t("detail.depth")} {formatNumber(report.estimatedWaterDepthM, locale, { minimumFractionDigits: 2, maximumFractionDigits: 2 })} {t("detail.meters")}</p>}
    <p className="data-card__meta"><DataTimestamp value={report.createdAt} label={t("detail.submittedLabel")} /></p>
    <p className="data-card__meta">{report.observedAt ? <DataTimestamp value={report.observedAt} label={t("detail.observed")} /> : t("detail.observedUnknown")}</p>
    {report.waterTrend && report.waterTrend !== "UNKNOWN" && <p className="data-card__meta">{t("detail.waterTrend")}: {t(`trend.${report.waterTrend}`)}</p>}
    {report.media.length > 0 && <ul className="report-media" aria-label={t("detail.mediaLabel")}>{(showDetailsControl ? report.media.slice(0, 1) : report.media).map((media) => <li key={media.id}><ReportMediaImage media={media} /></li>)}</ul>}
    {showDetailsControl && onShowDetails && <button type="button" className="report-card__detail-button" aria-pressed={isSelected} aria-label={t("reportDetails", { type: typeLabel, description: report.description })} onClick={() => onShowDetails(report.id)}>{t("detail.view")}<span aria-hidden="true"> →</span></button>}
    {showOwnerControls && report.status === "PENDING" && onEdit && <button type="button" className="action-button" onClick={() => onEdit(report)}>{t("editPending")}</button>}
  </article>;
}
