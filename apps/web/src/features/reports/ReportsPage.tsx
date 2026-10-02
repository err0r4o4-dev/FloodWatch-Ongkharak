import { useEffect, useRef, useState } from "react";
import { Link } from "react-router";
import { useOptionalAuth } from "../auth/AuthProvider";
import { AdminReviewPanel } from "./AdminReviewPanel";
import { reportsApi } from "./api";
import { ReportCard } from "./ReportCard";
import { ReportForm } from "./ReportForm";
import type { CommunityReport } from "./types";

type LoadState = "loading" | "ready" | "error";

export function ReportsPage() {
  const { account, status } = useOptionalAuth();
  const [publicReports, setPublicReports] = useState<CommunityReport[]>([]);
  const [publicState, setPublicState] = useState<LoadState>("loading");
  const [mine, setMine] = useState<CommunityReport[]>([]);
  const [mineOwnerId, setMineOwnerId] = useState<string | null>(null);
  const mineRequest = useRef(0);
  const [mineState, setMineState] = useState<LoadState>("loading");
  const [editing, setEditing] = useState<CommunityReport | null>(null);

  const loadPublic = async () => {
    setPublicState("loading");
    try {
      const page = await reportsApi.listPublic();
      setPublicReports(page.items.filter((report) => report.status === "VERIFIED" && report.verificationStatus === "VERIFIED" && report.sourceType === "COMMUNITY"));
      setPublicState("ready");
    } catch {
      setPublicState("error");
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

  useEffect(() => { void loadPublic(); }, []);
  useEffect(() => {
    if (status === "authenticated") void loadMine();
    return () => { mineRequest.current += 1; };
  }, [status, account?.id]);

  const completeForm = (preserveEditing = false) => {
    if (!preserveEditing) setEditing(null);
    void loadMine();
  };

  return <section className="feature-page reports-page"><div className="page-heading"><p className="eyebrow">COMMUNITY EVIDENCE</p><h1>รายงานสถานการณ์</h1><p>รายงานจากชุมชนจะแสดงต่อสาธารณะเมื่อผ่านการตรวจสอบแล้วเท่านั้น</p></div>
    {status !== "authenticated" && <aside className="notice"><span>มีข้อมูลพื้นที่ที่ต้องการแจ้งหรือไม่? เข้าสู่ระบบเพื่อส่งรายงานพร้อมตำแหน่งและหลักฐาน</span><Link className="action-button" to="/login?returnTo=%2Freports">เข้าสู่ระบบเพื่อส่งรายงาน</Link><Link className="action-button" to="/register?returnTo=%2Freports">สมัครสมาชิก</Link></aside>}
    <section className="data-section reports-public" aria-labelledby="public-reports-heading"><h2 id="public-reports-heading">รายงานที่ยืนยันแล้ว</h2><p className="section-caption">แหล่งข้อมูล: รายงานชุมชนที่ตรวจสอบแล้ว</p>
      {publicState === "loading" && <p role="status">กำลังโหลดข้อมูล</p>}
      {publicState === "error" && <div role="alert"><p>ไม่สามารถโหลดรายงานได้ ข้อมูลที่ไม่พร้อมใช้งานไม่ได้หมายถึงปลอดภัย</p><button type="button" className="action-button" onClick={() => void loadPublic()}>โหลดรายงานอีกครั้ง</button></div>}
      {publicState === "ready" && publicReports.length === 0 && <p className="section-empty">ยังไม่มีรายงานที่เผยแพร่</p>}
      {publicState === "ready" && publicReports.length > 0 && <ul className="data-list">{publicReports.map((report) => <li key={report.id}><ReportCard report={report} /></li>)}</ul>}
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
