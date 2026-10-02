import { DataTimestamp, SourceBadge } from "../../shared/components/DataStates";
import { ReportMediaImage } from "./ReportMediaImage";
import type { CommunityReport } from "./types";

const typeLabels = { FLOOD: "น้ำท่วม", ROAD: "สภาพถนน", OTHER: "อื่น ๆ" };
const statusLabels = { PENDING: "รอตรวจสอบ", VERIFIED: "ยืนยันแล้ว", REJECTED: "ไม่ผ่านการตรวจสอบ" };

export function ReportStatusBadge({ status }: { status: CommunityReport["status"] }) {
  return <span className={`badge badge--report-${status.toLowerCase()}`}>{statusLabels[status]}</span>;
}

export function ReportCard({ report, showOwnerControls = false, onEdit, showDetailsControl = false, onShowDetails }: {
  report: CommunityReport;
  showOwnerControls?: boolean;
  onEdit?: (report: CommunityReport) => void;
  showDetailsControl?: boolean;
  onShowDetails?: (reportId: string) => void;
}) {
  return <article className={`data-card report-card${report.media.length > 0 ? " report-card--with-media" : ""}`}>
    <div className="data-card__heading"><h3>{typeLabels[report.type]}</h3><ReportStatusBadge status={report.status} /></div>
    <p>{report.description}</p>
    <div className="badge-row"><SourceBadge type="COMMUNITY" /><span className="badge">พิกัด {report.location.coordinates[1].toFixed(5)}, {report.location.coordinates[0].toFixed(5)}</span></div>
    {report.estimatedWaterDepthM !== null && <p className="reading-values">ระดับน้ำโดยประมาณ {report.estimatedWaterDepthM.toFixed(2)} ม.</p>}
    <p className="data-card__meta"><DataTimestamp value={report.createdAt} label="ส่งรายงาน" /></p>
    {report.media.length > 0 && <ul className="report-media" aria-label="ภาพประกอบ">{(showDetailsControl ? report.media.slice(0, 1) : report.media).map((media) => <li key={media.id}><ReportMediaImage media={media} /></li>)}</ul>}
    {showDetailsControl && onShowDetails && <button type="button" className="report-card__detail-button" aria-label={`ดูรายละเอียด: ${typeLabels[report.type]} · ${report.description}`} onClick={() => onShowDetails(report.id)}>ดูรายละเอียด: {typeLabels[report.type]}</button>}
    {showOwnerControls && report.status === "PENDING" && onEdit && <button type="button" className="action-button" onClick={() => onEdit(report)}>แก้ไขรายงานที่รอตรวจสอบ</button>}
  </article>;
}
