import { useEffect, useState } from "react";
import { DataTimestamp, EmptyState, ErrorState, FreshnessBadge, LoadingState, SourceBadge, StatusBadge } from "../../shared/components/DataStates";
import { loadCollection } from "./api";
import type { CollectionKey, Collections } from "./api";
import type { Freshness } from "../../shared/types/flood";
import { useRealtimeRevision } from "../realtime/RealtimeProvider";
import { NotificationToggle } from "../notifications/NotificationToggle";

const overviewEvents = ["water_level.updated", "flood_event.created", "flood_event.updated", "road.status.changed"] as const;

const keys: CollectionKey[] = ["events", "risks", "roads", "water"];
const headings: Record<CollectionKey, string> = {
  events: "เหตุการณ์น้ำ", risks: "พื้นที่ประเมินความเสี่ยง", roads: "สภาพถนน", water: "สถานีวัดน้ำ"
};
type Result = { collections: Partial<Collections>; failed: CollectionKey[] };

function effectiveStatus(value: string, freshness: Freshness, verification?: string) {
  return freshness === "FRESH" && (!verification || verification === "VERIFIED") ? value : "UNKNOWN";
}

export function OverviewPage() {
  const [result, setResult] = useState<Result | null>(null);
  const [revision, setRevision] = useState(0);
  const realtimeRevision = useRealtimeRevision(overviewEvents);

  useEffect(() => {
    const controller = new AbortController();
    setResult(null);
    void Promise.allSettled(keys.map((key) => loadCollection(key, controller.signal))).then((outcomes) => {
      if (controller.signal.aborted) return;
      const collections: Partial<Collections> = {};
      const failed: CollectionKey[] = [];
      outcomes.forEach((outcome, index) => {
        const key = keys[index];
        if (outcome.status === "rejected") failed.push(key);
        else if (key === "events") collections.events = outcome.value as Collections["events"];
        else if (key === "risks") collections.risks = outcome.value as Collections["risks"];
        else if (key === "roads") collections.roads = outcome.value as Collections["roads"];
        else collections.water = outcome.value as Collections["water"];
      });
      setResult({ collections, failed });
    });
    return () => controller.abort();
  }, [revision, realtimeRevision]);

  const retry = () => setRevision((previous) => previous + 1);
  const hasItems = result && Object.values(result.collections).some((items) => items.length > 0);

  return <div className="overview-page">
    <div className="page-heading">
      <div style={{ display: 'flex', justifyContent: 'space-between', flexWrap: 'wrap', gap: '1rem', alignItems: 'flex-start' }}>
        <div>
          <p className="eyebrow">FLOOD WATCH · ONGKHARAK</p><h1>ภาพรวมสถานการณ์น้ำ</h1>
          <p>ข้อมูลจาก API สาธารณะพร้อมแหล่งที่มาและเวลา ไม่ใช่การยืนยันว่าพื้นที่ปลอดภัย</p>
        </div>
        <div style={{ padding: '0.75rem', borderRadius: '4px', border: '1px solid var(--border)', background: 'var(--surface-raised, #fff)' }}>
          <NotificationToggle />
        </div>
      </div>
    </div>
    {!result ? <LoadingState /> : result.failed.length === keys.length ? <ErrorState onRetry={retry} /> : <>
      {result.failed.length > 0 && <div className="notice" role="status"><strong>ข้อมูลบางส่วนไม่พร้อมใช้งาน</strong><span>โหลดไม่ได้: {result.failed.map((key) => headings[key]).join(", ")} · ไม่ควรใช้ข้อมูลที่ขาดหายยืนยันความปลอดภัย</span><button type="button" onClick={retry}>ลองอีกครั้ง</button></div>}
      {!hasItems ? <EmptyState title="ยังไม่มีข้อมูลสถานการณ์ที่เผยแพร่">ยังไม่มีรายการในชุดข้อมูลที่เรียกดู หรือข้อมูลบางส่วนไม่พร้อมใช้งาน กรุณาตรวจสอบแหล่งข้อมูลอีกครั้งก่อนตัดสินใจเดินทาง</EmptyState> : <div className="overview-sections">
        {result.collections.water && <section className="data-section" aria-labelledby="water-title"><h2 id="water-title">สถานีวัดน้ำ</h2><p className="section-caption">สูงสุด 5 สถานี · ค่าที่รายงานไม่ใช่การรับรองความปลอดภัย</p>
          {result.collections.water.length ? <ul className="data-list">{result.collections.water.map((station) => <li className="data-card" key={station.id}><div className="data-card__heading"><h3>{station.name}</h3><StatusBadge status={station.latestReading && station.latestReading.freshness === "FRESH" && station.latestReading.verificationStatus === "VERIFIED" ? "INFO" : "UNKNOWN"} /></div><div className="badge-row"><SourceBadge type={station.latestReading?.source.sourceType ?? station.source.sourceType} /><FreshnessBadge freshness={station.latestReading?.freshness ?? "UNKNOWN"} /></div><p className="data-card__meta">{station.latestReading?.source.name ?? station.source.name} · <DataTimestamp value={station.latestReading?.observedAt ?? null} /></p>{station.latestReading ? <p className="reading-values">{station.latestReading.waterLevelM !== null && <span>ระดับน้ำ {station.latestReading.waterLevelM} ม.</span>}{station.latestReading.flowRateM3s !== null && <span>อัตราการไหล {station.latestReading.flowRateM3s} ลบ.ม./วินาที</span>}{station.latestReading.rainfallMm !== null && <span>ฝน {station.latestReading.rainfallMm} มม.</span>}</p> : <p className="uncertainty">ยังไม่มีค่าที่สังเกตได้</p>}{station.latestReading && (station.latestReading.freshness !== "FRESH" || station.latestReading.verificationStatus !== "VERIFIED") && <p className="uncertainty">ค่าที่แสดงเป็นหลักฐานเก่าหรือยังไม่ผ่านการยืนยัน</p>}</li>)}</ul> : <p className="section-empty">ยังไม่มีสถานีในรายการที่เรียกดู</p>}</section>}
        {result.collections.events && <section className="data-section" aria-labelledby="events-title"><h2 id="events-title">เหตุการณ์น้ำ</h2><p className="section-caption">รายการล่าสุดสูงสุด 20 รายการ · สถานะมีผลยึดตามความใหม่และการตรวจสอบ</p>
          {result.collections.events.length ? <ul className="data-list">{result.collections.events.map((item) => <li className="data-card" key={item.id}><div className="data-card__heading"><h3>{item.title}</h3><StatusBadge status={effectiveStatus(item.effectiveSeverity, item.freshness, item.verificationStatus)} /></div><div className="badge-row"><SourceBadge type={item.source.sourceType} /><FreshnessBadge freshness={item.freshness} /><StatusBadge status={effectiveStatus(item.effectiveStatus, item.freshness, item.verificationStatus)} /></div><p className="data-card__meta">{item.source.name} · <DataTimestamp value={item.observedAt} /></p>{item.freshness !== "FRESH" || item.verificationStatus !== "VERIFIED" ? <p className="uncertainty">หลักฐานเก่าหรือยังไม่ผ่านการยืนยัน — ไม่ใช้สถานะเดิมยืนยันความปลอดภัย</p> : null}</li>)}</ul> : <p className="section-empty">ไม่มีเหตุการณ์ในรายการที่เรียกดู ไม่ได้แปลว่าไม่มีน้ำท่วมหรือปลอดภัย</p>}</section>}
        {result.collections.risks && <section className="data-section" aria-labelledby="risks-title"><h2 id="risks-title">พื้นที่ประเมินความเสี่ยง</h2><p className="section-caption">รายการล่าสุดสูงสุด 20 รายการ</p>
          {result.collections.risks.length ? <ul className="data-list">{result.collections.risks.map((item) => <li className="data-card" key={item.id}><div className="data-card__heading"><h3>{item.name}</h3><StatusBadge status={effectiveStatus(item.effectiveRiskLevel, item.freshness)} /></div><div className="badge-row"><SourceBadge type={item.source.sourceType} /><FreshnessBadge freshness={item.freshness} /></div><p className="data-card__meta">{item.source.name} · <DataTimestamp label="เริ่มมีผล" value={item.validFrom} /></p></li>)}</ul> : <p className="section-empty">ยังไม่มีผลประเมินพื้นที่ในรายการที่เรียกดู</p>}</section>}
        {result.collections.roads && <section className="data-section" aria-labelledby="roads-title"><h2 id="roads-title">สภาพถนน</h2><p className="section-caption">รายการล่าสุดสูงสุด 20 รายการ · ไม่แนะนำเส้นทางในระยะนี้</p>
          {result.collections.roads.length ? <ul className="data-list">{result.collections.roads.map((item) => <li className="data-card" key={item.id}><div className="data-card__heading"><h3>{item.name}</h3><StatusBadge status={effectiveStatus(item.currentCondition.status, item.currentCondition.freshness, item.currentCondition.verificationStatus)} /></div><div className="badge-row">{item.currentCondition.source && <SourceBadge type={item.currentCondition.source.sourceType} />}<FreshnessBadge freshness={item.currentCondition.freshness} /></div><p className="data-card__meta">{item.currentCondition.source?.name ?? "ไม่มีข้อมูลแหล่งที่มา"} · <DataTimestamp value={item.currentCondition.observedAt} /></p></li>)}</ul> : <p className="section-empty">ยังไม่มีข้อมูลถนนในรายการที่เรียกดู</p>}</section>}
      </div>}
    </>}
  </div>;
}
