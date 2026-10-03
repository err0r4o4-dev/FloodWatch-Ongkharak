import { useEffect, useRef, useState } from "react";
import { DataTimestamp, SourceBadge } from "../../shared/components/DataStates";
import { useOptionalAuth } from "../auth/AuthProvider";
import { useRealtimeRevision } from "../realtime/RealtimeProvider";
import { AdminReviewPanel } from "./AdminReviewPanel";
import { reportsApi } from "./api";
import { ReportCard } from "./ReportCard";
import { ReportForm } from "./ReportForm";
import { ReportMediaImage } from "./ReportMediaImage";
import "./ReportsPage.css";
import type { CommunityReport, ReportType } from "./types";

type LoadState = "loading" | "ready" | "error";
type ReportFilter = "ALL" | ReportType;

const reportEvents = ["report.verified"] as const;
const reportFilters: Array<{ value: ReportFilter; label: string }> = [
  { value: "ALL", label: "ทั้งหมด" },
  { value: "FLOOD", label: "น้ำท่วม" },
  { value: "ROAD", label: "สภาพถนน" },
  { value: "OTHER", label: "อื่น ๆ" }
];
const reportTypeLabels: Record<ReportType, string> = {
  FLOOD: "น้ำท่วม",
  ROAD: "สภาพถนน",
  OTHER: "อื่น ๆ"
};

function coordinateLabel(report: CommunityReport) {
  const [longitude, latitude] = report.location.coordinates;
  return `${latitude.toFixed(5)}, ${longitude.toFixed(5)}`;
}

function ReportDetails({ report }: { report: CommunityReport }) {
  return <aside className="reports-detail" aria-labelledby="report-detail-heading">
    <div className="reports-detail__heading">
      <div>
        <p className="eyebrow">รายงานที่เลือก</p>
        <h2 id="report-detail-heading">{report.description}</h2>
      </div>
      <span className="badge">{reportTypeLabels[report.type]}</span>
    </div>
    <div className="badge-row">
      <SourceBadge type="COMMUNITY" />
      <span className="badge badge--report-verified">ยืนยันแล้ว</span>
    </div>
    <p className="reports-detail__description">{report.description}</p>
    {report.media.length > 0 && <section className="reports-gallery" aria-labelledby="report-gallery-heading">
      <h3 id="report-gallery-heading">ภาพประกอบ</h3>
      <ul className="report-media" aria-label="ภาพประกอบรายงานที่เลือก">
        {report.media.map((media) => <li key={media.id}><ReportMediaImage media={media} /></li>)}
      </ul>
    </section>}
    <dl className="reports-detail__facts">
      <div><dt>ตำแหน่ง</dt><dd>{coordinateLabel(report)}</dd></div>
      <div><dt>เวลาที่ส่ง</dt><dd><DataTimestamp value={report.createdAt} label="ส่งรายงาน" /></dd></div>
      {report.verifiedAt && <div><dt>ตรวจสอบเมื่อ</dt><dd><DataTimestamp value={report.verifiedAt} label="ยืนยัน" /></dd></div>}
      {report.estimatedWaterDepthM !== null && <div><dt>ระดับน้ำโดยประมาณ</dt><dd>{report.estimatedWaterDepthM.toFixed(2)} ม.</dd></div>}
    </dl>
  </aside>;
}

function PublicReportListState({ state, onRetry }: { state: LoadState; onRetry: () => void }) {
  if (state === "loading") return <div className="reports-state-panel" role="status"><h3>กำลังโหลดข้อมูล</h3><p>กำลังตรวจสอบรายงานชุมชนที่ยืนยันแล้ว</p></div>;
  if (state === "error") return <div className="reports-state-panel" role="alert"><h3>ไม่สามารถโหลดรายงานได้</h3><p>ข้อมูลที่ไม่พร้อมใช้งานไม่ได้หมายถึงปลอดภัย</p><button type="button" className="action-button" onClick={onRetry}>โหลดรายงานอีกครั้ง</button></div>;
  return <div className="reports-state-panel"><h3>ยังไม่มีรายงานที่เผยแพร่</h3><p>ยังไม่มีรายงานชุมชนที่ผ่านการตรวจสอบสำหรับแสดงต่อสาธารณะ</p></div>;
}

function ReportDetailsState({ state }: { state: LoadState }) {
  const message = state === "loading"
    ? "กำลังรอข้อมูลรายงานเพื่อแสดงรายละเอียด"
    : state === "error"
      ? "ไม่สามารถแสดงรายละเอียดได้จนกว่าจะโหลดรายงานสำเร็จ"
      : "จะมีรายละเอียดให้เลือกเมื่อมีรายงานชุมชนที่ผ่านการตรวจสอบ";
  return <aside className="reports-detail reports-detail--state" aria-labelledby="report-detail-heading">
    <p className="eyebrow">รายละเอียดรายงาน</p>
    <h2 id="report-detail-heading">{state === "error" ? "รายละเอียดไม่พร้อมใช้งาน" : "ยังไม่มีรายละเอียดรายงาน"}</h2>
    <p>{message}</p>
  </aside>;
}

