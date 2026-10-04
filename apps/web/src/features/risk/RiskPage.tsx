import type { ReactNode } from "react";
import { PageHeroArt } from "../../shared/components/PageHeroArt";
import { useCallback, useEffect, useMemo, useState } from "react";
import { DataTimestamp, EmptyState, ErrorState, FreshnessBadge, LoadingState, SourceBadge, StatusBadge } from "../../shared/components/DataStates";
import { FeatureDetails } from "../map/components/FeatureDetails";
import { MapLegend } from "../map/components/MapLegend";
import { SituationMap } from "../map/components/SituationMap";
import { useMapData } from "../map/hooks/useMapData";
import type { LayerVisibility, MapBounds, MapSelection } from "../map/types";
import { geometryBounds, validBounds, validGeometry } from "../map/utils";
import { loadCollection } from "../overview/api";
import type { RiskArea } from "../overview/api";
import "./RiskPage.css";

type RiskFilter = "ALL" | "VERY_HIGH" | "HIGH" | "MODERATE" | "LOW" | "UNKNOWN";
const riskMapVisibility: LayerVisibility = { flood: false, water: false, risk: true, roads: false, reports: false };
const emptyPoint = { longitude: "", latitude: "" };

const filters: { key: RiskFilter; label: string }[] = [
  { key: "ALL", label: "ทั้งหมด" },
  { key: "VERY_HIGH", label: "ความเสี่ยงสูงมาก" },
  { key: "HIGH", label: "ความเสี่ยงสูง" },
  { key: "MODERATE", label: "ความเสี่ยงปานกลาง" },
  { key: "LOW", label: "ความเสี่ยงต่ำ" },
  { key: "UNKNOWN", label: "ไม่ทราบสถานะ" }
];

function effectiveLevel(item: Pick<RiskArea, "effectiveRiskLevel" | "freshness">): string {
  return item.freshness === "FRESH" ? item.effectiveRiskLevel : "UNKNOWN";
}

function levelCount(items: RiskArea[], level: string) {
  return items.filter((item) => effectiveLevel(item) === level).length;
}

function RiskSummary({ items }: { items: RiskArea[] | null }) {
  const unavailable = !items?.length;
  const cards = [
    { 
      level: "VERY_HIGH", 
      label: "ความเสี่ยงสูงมาก", 
      description: "ตรวจสอบประกาศและหลีกเลี่ยงพื้นที่ที่ได้รับผลกระทบ",
      icon: "⚠",
      gradient: "linear-gradient(135deg, #fee2e2 0%, #fecaca 100%)",
      iconBg: "linear-gradient(135deg, #fca5a5 0%, #f87171 100%)",
      color: "#991b1b"
    },
    { 
      level: "HIGH", 
      label: "ความเสี่ยงสูง", 
      description: "ต้องตรวจสอบหลักฐานล่าสุดก่อนเดินทาง",
      icon: "!",
      gradient: "linear-gradient(135deg, #ffedd5 0%, #fed7aa 100%)",
      iconBg: "linear-gradient(135deg, #fdba74 0%, #fb923c 100%)",
      color: "#9a3412"
    },
    { 
      level: "MODERATE", 
      label: "ความเสี่ยงปานกลาง", 
      description: "ควรติดตามสถานการณ์และประกาศ",
      icon: "⚡",
      gradient: "linear-gradient(135deg, #fef9c3 0%, #fef08a 100%)",
      iconBg: "linear-gradient(135deg, #fde047 0%, #facc15 100%)",
      color: "#854d0e"
    },
    { 
      level: "LOW", 
      label: "ความเสี่ยงต่ำ", 
      description: "ไม่ใช่การยืนยันว่าพื้นที่ปลอดภัย",
      icon: "✓",
      gradient: "linear-gradient(135deg, #d1fae5 0%, #a7f3d0 100%)",
      iconBg: "linear-gradient(135deg, #6ee7b7 0%, #34d399 100%)",
      color: "#065f46"
    },
    { 
      level: "UNKNOWN", 
      label: "ไม่ทราบสถานะ", 
      description: "ข้อมูลเก่าหรือไม่พร้อมใช้งาน",
      icon: "?",
      gradient: "linear-gradient(135deg, #f1f5f9 0%, #e2e8f0 100%)",
      iconBg: "linear-gradient(135deg, #cbd5e1 0%, #94a3b8 100%)",
      color: "#475569"
    }
  ];

  return <section className="summary-metrics" aria-label="สรุประดับความเสี่ยงจากรายการที่เรียกดู">
    {cards.map((card) => {
      const count = unavailable ? "—" : levelCount(items, card.level);
      return <article 
        className={`summary-metric summary-metric--${card.level.toLowerCase()}`} 
        key={card.level}
        style={{ 
          background: card.gradient,
          borderColor: card.color + "40"
        }}
      >
        <span 
          className="summary-metric__icon"
          style={{ 
            background: card.iconBg,
            color: "white"
          }}
        >
          {card.icon}
        </span>
        <div className="summary-metric__content">
          <span className="summary-metric__label">{card.label}</span>
          <strong className="summary-metric__count" style={{ color: card.color }}>{count}</strong>
          <small className="summary-metric__description">{card.description}</small>
        </div>
      </article>;
    })}
  </section>;
}

