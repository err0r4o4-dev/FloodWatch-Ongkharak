import { useEffect, useState } from "react";
import { DataTimestamp, EmptyState, ErrorState, FreshnessBadge, LoadingState, SourceBadge, StatusBadge } from "../../shared/components/DataStates";
import { useRealtimeRevision } from "../realtime/RealtimeProvider";
import { listPublicAlerts } from "./api";
import type { PublicAlert } from "./api";

const alertEvents = ["alert.created"] as const;

export function AnnouncementsPage() {
  const realtimeRevision = useRealtimeRevision(alertEvents);
  const [retry, setRetry] = useState(0);
  const [result, setResult] = useState<{ state: "loading" | "ready" | "error"; items: PublicAlert[] }>({ state: "loading", items: [] });

  useEffect(() => {
    const controller = new AbortController();
    setResult((current) => ({ ...current, state: "loading" }));
    void listPublicAlerts(controller.signal).then((items) => {
      if (!controller.signal.aborted) setResult({ state: "ready", items });
    }).catch(() => {
      if (!controller.signal.aborted) setResult({ state: "error", items: [] });
    });
    return () => controller.abort();
  }, [retry, realtimeRevision]);

  return <section className="feature-page"><div className="page-heading"><p className="eyebrow">PUBLIC INFORMATION</p><h1>ประกาศ</h1><p>พื้นที่สำหรับประกาศที่เผยแพร่และตรวจสอบแหล่งข้อมูลได้</p></div>
    {result.state === "loading" ? <LoadingState /> : result.state === "error" ? <ErrorState onRetry={() => setRetry((value) => value + 1)} /> : result.items.length === 0 ?
      <EmptyState title="ยังไม่มีประกาศที่เผยแพร่">ไม่มี alert จากกฎที่กำหนดไว้ในขณะนี้ ไม่ได้หมายความว่าพื้นที่ปลอดภัย โปรดตรวจสอบข้อมูลล่าสุดจากแหล่งทางการด้วย</EmptyState> :
      <ul className="data-list">{result.items.map((alert) => <li key={alert.id}><article className="data-card"><div className="data-card__heading"><h2>{alert.title}</h2><StatusBadge status={alert.severity} /></div><p>{alert.message}</p><div className="badge-row"><SourceBadge type={alert.sourceType} /><FreshnessBadge freshness="UNKNOWN" /><span className="badge">{alert.resourceType}</span></div><p className="data-card__meta"><DataTimestamp label="เริ่มแจ้งเตือน" value={alert.triggeredAt} /></p><p className="section-caption">ความสดของหลักฐานปัจจุบันไม่ทราบ · ตรวจสอบข้อมูลต้นทางก่อนตัดสินใจ ไม่ใช่การยืนยันว่าพื้นที่ปลอดภัย</p></article></li>)}</ul>}
  </section>;
}