export function ReportsPage() {
  const { account, status } = useOptionalAuth();
  const realtimeRevision = useRealtimeRevision(reportEvents);
  const [publicReports, setPublicReports] = useState<CommunityReport[]>([]);
  const [publicState, setPublicState] = useState<LoadState>("loading");
  const [filter, setFilter] = useState<ReportFilter>("ALL");
  const [query, setQuery] = useState("");
  const [selectedReportId, setSelectedReportId] = useState<string | null>(null);
  const publicRequest = useRef(0);
  const [mine, setMine] = useState<CommunityReport[]>([]);
  const [mineOwnerId, setMineOwnerId] = useState<string | null>(null);
  const mineRequest = useRef(0);
  const [mineState, setMineState] = useState<LoadState>("loading");
  const [editing, setEditing] = useState<CommunityReport | null>(null);

  const loadPublic = async () => {
    const requestId = ++publicRequest.current;
    setPublicState("loading");
    try {
      const page = await reportsApi.listPublic();
      if (requestId !== publicRequest.current) return;
      setPublicReports(page.items.filter((report) => report.status === "VERIFIED" && report.verificationStatus === "VERIFIED" && report.sourceType === "COMMUNITY"));
      setPublicState("ready");
    } catch {
      if (requestId === publicRequest.current) setPublicState("error");
    }
  };

  const loadMine = async () => {
    if (!account) return;
    const requestId = ++mineRequest.current;
    setMineState("loading");
    try {
      const page = await reportsApi.mine();
      if (requestId !== mineRequest.current) return;
      setMine(page.items);
      setMineOwnerId(account.id);
      setMineState("ready");
    } catch {
      if (requestId === mineRequest.current) setMineState("error");
    }
  };

  useEffect(() => {
    void loadPublic();
    return () => { publicRequest.current += 1; };
  }, [realtimeRevision]);

  useEffect(() => {
    if (status === "authenticated") void loadMine();
    return () => { mineRequest.current += 1; };
  }, [status, account?.id]);

  const completeForm = (preserveEditing = false) => {
    if (!preserveEditing) setEditing(null);
    void loadMine();
  };
  const normalizedQuery = query.trim().toLocaleLowerCase("th-TH");
  const visibleReports = publicReports.filter((report) =>
    (filter === "ALL" || report.type === filter) &&
    (!normalizedQuery || report.description.toLocaleLowerCase("th-TH").includes(normalizedQuery))
  );
  const selectedReport = visibleReports.find((report) => report.id === selectedReportId) ?? visibleReports[0] ?? null;

  return <section className="feature-page reports-page">
    <header className="page-hero reports-page__heading">
      <div className="page-hero__copy">
        <p className="eyebrow">COMMUNITY EVIDENCE</p>
        <h1>รายงานสถานการณ์</h1>
        <p>รายงานจากชุมชนจะแสดงต่อสาธารณะเมื่อผ่านการตรวจสอบแล้วเท่านั้น</p>
      </div>
    </header>
    <section className="data-section reports-public" aria-labelledby="public-reports-heading">
      <div className="reports-public__heading">
        <div><h2 id="public-reports-heading">รายงานที่ยืนยันแล้ว</h2><p className="section-caption">แหล่งข้อมูล: รายงานชุมชนที่ตรวจสอบแล้ว</p></div>
        <label className="reports-search">ค้นหารายงาน<input type="search" value={query} onChange={(event) => setQuery(event.target.value)} placeholder="ค้นหาจากรายละเอียดรายงาน" /></label>
      </div>
      <div className="reports-filters" aria-label="กรองประเภทรายงาน">
        {reportFilters.map((item) => <button key={item.value} type="button" aria-pressed={filter === item.value} onClick={() => setFilter(item.value)}>{item.label}</button>)}
      </div>
      <div className="reports-browser">
        {publicState === "ready" && publicReports.length > 0
          ? <ul className="data-list reports-list">{visibleReports.map((report) => <li key={report.id}><ReportCard report={report} showDetailsControl onShowDetails={setSelectedReportId} /></li>)}</ul>
          : <div className="reports-list reports-list--state"><PublicReportListState state={publicState} onRetry={() => void loadPublic()} /></div>}
        {publicState !== "ready" || publicReports.length === 0
          ? <ReportDetailsState state={publicState} />
          : visibleReports.length === 0
            ? <aside className="reports-detail reports-detail--state" aria-labelledby="report-detail-heading"><h2 id="report-detail-heading" className="section-empty reports-filter-empty">ไม่พบรายงานที่ตรงกับตัวกรอง</h2></aside>
            : selectedReport && <ReportDetails report={selectedReport} />}
      </div>
    </section>
    {status === "authenticated" && <div className="reports-authenticated"><ReportForm editing={editing} onComplete={completeForm} onCancel={() => setEditing(null)} />
      <section className="data-section my-reports" aria-labelledby="my-reports-heading"><h2 id="my-reports-heading">รายงานของฉัน</h2><p className="section-caption">รายงานที่รอตรวจสอบยังแก้ไขได้ รายงานที่ยืนยันหรือปฏิเสธแล้วจะคงเดิม</p>
        {(mineState === "loading" || mineOwnerId !== account?.id && mineState !== "error") && <p role="status">กำลังโหลดรายงานของฉัน</p>}
        {mineState === "error" && <div role="alert"><p>ไม่สามารถโหลดรายงานของฉันได้</p><button type="button" className="action-button" onClick={() => void loadMine()}>โหลดอีกครั้ง</button></div>}
        {mineState === "ready" && mineOwnerId === account?.id && mine.length === 0 && <p className="section-empty">คุณยังไม่ได้ส่งรายงาน</p>}
        {mineState === "ready" && mineOwnerId === account?.id && mine.length > 0 && <ul className="data-list">{mine.map((report) => <li key={report.id}><ReportCard report={report} showOwnerControls onEdit={setEditing} /></li>)}</ul>}
      </section>
      {account?.role === "ADMIN" && <AdminReviewPanel />}
    </div>}
  </section>;
}