function RiskCard({ item }: { item: RiskArea }) {
  const level = effectiveLevel(item);
  const stale = item.freshness !== "FRESH";
  const query = new URLSearchParams({ risk: item.id });
  if (validGeometry(item.geometry)) {
    const bounds = geometryBounds(item.geometry);
    if (validBounds(bounds)) query.set("bbox", [bounds.west, bounds.south, bounds.east, bounds.north].join(","));
  }

  return <li className="data-card risk-card">
    <div className="data-card__heading risk-card__heading">
      <div><h3>{item.name}</h3><p className="section-caption">{item.description || "ไม่มีคำอธิบายจากแหล่งข้อมูล"}</p></div>
      <StatusBadge status={level} />
    </div>
    <div className="badge-row"><SourceBadge type={item.source.sourceType} /><FreshnessBadge freshness={item.freshness} /></div>
    <p className="data-card__meta risk-card__source">{item.source.name}</p>
    <div className="risk-card__times"><DataTimestamp label="เริ่มมีผล" value={item.validFrom} /><DataTimestamp label="สิ้นสุด" value={item.validTo} /></div>
    {stale && <p className="uncertainty risk-card__uncertainty">ข้อมูลเก่าหรือไม่พร้อมใช้งาน จึงแสดงเป็นไม่ทราบสถานะและไม่ยืนยันว่าพื้นที่ปลอดภัย</p>}
    <a className="risk-card__map-link" href={`/map?${query}`} aria-label={`ดูบนแผนที่: ${item.name}`}>ดูบนแผนที่ <span aria-hidden="true">↗</span></a>
  </li>;
}

function RiskMapPanel() {
  const [bounds, setBounds] = useState<MapBounds | null>(null);
  const [selection, setSelection] = useState<MapSelection | null>(null);
  const [retry, setRetry] = useState(0);
  const data = useMapData(bounds, riskMapVisibility, retry);
  const onBoundsChange = useCallback((next: MapBounds) => { setBounds(next); setSelection(null); }, []);

  return <section className="data-section risk-map-panel overview-map" aria-labelledby="risk-map-title">
    <div className="overview-section-header risk-map-panel__heading"><div><h2 id="risk-map-title">แผนที่พื้นที่เสี่ยง</h2><p className="section-caption">ขอบเขตพื้นที่เสี่ยงจาก API ตามบริเวณที่กำลังดู</p></div><a className="risk-map-panel__link" href="/map">เปิดแผนที่พื้นที่เสี่ยง <span aria-hidden="true">→</span></a></div>
    <div className="overview-map__canvas risk-map-panel__canvas">
      <SituationMap items={data.items} visible={riskMapVisibility} mode="SITUATION" points={{ origin: emptyPoint, destination: emptyPoint }} picking={null} onBoundsChange={onBoundsChange} onMapPoint={() => undefined} onSelect={setSelection} />
      {data.loading && <p className="overview-map__status risk-map-panel__status" role="status">กำลังโหลดขอบเขตพื้นที่เสี่ยง</p>}
      {!data.loading && data.failed.length > 0 && <div className="overview-map__status overview-map__status--error risk-map-panel__status risk-map-panel__status--error" role="status">ไม่สามารถโหลดพื้นที่เสี่ยงบนแผนที่ได้ · สถานะไม่ทราบ <button type="button" onClick={() => setRetry((previous) => previous + 1)}>ลองโหลดแผนที่อีกครั้ง</button></div>}
    </div>
    <MapLegend visible={riskMapVisibility} />
    {selection && <FeatureDetails selection={selection} onClose={() => setSelection(null)} />}
    {data.truncated.length > 0 && <p className="risk-map-panel__note section-caption">ขอบเขตที่โหลดมีรายการเกินกำหนด โปรดขยายแผนที่เพื่อตรวจสอบพื้นที่ย่อย</p>}
    <p className="risk-map-panel__note section-caption">พื้นที่ที่ไม่มีข้อมูลหรือข้อมูลเก่าไม่ใช่การยืนยันว่าปลอดภัย</p>
  </section>;
}

