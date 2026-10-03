import { useState } from "react";
import { FreshnessBadge, SourceBadge, StatusBadge } from "../../../shared/components/DataStates";
import type { MapSelection, RiskFeature } from "../types";
import { effectiveRiskLevel } from "../utils";

const levels = [
  { key: "ALL", label: "ทั้งหมด" },
  { key: "VERY_HIGH", label: "ความเสี่ยงสูงมาก" },
  { key: "HIGH", label: "ความเสี่ยงสูง" },
  { key: "MODERATE", label: "ความเสี่ยงปานกลาง" },
  { key: "LOW", label: "ความเสี่ยงต่ำ" },
  { key: "UNKNOWN", label: "ไม่ทราบสถานะ" }
];

export function RiskDirectory({ items, loading, failed, truncated, selectedId, onSelect, onRetry }: {
  items: RiskFeature[];
  loading: boolean;
  failed: boolean;
  truncated: boolean;
  selectedId: string | null;
  onSelect: (selection: MapSelection) => void;
  onRetry: () => void;
}) {
  const [filter, setFilter] = useState("ALL");
  const filtered = items.filter((item) => filter === "ALL" || effectiveRiskLevel(item) === filter);
  const ready = !loading && !failed;
  return <section className="map-risk-directory" aria-labelledby="map-risk-title">
    <h2 id="map-risk-title">พื้นที่เสี่ยงน้ำท่วม</h2>
    <p>พื้นที่ในบริเวณแผนที่ที่กำลังดู</p>
    <div className="map-risk-filters" role="group" aria-label="กรองรายการระดับความเสี่ยง">
      {levels.map(({ key, label }) => <button key={key} type="button" aria-label={`กรอง: ${label}`} aria-pressed={filter === key} disabled={!ready} onClick={() => setFilter(key)}><span>{label}</span><strong>{ready ? (key === "ALL" ? items.length : items.filter((item) => effectiveRiskLevel(item) === key).length) : "—"}</strong></button>)}
    </div>
    <p className="map-risk-directory__note">ตัวกรองใช้กับรายการเท่านั้น แผนที่ยังแสดงทุกระดับในชั้นพื้นที่เสี่ยง</p>
    {loading ? <p role="status">กำลังโหลดพื้นที่เสี่ยงตามขอบเขตแผนที่</p> : failed ? <div role="alert"><p>ไม่สามารถโหลดพื้นที่เสี่ยงได้ · สถานะไม่ทราบ</p><button className="action-button" type="button" onClick={onRetry}>ลองโหลดพื้นที่เสี่ยงอีกครั้ง</button></div> : items.length === 0 ? <p role="status">ไม่มีพื้นที่เสี่ยงในข้อมูลที่โหลด ไม่ได้หมายความว่าปลอดภัย</p> : filtered.length === 0 ? <p role="status">ไม่มีรายการระดับนี้ในข้อมูลที่โหลด</p> : <ul className="map-feature-list map-risk-list">{filtered.map((item) => <li key={item.id}><button type="button" aria-pressed={selectedId === item.id} onClick={() => onSelect({ layer: "risk", feature: item })}><span><strong>{item.name}</strong><span className="map-risk-list__badges"><StatusBadge status={effectiveRiskLevel(item)} /><FreshnessBadge freshness={item.freshness} /><SourceBadge type={item.source.sourceType} /></span><small>{item.source.name}</small></span><span aria-hidden="true">↗</span></button></li>)}</ul>}
    {truncated && <p role="status">พื้นที่เสี่ยงเกิน 100 รายการ · จำนวนสรุปนับเฉพาะรายการที่โหลด โปรดซูมเข้าเพื่อดูขอบเขตย่อย</p>}
    <p className="map-risk-directory__note">ข้อมูลเก่าหรือไม่พร้อมใช้งานเป็นไม่ทราบสถานะ ความเสี่ยงต่ำไม่ใช่การยืนยันว่าปลอดภัย</p>
  </section>;
}
