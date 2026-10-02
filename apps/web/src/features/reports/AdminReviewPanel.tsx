import { useEffect, useState } from "react";
import { ApiError, reportsApi } from "./api";
import { ReportCard } from "./ReportCard";
import type { CommunityReport } from "./types";

export function AdminReviewPanel() {
  const [items, setItems] = useState<CommunityReport[]>([]);
  const [selected, setSelected] = useState<CommunityReport | null>(null);
  const [reason, setReason] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const page = await reportsApi.listPending();
      setItems(page.items);
    } catch {
      setError("ไม่สามารถโหลดรายงานที่รอตรวจสอบได้");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { void load(); }, []);

  const select = async (id: string) => {
    setError(null);
    try {
      setSelected(await reportsApi.getAdminReport(id));
      setReason("");
    } catch {
      setError("ไม่สามารถเปิดรายละเอียดรายงานได้");
    }
  };

  const review = async (action: "verify" | "reject") => {
    if (!selected) return;
    setSaving(true);
    setError(null);
    try {
      await reportsApi.review(selected.id, action, reason.trim(), selected.updatedAt);
      setItems((current) => current.filter((report) => report.id !== selected.id));
      setSelected(null);
      setReason("");
    } catch (cause) {
      setError(cause instanceof ApiError && cause.status === 409 ? "รายงานมีการแก้ไขหรือเพิ่มภาพแล้ว โปรดเปิดรายละเอียดใหม่ก่อนตรวจสอบ" : cause instanceof ApiError ? cause.message : "ไม่สามารถบันทึกผลการตรวจสอบได้");
      if (cause instanceof ApiError && cause.status === 409) setSelected(null);
    } finally {
      setSaving(false);
    }
  };

  return <section className="data-section admin-review" aria-labelledby="admin-review-heading">
    <h2 id="admin-review-heading">ตรวจสอบรายงานชุมชน</h2>
    <p className="section-caption">สำหรับผู้ดูแลระบบ: ตรวจสอบหลักฐานก่อนเผยแพร่หรือปฏิเสธ</p>
    {error && <p className="form-error" role="alert">{error}</p>}
    {loading ? <p role="status">กำลังโหลดคิวตรวจสอบ</p> : items.length === 0 ? <p className="section-empty">ไม่มีรายงานที่รอตรวจสอบ</p> : <ul className="data-list admin-review__list">{items.map((report) => <li key={report.id}><button type="button" onClick={() => void select(report.id)}>เปิดรายงาน {report.type}</button></li>)}</ul>}
    {selected && <div className="admin-review__detail"><ReportCard report={selected} /><label>เหตุผลที่ปฏิเสธ (ไม่บังคับ)<textarea value={reason} maxLength={500} onChange={(event) => setReason(event.target.value)} /></label><div className="admin-review__actions"><button type="button" className="action-button" disabled={saving} onClick={() => void review("verify")}>ยืนยันรายงาน</button><button type="button" className="action-button action-button--danger" disabled={saving} onClick={() => void review("reject")}>ปฏิเสธรายงาน</button></div></div>}
  </section>;
}