export function RiskPage() {
  const [items, setItems] = useState<RiskArea[] | null>(null);
  const [failed, setFailed] = useState(false);
  const [retry, setRetry] = useState(0);
  const [filter, setFilter] = useState<RiskFilter>("ALL");

  useEffect(() => {
    const controller = new AbortController();
    setItems(null);
    setFailed(false);
    void loadCollection("risks", controller.signal)
      .then((nextItems) => { if (!controller.signal.aborted) setItems(nextItems); })
      .catch(() => { if (!controller.signal.aborted) setFailed(true); });
    return () => controller.abort();
  }, [retry]);

  const visibleItems = useMemo(() => items?.filter((item) => filter === "ALL" || effectiveLevel(item) === filter) ?? [], [filter, items]);
  const retryLoad = () => setRetry((value) => value + 1);
  const hasItems = items !== null && items.length > 0;
  let directoryContent: ReactNode;

  if (items === null) {
    directoryContent = failed ? <ErrorState onRetry={retryLoad} /> : <LoadingState />;
  } else if (items.length === 0) {
    directoryContent = <EmptyState title="ยังไม่มีพื้นที่เสี่ยงที่เผยแพร่">ยังไม่มีผลประเมินในรายการที่เรียกดู การไม่มีข้อมูลไม่ได้หมายความว่าไม่มีความเสี่ยงหรือปลอดภัย</EmptyState>;
  } else {
    directoryContent = visibleItems.length ? <ul className="data-list risk-list">{visibleItems.map((item) => <RiskCard item={item} key={item.id} />)}</ul> : <p className="section-empty risk-directory__empty">ไม่มีรายการ {filters.find((option) => option.key === filter)?.label} ในข้อมูลที่เรียกดู</p>;
  }

  return <div className="risk-page">
    <header className="page-hero">
      <div className="page-hero__copy">
        <p className="eyebrow">RISK AREAS · ONGKHARAK</p>
        <h1>พื้นที่เสี่ยงน้ำท่วม</h1>
        <p>ประเมินพื้นที่จากข้อมูลที่เผยแพร่พร้อมแหล่งที่มาและช่วงเวลาที่มีผล</p>
        <small>ข้อมูลที่ขาดหายหรือเก่าไม่ใช่หลักฐานว่าปลอดภัย แสดงสูงสุด 20 รายการล่าสุดจาก API สาธารณะ</small>
      </div>
      <PageHeroArt kind="risk" />
    </header>
    <RiskSummary items={items} />
    <div className="overview-sections risk-dashboard">
      <section className="data-section risk-directory" aria-labelledby="risk-directory-title">
        <div className="overview-section-header risk-directory__heading">
          <div><h2 id="risk-directory-title">รายการพื้นที่ประเมิน</h2><p className="section-caption">เลือกดูตามสถานะที่มีผลจากความใหม่ของข้อมูล</p></div>
          <p className="risk-directory__count" aria-live="polite">{hasItems ? `พบ ${visibleItems.length} รายการจากข้อมูลที่โหลด` : "—"}</p>
        </div>
        <div className="overview-map__layers risk-filter" role="group" aria-label="กรองระดับความเสี่ยง">
          {filters.map((option) => <button type="button" key={option.key} aria-pressed={filter === option.key} disabled={!hasItems} onClick={() => setFilter(option.key)}>{option.label}</button>)}
        </div>
        {directoryContent}
      </section>
      <RiskMapPanel />
    </div>
  </div>;
}
