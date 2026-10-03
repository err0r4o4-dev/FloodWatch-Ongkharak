import { useEffect, useState, type ReactNode } from "react";
import { Link } from "react-router";
import { DataTimestamp, EmptyState, ErrorState, FreshnessBadge, LoadingState, SourceBadge, StatusBadge } from "../../shared/components/DataStates";
import { HeroImageCredit } from "../../shared/components/HeroImageCredit";
import type { Freshness, SourceRef } from "../../shared/types/flood";
import { listPublicAlerts, type PublicAlert } from "../announcements/api";
import { reportsApi } from "../reports/api";
import { ReportMediaImage } from "../reports/ReportMediaImage";
import type { CommunityReport } from "../reports/types";
import { useRealtimeRevision } from "../realtime/RealtimeProvider";
import { loadCollection, type Collections, type CollectionKey, type WaterStation } from "./api";
import "./OverviewPage.css";

const overviewEvents = ["water_level.updated", "flood_event.created", "flood_event.updated", "road.status.changed", "alert.created", "report.verified"] as const;
const keys: CollectionKey[] = ["events", "risks", "roads", "water"];
type FeedKey = CollectionKey | "alerts" | "reports";
type OverviewCollections = Collections & { alerts: PublicAlert[]; reports: CommunityReport[] };
type Result = { collections: Partial<OverviewCollections>; failed: FeedKey[] };
const headings: Record<FeedKey, string> = {
  events: "เหตุการณ์น้ำ", risks: "พื้นที่ประเมินความเสี่ยง", roads: "สภาพถนน", water: "สถานีวัดน้ำ", alerts: "ประกาศล่าสุด", reports: "รายงานจากประชาชน"
};
const reportLabels = { FLOOD: "รายงานน้ำท่วม", ROAD: "รายงานสภาพถนน", OTHER: "รายงานสถานการณ์" };
type OverviewIconName = "water" | "rain" | "risk" | "road" | "announcement" | "people";

function OverviewIcon({ name }: { name: OverviewIconName }) {
  const paths: Record<OverviewIconName, ReactNode> = {
    water: <><path d="M3 7c3-4 6 4 9 0s6 4 9 0M3 12c3-4 6 4 9 0s6 4 9 0M3 17c3-4 6 4 9 0s6 4 9 0" /></>,
    rain: <><path d="M6 15a4 4 0 1 1 1-8 5 5 0 0 1 10 0 4 4 0 0 1 1 8M7 18l-1 3M12 18l-1 3M17 18l-1 3" /></>,
    risk: <><path d="m12 3 10 18H2L12 3Z" /><path d="M12 9v5M12 17h.01" /></>,
    road: <><path d="M7 3 3 21M17 3l4 18M12 3v3M12 10v4M12 18v3" /></>,
    announcement: <><path d="m3 9 16-5v15L3 14V9ZM19 9h2v5h-2M6 15l2 6h4l-2-5" /></>,
    people: <><circle cx="9" cy="8" r="3" /><path d="M3 21v-2a6 6 0 0 1 12 0v2M17 5a3 3 0 0 1 0 6M18 15a5 5 0 0 1 3 5" /></>
  };
  return <svg viewBox="0 0 24 24" width="24" height="24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">{paths[name]}</svg>;
}

function effectiveStatus(value: string, freshness: Freshness, verification?: string) {
  return freshness === "FRESH" && (!verification || verification === "VERIFIED") ? value : "UNKNOWN";
}

function latest<T>(items: T[] | undefined, timestamp: (item: T) => string | null, limit = 5) {
  const time = (item: T) => Date.parse(timestamp(item) ?? "") || 0;
  return [...(items ?? [])].sort((first, second) => time(second) - time(first)).slice(0, limit);
}

function OverviewPanel({ id, title, icon, to, caption, children }: {
  id: string; title: string; icon: OverviewIconName; to: string; caption?: string; children: ReactNode;
}) {
  return <section className="data-section overview-panel" aria-labelledby={id}>
    <div className="overview-section-header"><h2 className="overview-section-title" id={id}><span className="overview-section-icon" aria-hidden="true"><OverviewIcon name={icon} /></span>{title}</h2><Link className="overview-section-action" to={to} aria-label={`ดูทั้งหมด: ${title}`}>ดูทั้งหมด →</Link></div>
    {caption && <p className="section-caption">{caption}</p>}
    {children}
  </section>;
}

