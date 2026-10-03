import { useEffect, useRef, useState } from "react";
import { Link } from "react-router";
import { ApiError } from "../auth/api";
import { confirmAction } from "../../shared/alerts";
import { DataTimestamp, SourceBadge, StatusBadge } from "../../shared/components/DataStates";
import { AI_EVIDENCE_PAGE_SIZE, aiApi } from "./ai-api";
import type { AIEvidence, AIEvidencePage, RoadOption } from "./ai-api";

type LoadState = "loading" | "ready" | "error";

const evidenceStatusLabels: Record<AIEvidence["status"], string> = {
  UNVERIFIED: "รอตรวจสอบ",
  VERIFIED: "ยืนยันแล้ว",
  REJECTED: "ปฏิเสธแล้ว"
};

function confidenceLabel(value: number) {
  return `${Math.round(value * 100)}%`;
}

function depthLabel(value: number | null) {
  return value === null ? "ไม่ระบุ" : `${value.toFixed(2)} ม.`;
}

function evidenceStatus(status: AIEvidence["status"]) {
  return <span className="badge badge--status-unknown">{evidenceStatusLabels[status]}</span>;
}

export function AIEvidenceReview() {
  const [page, setPage] = useState<AIEvidencePage | null>(null);
  const [listState, setListState] = useState<LoadState>("loading");
  const [listError, setListError] = useState(false);
  const [offset, setOffset] = useState(0);
  const [retry, setRetry] = useState(0);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [detail, setDetail] = useState<AIEvidence | null>(null);
  const [detailState, setDetailState] = useState<LoadState>("ready");
  const [roadOptions, setRoadOptions] = useState<RoadOption[]>([]);
  const [roadState, setRoadState] = useState<LoadState>("ready");
  const [roadId, setRoadId] = useState("");
  const [rejectionReasons, setRejectionReasons] = useState<Record<string, string>>({});
  const [validationError, setValidationError] = useState<string | null>(null);
  const [conflictId, setConflictId] = useState<string | null>(null);
  const [reviewingId, setReviewingId] = useState<string | null>(null);
  const listRequest = useRef(0);
  const detailRequest = useRef(0);
  const reviewPending = useRef(false);

  useEffect(() => {
    const requestId = ++listRequest.current;
    const controller = new AbortController();
    setListState("loading");
    setListError(false);

    void aiApi.list({ limit: AI_EVIDENCE_PAGE_SIZE, offset, signal: controller.signal })
      .then((nextPage) => {
        if (requestId !== listRequest.current) return;
        setPage(nextPage);
        setListState("ready");
      })
      .catch(() => {
        if (requestId !== listRequest.current) return;
        setListError(true);
        setListState("error");
      });

    return () => {
      controller.abort();
    };
  }, [offset, retry]);

  const loadRoadOptions = async () => {
    setRoadState("loading");
    try {
      setRoadOptions(await aiApi.listRoads({ limit: 20, offset: 0 }));
      setRoadState("ready");
    } catch {
      setRoadState("error");
    }
  };

  const openDetails = async (id: string) => {
    const requestId = ++detailRequest.current;
    setSelectedId(id);
    setDetail(null);
    setDetailState("loading");
    setRoadOptions([]);
    setRoadState("ready");
    setRoadId("");
    setValidationError(null);
    setConflictId(null);
    try {
      const nextDetail = await aiApi.get(id);
      if (requestId !== detailRequest.current) return;
      setDetail(nextDetail);
      setDetailState("ready");
      if (nextDetail.road) void loadRoadOptions();
    } catch {
      if (requestId === detailRequest.current) setDetailState("error");
    }
  };

  const review = async (action: "verify" | "reject") => {
    if (!detail || detail.status !== "UNVERIFIED" || reviewPending.current) return;
    const reason = rejectionReasons[detail.id]?.trim() ?? "";
    if (action === "reject" && !reason) {
      setValidationError("กรุณาระบุเหตุผลก่อนปฏิเสธหลักฐาน AI");
      return;
    }
    if (action === "verify" && detail.road && (!roadId || roadState !== "ready")) {
      setValidationError("กรุณาเลือกถนนจริงจากรายการก่อนอนุมัติผลถนน");
      return;
    }

    reviewPending.current = true;
    setReviewingId(detail.id);
    setValidationError(null);
    setConflictId(null);
    try {
      const confirmed = await confirmAction({
        title: action === "verify" ? "อนุมัติหลักฐาน AI หรือไม่?" : "ปฏิเสธหลักฐาน AI หรือไม่?",
        text: action === "verify" ? "ตรวจสอบผล AI และแหล่งข้อมูลก่อนอนุมัติ ผลที่อนุมัติจะยังคงระบุว่าเป็น AI" : "โปรดระบุเหตุผลที่เพียงพอสำหรับการปฏิเสธหลักฐาน AI",
        confirmText: action === "verify" ? "อนุมัติหลักฐาน" : "ปฏิเสธหลักฐาน",
        danger: action === "reject",
        action: async () => {
          try {
            if (action === "verify") {
              await aiApi.verify(detail.id, detail.road ? { roadId } : {});
            } else {
              await aiApi.reject(detail.id, { reason });
            }
          } catch (cause) {
            if (cause instanceof ApiError && cause.status === 409) setConflictId(detail.id);
            throw cause;
          }
        },
        successTitle: action === "verify" ? "อนุมัติหลักฐานแล้ว" : "ปฏิเสธหลักฐานแล้ว",
        errorText: "ไม่สามารถบันทึกผลการตรวจสอบหลักฐาน AI ได้ กรุณาลองใหม่อีกครั้ง"
      });
      if (confirmed) {
        setSelectedId(null);
        setDetail(null);
        setRetry((current) => current + 1);
      }
    } finally {
      reviewPending.current = false;
      setReviewingId(null);
    }
  };

  const renderDetail = () => {
    if (!selectedId) return null;
    if (detailState === "loading") return <p className="panel" role="status">กำลังโหลดรายละเอียดหลักฐาน AI…</p>;
    if (detailState === "error" || !detail) return <div className="panel error-state" role="alert"><p>ไม่สามารถโหลดรายละเอียดหลักฐาน AI ได้</p><button type="button" className="action-button" onClick={() => void openDetails(selectedId)}>โหลดรายละเอียดอีกครั้ง</button></div>;

    const canReview = detail.status === "UNVERIFIED";
    const selectedReason = rejectionReasons[detail.id] ?? "";
    return <section className="panel space-y-4" aria-labelledby="ai-evidence-detail-heading" aria-busy={reviewingId === detail.id}>
      <div className="flex items-start justify-between gap-4">
        <div>
          <p className="eyebrow">AI EVIDENCE DETAIL</p>
          <h2 id="ai-evidence-detail-heading">รายละเอียดหลักฐาน</h2>
        </div>
        {evidenceStatus(detail.status)}
      </div>
      <div className="badge-row"><SourceBadge type="AI" /><span className="badge">รุ่นโมเดล: {detail.modelVersion}</span></div>
      <dl className="grid gap-2 sm:grid-cols-2">
        <div><dt>เวลาที่โมเดลสังเกต</dt><dd><DataTimestamp value={detail.observedAt} label="สังเกต" /></dd></div>
        <div><dt>เวลาที่ระบบรับหลักฐาน</dt><dd><DataTimestamp value={detail.receivedAt} label="รับข้อมูล" /></dd></div>
      </dl>
      <p><Link to={`/reports?report=${encodeURIComponent(detail.reportId)}`}>ดูรายงานที่เกี่ยวข้อง</Link></p>

      <div className="grid gap-4 md:grid-cols-2">
        <section aria-labelledby="ai-flood-result-heading" className="panel">
          <h3 id="ai-flood-result-heading">ผลการประเมินน้ำท่วม</h3>
          {detail.flood ? <dl className="grid gap-2"><div><dt>ระดับ</dt><dd><StatusBadge status={detail.flood.severity} /></dd></div><div><dt>ระดับน้ำ</dt><dd>{depthLabel(detail.flood.water_depth_m)}</dd></div><div><dt>ความมั่นใจ</dt><dd>{confidenceLabel(detail.flood.confidence)}</dd></div><div><dt>กรอบพื้นที่</dt><dd>{detail.flood.bounding_box.length ? "มีกรอบพื้นที่จาก AI" : "ไม่มีกรอบพื้นที่"}</dd></div></dl> : <p>ไม่พบผลการประเมินน้ำท่วม</p>}
        </section>
        <section aria-labelledby="ai-road-result-heading" className="panel">
          <h3 id="ai-road-result-heading">ผลการประเมินถนน</h3>
          {detail.road ? <dl className="grid gap-2"><div><dt>สถานะ</dt><dd><StatusBadge status={detail.road.status} /></dd></div><div><dt>ความมั่นใจ</dt><dd>{confidenceLabel(detail.road.confidence)}</dd></div><div><dt>กรอบพื้นที่</dt><dd>{detail.road.bounding_box.length ? "มีกรอบพื้นที่จาก AI" : "ไม่มีกรอบพื้นที่"}</dd></div></dl> : <p>ไม่พบผลการประเมินถนน</p>}
        </section>
      </div>

      <section aria-labelledby="ai-limitations-heading">
        <h3 id="ai-limitations-heading">ข้อจำกัดของหลักฐาน</h3>
        {detail.limitations.length ? <ul>{detail.limitations.map((limitation, index) => <li key={`${detail.id}-limitation-${index}`}>{limitation}</li>)}</ul> : <p>ไม่มีข้อจำกัดเพิ่มเติมที่รายงาน</p>}
      </section>

      {detail.road && canReview && <div className="grid gap-2"><label htmlFor="ai-road-match">จับคู่ถนนสำหรับอนุมัติ</label><select id="ai-road-match" value={roadId} disabled={roadState !== "ready" || reviewingId === detail.id} onChange={(event) => { setRoadId(event.target.value); setValidationError(null); }}><option value="">เลือกถนนจริงจากรายการ</option>{roadOptions.map((road) => <option key={road.id} value={road.id}>{road.name}</option>)}</select>{roadState === "loading" && <p role="status">กำลังโหลดรายการถนน…</p>}{roadState === "error" && <div role="alert"><p>ไม่สามารถโหลดรายการถนนจริงได้</p><button type="button" className="action-button" onClick={() => void loadRoadOptions()}>โหลดถนนอีกครั้ง</button></div>}<p className="muted">อนุมัติผลถนนได้เมื่อเลือกถนนจริงจากรายการเท่านั้น</p></div>}

      {canReview && <div className="grid gap-3"><label htmlFor="ai-rejection-reason">เหตุผลการปฏิเสธ</label><textarea id="ai-rejection-reason" value={selectedReason} disabled={reviewingId === detail.id} onChange={(event) => { setRejectionReasons((current) => ({ ...current, [detail.id]: event.target.value })); setValidationError(null); }} rows={3} placeholder="ระบุเหตุผลที่ตรวจสอบได้" />{validationError && <p role="alert">{validationError}</p>}<div className="flex flex-wrap gap-2"><button type="button" className="button button--primary" disabled={!canReview || reviewingId === detail.id || Boolean(detail.road && (!roadId || roadState !== "ready"))} onClick={() => void review("verify")}>อนุมัติหลักฐาน AI</button><button type="button" className="action-button action-button--danger" disabled={!canReview || reviewingId === detail.id} onClick={() => void review("reject")}>ปฏิเสธหลักฐาน AI</button></div></div>}
    </section>;
  };

  return <div className="page-container admin-page space-y-4">
    <header className="page-header"><p className="eyebrow">AI EVIDENCE REVIEW</p><h1>ตรวจสอบหลักฐานจาก AI</h1><p>ผลการประเมินยังไม่ใช่ข้อมูลทางการ ต้องตรวจสอบแหล่งที่มา ข้อจำกัด และเวลาที่เกี่ยวข้องก่อนอนุมัติ</p></header>
    {conflictId && <div className="panel error-state" role="alert"><p>รายการนี้ถูกเปลี่ยนแปลงแล้วโดยผู้ดูแลระบบอื่น</p><button type="button" className="action-button" onClick={() => { setConflictId(null); setSelectedId(null); setDetail(null); setRetry((current) => current + 1); }}>โหลดรายการอีกครั้ง</button></div>}
    {listState === "loading" && <p role="status">กำลังโหลดหลักฐาน AI…</p>}
    {listError && <div className="panel error-state" role="alert"><p>ไม่สามารถโหลดหลักฐาน AI ได้ กรุณาลองใหม่อีกครั้ง</p><button type="button" className="action-button" onClick={() => setRetry((current) => current + 1)}>ลองอีกครั้ง</button></div>}
    {listState === "ready" && page?.items.length === 0 && <div className="panel"><p>ไม่มีหลักฐาน AI ที่รอการตรวจสอบ</p></div>}
    {listState === "ready" && page && page.items.length > 0 && <section aria-labelledby="ai-evidence-list-heading"><h2 id="ai-evidence-list-heading">รายการที่รอตรวจสอบ</h2><div className="grid gap-4">{page.items.map((item) => <article key={item.id} className="panel"><div className="flex items-start justify-between gap-4"><div><h3>หลักฐาน {item.id}</h3><div className="badge-row"><SourceBadge type="AI" />{evidenceStatus(item.status)}</div></div><span className="badge">รุ่นโมเดล: {item.modelVersion}</span></div><p>{item.flood ? `น้ำท่วม: ${item.flood.severity} · ความมั่นใจ ${confidenceLabel(item.flood.confidence)}` : "ไม่พบผลน้ำท่วม"}{item.road ? ` · ถนน: ${item.road.status} · ความมั่นใจ ${confidenceLabel(item.road.confidence)}` : " · ไม่พบผลถนน"}</p><p><DataTimestamp value={item.receivedAt} label="รับข้อมูล" /></p><div className="flex flex-wrap items-center gap-3"><Link to={`/reports?report=${encodeURIComponent(item.reportId)}`}>ดูรายงานที่เกี่ยวข้อง</Link><button type="button" className="action-button" onClick={() => void openDetails(item.id)}>เปิดรายละเอียด: {item.id}</button></div></article>)}</div></section>}
    {listState === "ready" && page && page.pagination.total > 0 && <nav className="flex flex-wrap items-center justify-between gap-3" aria-label="การแบ่งหน้าหลักฐาน AI"><span>แสดง {page.items.length ? page.pagination.offset + 1 : 0}–{Math.min(page.pagination.offset + page.items.length, page.pagination.total)} จาก {page.pagination.total}</span><div className="flex gap-2"><button type="button" className="action-button" disabled={page.pagination.offset === 0} onClick={() => { setOffset(Math.max(0, page.pagination.offset - page.pagination.limit)); setSelectedId(null); setDetail(null); }}>หน้าก่อนหน้า</button><button type="button" className="action-button" disabled={page.pagination.offset + page.pagination.limit >= page.pagination.total} onClick={() => { setOffset(page.pagination.offset + page.pagination.limit); setSelectedId(null); setDetail(null); }}>หน้าถัดไป</button></div></nav>}
    {renderDetail()}
  </div>;
}
