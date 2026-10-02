import { useEffect, useState } from "react";
import { DataTimestamp, FreshnessBadge, SourceBadge, StatusBadge } from "../../shared/components/DataStates";
import { useRealtimeRevision } from "../realtime/RealtimeProvider";
import { listPublicAlerts } from "./api";
import type { PublicAlert } from "./api";
import "./AnnouncementsPage.css";

const alertEvents = ["alert.created"] as const;
const filters: Array<{ severity: PublicAlert["severity"] | "ALL"; label: string }> = [
  { severity: "ALL", label: "ทั้งหมด" },
  { severity: "CRITICAL", label: "ประกาศสำคัญ" },
  { severity: "WARNING", label: "เตือนภัย" },
  { severity: "WATCH", label: "เฝ้าระวัง" },
  { severity: "INFO", label: "ข้อมูลทั่วไป" }
];

function matchesFilter(alert: PublicAlert, filter: PublicAlert["severity"] | "ALL") {
  return filter === "ALL" || alert.severity === filter;
}

export function AnnouncementsPage() {
  const realtimeRevision = useRealtimeRevision(alertEvents);
  const [retry, setRetry] = useState(0);
  const [filter, setFilter] = useState<PublicAlert["severity"] | "ALL">("ALL");
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

  const visibleAlerts = result.items.filter((alert) => matchesFilter(alert, filter));
  const featuredAlert = visibleAlerts[0];
  const hasAlerts = result.items.length > 0;
  const showError = result.state === "error";
  const showLoading = result.state === "loading";
  const summaryState = showError ? "ข้อมูลสรุปไม่พร้อมใช้งาน" : showLoading ? "กำลังโหลดข้อมูลสรุป" : !hasAlerts ? "ยังไม่มีข้อมูลสรุป" : null;
  const sourceCounts = result.items.reduce<Record<PublicAlert["sourceType"], number>>((counts, alert) => {
    counts[alert.sourceType] += 1;
    return counts;
  }, { OFFICIAL: 0, COMMUNITY: 0, AI: 0 });

  return <section className="feature-page announcements-page">
    <div className="announcements-page__header">
      <div className="page-heading">
        <p className="eyebrow">PUBLIC INFORMATION</p>
        <h1>ประกาศและแจ้งเตือน</h1>
        <p>ติดตามประกาศที่เผยแพร่ พร้อมระดับความรุนแรง แหล่งข้อมูล และเวลาที่แจ้งเตือน</p>
      </div>
    </div>
    <div className="announcements-page__layout">
        <div className="announcements-page__main">
          <div className="announcement-filters" aria-label="กรองประกาศตามระดับความรุนแรง">
            {filters.map((option) => {
              const count = result.items.filter((alert) => matchesFilter(alert, option.severity)).length;
              return <button key={option.severity} type="button" className="announcement-filter" aria-pressed={filter === option.severity} onClick={() => setFilter(option.severity)}>
                {option.label}{showError || showLoading ? "" : ` (${count})`}
              </button>;
            })}
          </div>

          {showLoading ? <section className="announcement-state" role="status" aria-label="กำลังโหลดข้อมูล"><h2>กำลังโหลดข้อมูล</h2><p>กำลังตรวจสอบข้อมูลจาก API สาธารณะ</p></section> :
            showError ? <section className="announcement-state announcement-state--error" role="alert"><h2>ไม่สามารถโหลดข้อมูลสถานการณ์ได้</h2><p>โปรดลองอีกครั้ง ข้อมูลที่ไม่พร้อมใช้งานไม่ได้หมายถึงพื้นที่ปลอดภัย</p><button type="button" className="action-button" onClick={() => setRetry((value) => value + 1)}>ลองอีกครั้ง</button></section> :
            !hasAlerts ? <section className="announcement-state"><h2>ยังไม่มีประกาศที่เผยแพร่</h2><p>ไม่มี alert จากกฎที่กำหนดไว้ในขณะนี้ ไม่ได้หมายความว่าพื้นที่ปลอดภัย โปรดตรวจสอบข้อมูลล่าสุดจากแหล่งทางการด้วย</p></section> :
            featuredAlert ? <article className={`announcement-feature announcement-feature--${featuredAlert.severity.toLowerCase()}`}>
            <div className="announcement-feature__content">
              <div className="announcement-feature__heading"><StatusBadge status={featuredAlert.severity} /><span className="announcement-feature__label">ประกาศล่าสุด</span></div>
              <h2>{featuredAlert.title}</h2>
              <p>{featuredAlert.message}</p>
              <div className="announcement-feature__metadata"><DataTimestamp label="เริ่มแจ้งเตือน" value={featuredAlert.triggeredAt} /><SourceBadge type={featuredAlert.sourceType} /></div>
            </div>
          </article> : <section className="announcement-state"><h2>ไม่พบประกาศในระดับนี้</h2><p>เลือกตัวกรองอื่นเพื่อดูประกาศที่เผยแพร่ทั้งหมด</p></section>}

          {!showLoading && !showError && visibleAlerts.length > 0 && <section className="announcements-list" aria-labelledby="published-alerts-heading">
            <div className="announcements-list__heading"><div><h2 id="published-alerts-heading">รายการประกาศทั้งหมด</h2><p className="section-caption">ข้อมูลที่แสดงอ้างอิงรายการจาก API สาธารณะ</p></div><span>{visibleAlerts.length} รายการ</span></div>
            <ul>{visibleAlerts.map((alert) => <li key={alert.id}>
              <article className="announcement-row">
                <div className="announcement-row__severity"><StatusBadge status={alert.severity} /></div>
                <div className="announcement-row__content"><h3>{alert.title}</h3><p>{alert.message}</p><div className="badge-row"><SourceBadge type={alert.sourceType} /><FreshnessBadge freshness="UNKNOWN" /><span className="badge">{alert.resourceType}</span></div></div>
                <div className="announcement-row__meta"><DataTimestamp label="เริ่มแจ้งเตือน" value={alert.triggeredAt} /><span>หลักฐานปัจจุบัน: ไม่ทราบความสด</span></div>
              </article>
            </li>)}</ul>
          </section>}
        </div>
        <aside className="announcements-page__sidebar" aria-label="สรุปประกาศ">
          <section className="announcement-summary"><div className="announcement-summary__heading"><h2>สถานะประกาศ</h2><span>ข้อมูลที่เผยแพร่</span></div><div className="announcement-summary__grid">
            {summaryState ? <p className="announcement-summary__state">{summaryState}</p> : <>
            <div><strong>{result.items.length}</strong><span>ทั้งหมด</span></div>
            <div><strong>{result.items.filter((alert) => alert.severity === "CRITICAL" || alert.severity === "WARNING").length}</strong><span>ต้องระวัง</span></div>
            <div><strong>{result.items.filter((alert) => alert.severity === "WATCH").length}</strong><span>เฝ้าระวัง</span></div>
            <div><strong>{result.items.filter((alert) => alert.severity === "INFO").length}</strong><span>ข้อมูลทั่วไป</span></div>
            </>}
          </div></section>
          <section className="announcement-sidebar-card"><h2>แหล่งข้อมูลประกาศ</h2><p className="section-caption">{showError ? "ไม่สามารถสรุปแหล่งข้อมูลได้ในขณะนี้" : showLoading ? "กำลังตรวจสอบแหล่งข้อมูล" : !hasAlerts ? "ยังไม่มีแหล่งข้อมูลในรายการที่เผยแพร่" : "แหล่งที่มาแสดงตามข้อมูลของแต่ละประกาศ"}</p>
            {hasAlerts && !showError && !showLoading && <ul className="announcement-source-list">{(Object.entries(sourceCounts) as Array<[PublicAlert["sourceType"], number]>).map(([sourceType, count]) => <li key={sourceType}><SourceBadge type={sourceType} /><strong>{count} รายการ</strong></li>)}</ul>}
          </section>
          <section className="announcement-sidebar-card"><h2>ข้อควรทราบ</h2><p>ความสดของหลักฐานสำหรับ alert ยังไม่ระบุ จึงไม่ควรใช้รายการเดิมยืนยันว่าพื้นที่ปลอดภัย</p></section>
        </aside>
      </div>
  </section>;
}