function PanelState({ result, feed, empty }: { result: Result | null; feed: FeedKey; empty: string }) {
  return <p className="overview-panel-state" role="status">{!result ? "กำลังโหลดข้อมูล" : result.failed.includes(feed) ? `ไม่สามารถโหลด${headings[feed]}ได้ · ข้อมูลไม่พร้อมใช้งาน ไม่ใช่การยืนยันความปลอดภัย` : empty}</p>;
}

function Evidence({ source, freshness }: { source: SourceRef | null; freshness: Freshness }) {
  return <div className="badge-row overview-evidence">{source ? <SourceBadge type={source.sourceType} /> : <span className="badge">ไม่ทราบแหล่งข้อมูล</span>}<FreshnessBadge freshness={freshness} /></div>;
}

function Observation({ source, time, label }: { source: SourceRef | null; time: string | null; label?: string }) {
  return <div className="overview-cell-meta">{source?.name ?? "ไม่ระบุแหล่งข้อมูล"} · <DataTimestamp value={time} label={label} /></div>;
}

function CompactObservation({ value }: { value: string | null }) {
  const date = value ? new Date(value) : null;
  if (!date || Number.isNaN(date.getTime())) return <span className="overview-cell-meta">ไม่ระบุเวลา</span>;
  const options = { timeZone: "Asia/Bangkok" };
  const full = new Intl.DateTimeFormat("th-TH", { ...options, dateStyle: "medium", timeStyle: "short" }).format(date);
  return <time className="overview-reading-time" dateTime={value!} title={full} aria-label={`เวลาสังเกต: ${full}`}><strong>{new Intl.DateTimeFormat("th-TH", { ...options, hour: "2-digit", minute: "2-digit", hourCycle: "h23" }).format(date)} น.</strong><small>{new Intl.DateTimeFormat("th-TH", { ...options, day: "numeric", month: "short", year: "2-digit" }).format(date)}</small></time>;
}

function Table({ label, columns, children }: { label: string; columns: string[]; children: ReactNode }) {
  return <div className="overview-table-wrap" tabIndex={0} role="region" aria-label={`ตาราง${label} เลื่อนเพื่อดูข้อมูลทั้งหมด`}><table className="overview-table"><caption className="sr-only">{label}</caption><thead><tr>{columns.map((column) => <th key={column} scope="col">{column}</th>)}</tr></thead><tbody>{children}</tbody></table></div>;
}

function ReadingTable({ stations, rainfall = false }: { stations: WaterStation[]; rainfall?: boolean }) {
  return <Table label={rainfall ? "ปริมาณฝน" : "ระดับน้ำ"} columns={["สถานี / แหล่งข้อมูล", rainfall ? "ปริมาณฝน" : "ค่าที่รายงาน", "สถานะ", "เวลาสังเกต"]}>
    {stations.map((station) => {
      const reading = station.latestReading;
      const source = reading?.source ?? station.source;
      const value = rainfall ? reading?.rainfallMm : reading?.waterLevelM;
      const known = value !== null && value !== undefined;
      return <tr key={station.id}><th scope="row">{station.name}<div className="overview-cell-source"><SourceBadge type={source.sourceType} /><small>{source.name}</small></div></th><td>{known ? <strong>{rainfall ? `${value} มม.` : `ระดับน้ำ ${value} ม.`}</strong> : "ยังไม่มีค่าที่สังเกตได้"}{!rainfall && reading?.flowRateM3s !== null && reading?.flowRateM3s !== undefined && <p>อัตราการไหล {reading.flowRateM3s} ลบ.ม./วินาที</p>}</td>
        <td><StatusBadge status={known && reading ? effectiveStatus("INFO", reading.freshness, reading.verificationStatus) : "UNKNOWN"} /><FreshnessBadge freshness={reading?.freshness ?? "UNKNOWN"} /></td>
        <td><CompactObservation value={reading?.observedAt ?? null} /></td></tr>;
    })}
  </Table>;
}

export function OverviewPage() {
  const [result, setResult] = useState<Result | null>(null);
  const [revision, setRevision] = useState(0);
  const realtimeRevision = useRealtimeRevision(overviewEvents);

  useEffect(() => {
    const controller = new AbortController();
    setResult(null);
    const requests: Array<{ key: FeedKey; load: Promise<OverviewCollections[FeedKey]> }> = [
      ...keys.map((key) => ({ key, load: loadCollection(key, controller.signal) })),
      { key: "alerts", load: listPublicAlerts(controller.signal) },
      { key: "reports", load: reportsApi.listPublic({ signal: controller.signal }).then((page) => page.items.filter((report) => report.status === "VERIFIED" && report.verificationStatus === "VERIFIED" && report.sourceType === "COMMUNITY")) }
    ];
    void Promise.allSettled(requests.map((request) => request.load)).then((outcomes) => {
      if (controller.signal.aborted) return;
      const collections: Partial<OverviewCollections> = {};
      const failed: FeedKey[] = [];
      outcomes.forEach((outcome, index) => {
        const key = requests[index].key;
        if (outcome.status === "rejected") failed.push(key);
        else Object.assign(collections, { [key]: outcome.value });
      });
      setResult({ collections, failed });
    });
    return () => controller.abort();
  }, [revision, realtimeRevision]);

  const collections = result?.collections;
  const water = latest(collections?.water, (station) => station.latestReading?.observedAt ?? null);
  const risks = latest(collections?.risks, (risk) => risk.validFrom);
  const roads = latest(collections?.roads, (road) => road.currentCondition.observedAt);
  const events = latest(collections?.events, (event) => event.observedAt);
  const alerts = latest(collections?.alerts, (alert) => alert.triggeredAt, 3);
  const reports = latest(collections?.reports, (report) => report.createdAt, 3);
  const allFailed = result?.failed.length === keys.length + 2;
  const hasItems = collections && Object.values(collections).some((items) => items.length > 0);
  const metrics = [
    { key: "risks", label: "พื้นที่เสี่ยงในรายการ", to: "/risk", icon: "risk", tone: "danger", caption: "พื้นที่ประเมินที่โหลดได้" },
    { key: "water", label: "สถานีวัดน้ำในรายการ", to: "/map", icon: "water", tone: "", caption: "ค่าล่าสุดพร้อมแหล่งข้อมูล" },
    { key: "events", label: "เหตุการณ์น้ำในรายการ", to: "/map", icon: "rain", tone: "sky", caption: "เหตุการณ์ที่มีข้อมูลเผยแพร่" },
    { key: "roads", label: "ถนนในรายการ", to: "/map", icon: "road", tone: "road", caption: "ดูสถานะก่อนตัดสินใจเดินทาง" }
  ] as const;

  return <div className="overview-page">
    <header className="page-hero"><div className="page-hero__copy"><p className="eyebrow">FLOOD WATCH · ONGKHARAK</p><h1>ภาพรวมสถานการณ์น้ำ</h1><p>อำเภอองครักษ์ จังหวัดนครนายก</p><small>ติดตามข้อมูลที่เผยแพร่ พร้อมแหล่งที่มาและเวลาสังเกต · ไม่ใช่การยืนยันว่าพื้นที่ปลอดภัย</small></div><div className="page-hero__context"><strong>ข้อมูลสถานการณ์</strong><span>พร้อมแหล่งที่มาและเวลาสังเกต</span></div><HeroImageCredit /></header>
    <div className="summary-metrics" aria-label={result ? "สรุปข้อมูลที่โหลดได้" : "กำลังโหลดข้อมูลสรุป"}>{metrics.map((metric) => <Link key={metric.key} to={metric.to} className="summary-metric"><span className={`summary-metric__icon${metric.tone ? ` summary-metric__icon--${metric.tone}` : ""}`} aria-hidden="true"><OverviewIcon name={metric.icon} /></span><span>{metric.label}<strong>{collections?.[metric.key]?.length ?? "—"}</strong><small>{!result ? "กำลังโหลดข้อมูล" : result.failed.includes(metric.key) ? "ข้อมูลยังไม่พร้อมใช้งาน" : metric.caption}</small></span><span className="summary-metric__arrow" aria-hidden="true">›</span></Link>)}</div>
    {!result && <LoadingState />}
    {allFailed ? <ErrorState onRetry={() => setRevision((previous) => previous + 1)} /> : result && result.failed.length > 0 && <div className="notice" role="status"><strong>ข้อมูลบางส่วนไม่พร้อมใช้งาน</strong><span>โหลดไม่ได้: {result.failed.map((key) => headings[key]).join(", ")} · ไม่ควรใช้ข้อมูลที่ขาดหายยืนยันความปลอดภัย</span><button type="button" onClick={() => setRevision((previous) => previous + 1)}>ลองอีกครั้ง</button></div>}
    {result && !allFailed && !hasItems && <EmptyState title="ยังไม่มีข้อมูลสถานการณ์ที่เผยแพร่">ยังไม่มีรายการในชุดข้อมูลที่เรียกดู หรือข้อมูลบางส่วนไม่พร้อมใช้งาน กรุณาตรวจสอบแหล่งข้อมูลอีกครั้งก่อนตัดสินใจเดินทาง</EmptyState>}
    <div className="overview-dashboard"><div className="overview-primary">
      <OverviewPanel id="water-title" title="ระดับน้ำ (จุดสำคัญ)" icon="water" to="/map" caption="ค่าที่รายงานสูงสุด 5 สถานี · ยังไม่มีชุดย้อนหลังสำหรับคำนวณแนวโน้ม">
        {water.length ? <ReadingTable stations={water} /> : <PanelState result={result} feed="water" empty="ยังไม่มีสถานีในรายการที่เรียกดู" />}
      </OverviewPanel>
      <OverviewPanel id="rainfall-title" title="ปริมาณฝนที่รายงาน" icon="rain" to="/map" caption="ค่าจากสถานีที่โหลดได้ · ไม่ใช่ยอดสะสมทั้งอำเภอหรือยอดรายวัน">
        {water.length ? <ReadingTable stations={water} rainfall /> : <PanelState result={result} feed="water" empty="ยังไม่มีค่าฝนจากสถานีในรายการ" />}
      </OverviewPanel>
      <OverviewPanel id="risks-title" title="พื้นที่เสี่ยง (ล่าสุด)" icon="risk" to="/risk" caption="สูงสุด 5 รายการจากผลประเมินที่โหลดได้">
        {risks.length ? <Table label="พื้นที่เสี่ยง" columns={["พื้นที่", "ระดับความเสี่ยง", "รายละเอียด / แหล่งข้อมูล"]}>{risks.map((risk) => <tr key={risk.id}><th scope="row">{risk.name}</th><td><StatusBadge status={effectiveStatus(risk.effectiveRiskLevel, risk.freshness)} /><Evidence source={risk.source} freshness={risk.freshness} /></td><td>{risk.description}<Observation source={risk.source} time={risk.validFrom} label="เริ่มมีผล" />{risk.validTo && <DataTimestamp value={risk.validTo} label="สิ้นสุดช่วงมีผล" />}</td></tr>)}</Table> : <PanelState result={result} feed="risks" empty="ยังไม่มีผลประเมินพื้นที่ในรายการที่เรียกดู" />}
      </OverviewPanel>
      <OverviewPanel id="roads-title" title="สภาพถนน (ล่าสุด)" icon="road" to="/map" caption="สูงสุด 5 รายการ · สถานะที่รายงานไม่ใช่การรับรองเส้นทางปลอดภัย">
        {roads.length ? <Table label="สภาพถนน" columns={["ชื่อเส้นทาง", "สถานะ", "แหล่งข้อมูล / เวลา"]}>{roads.map((road) => <tr key={road.id}><th scope="row">{road.name}</th><td><StatusBadge status={effectiveStatus(road.currentCondition.status, road.currentCondition.freshness, road.currentCondition.verificationStatus)} /><Evidence source={road.currentCondition.source} freshness={road.currentCondition.freshness} /></td><td><Observation source={road.currentCondition.source} time={road.currentCondition.observedAt} /></td></tr>)}</Table> : <PanelState result={result} feed="roads" empty="ยังไม่มีข้อมูลถนนในรายการที่เรียกดู" />}
      </OverviewPanel>
      <OverviewPanel id="events-title" title="เหตุการณ์น้ำที่รายงาน" icon="rain" to="/map" caption="แยกเหตุการณ์ออกจากประกาศและรายงานที่บุคคลส่งเข้ามา">
        {events.length ? <ul className="overview-bulletins">{events.map((event) => <li className={`overview-bulletin overview-bulletin--${effectiveStatus(event.effectiveSeverity, event.freshness, event.verificationStatus).toLowerCase()}`} key={event.id}><article><h3>{event.title}</h3><p>{event.description}</p><div className="badge-row"><StatusBadge status={effectiveStatus(event.effectiveSeverity, event.freshness, event.verificationStatus)} /><StatusBadge status={effectiveStatus(event.effectiveStatus, event.freshness, event.verificationStatus)} /></div><Evidence source={event.source} freshness={event.freshness} /><Observation source={event.source} time={event.observedAt} /></article></li>)}</ul> : <PanelState result={result} feed="events" empty="ยังไม่มีเหตุการณ์ในรายการ" />}
      </OverviewPanel>
    </div><aside className="overview-sidebar" aria-label="ประกาศและรายงานจากประชาชน">
      <OverviewPanel id="alerts-title" title="ประกาศล่าสุด" icon="announcement" to="/announcements" caption="ประกาศที่เผยแพร่จากระบบแจ้งเตือน">
        {alerts.length ? <ul className="overview-bulletins">{alerts.map((alert) => <li className={`overview-bulletin overview-bulletin--${alert.severity === "WATCH" ? "warning" : alert.severity.toLowerCase()}`} key={alert.id}><article><div className="badge-row"><StatusBadge status={alert.severity} /><FreshnessBadge freshness="UNKNOWN" /><SourceBadge type={alert.sourceType} /></div><h3>{alert.title}</h3><p>{alert.message}</p><div className="overview-cell-meta"><DataTimestamp value={alert.triggeredAt} label="เริ่มแจ้งเตือน" /></div></article></li>)}</ul> : <PanelState result={result} feed="alerts" empty="ยังไม่มีประกาศที่เผยแพร่" />}
      </OverviewPanel>
      <OverviewPanel id="reports-title" title="รายงานจากประชาชน (ล่าสุด)" icon="people" to="/reports" caption="รายงานที่บุคคลส่งเข้ามาและผ่านการตรวจสอบแล้ว">
        {reports.length ? <ul className="overview-report-list">{reports.map((report) => <li key={report.id}><article><div className="overview-report-thumbnail">{report.media[0] ? <ReportMediaImage media={report.media[0]} /> : <span>ไม่มีภาพประกอบ</span>}</div><div className="overview-report-copy"><Link className="overview-report-link" to={`/reports?report=${encodeURIComponent(report.id)}`}><div><h3>{reportLabels[report.type]}</h3><p>{report.description}</p></div></Link><div className="badge-row"><SourceBadge type="COMMUNITY" /><span className="badge">ยืนยันแล้ว</span></div><small><DataTimestamp value={report.createdAt} label="ส่งรายงาน" /></small></div></article></li>)}</ul> : <PanelState result={result} feed="reports" empty="ยังไม่มีรายงานจากประชาชนที่ยืนยันแล้ว" />}
        <p className="overview-footer-note">ข้อมูลจากบุคคล ไม่ใช่ประกาศทางการ · การยืนยันรายงานไม่ใช่การยืนยันความปลอดภัยในปัจจุบัน</p>
      </OverviewPanel>
    </aside></div>
    <p className="overview-footer-note">จำนวนในสรุปนับเฉพาะรายการที่โหลดได้ ไม่ใช่ยอดรวมทั้งอำเภอ · ข้อมูลเก่าหรือไม่พร้อมใช้งานแสดงเป็นไม่ทราบสถานะ</p>
  </div>;
}
