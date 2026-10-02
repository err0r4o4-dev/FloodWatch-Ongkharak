import { EmptyState, StatusBadge } from "../../../shared/components/DataStates";
import { FeatureDetails } from "./FeatureDetails";
import { RoutePanel } from "./RoutePanel";
import type { Picking, RoutePoint, RoutePoints } from "./RoutePanel";
import { effectiveFloodSeverity, effectiveRiskLevel, effectiveRoadStatus } from "../utils";
import { layerKeys, layerLabels } from "../types";
import type { LayerItems, LayerKey, LayerVisibility, MapMode, MapSelection } from "../types";

export function MapSidePanel({ mode, items, visible, loading, failed, failedLayers, truncated, onRetry, selection, onSelect, onClose, points, onPointChange, picking, onPickingChange }: {
  mode: MapMode;
  items: LayerItems;
  visible: LayerVisibility;
  loading: boolean;
  failed: boolean;
  failedLayers: LayerKey[];
  truncated: LayerKey[];
  onRetry: () => void;
  selection: MapSelection | null;
  onSelect: (selection: MapSelection) => void;
  onClose: () => void;
  points: RoutePoints;
  onPointChange: (key: keyof RoutePoints, point: RoutePoint) => void;
  picking: Picking;
  onPickingChange: (key: Picking) => void;
}) {
  const selections: MapSelection[] = [
    ...items.flood.map((feature) => ({ layer: "flood" as const, feature })),
    ...items.water.map((feature) => ({ layer: "water" as const, feature })),
    ...items.risk.map((feature) => ({ layer: "risk" as const, feature })),
    ...items.roads.map((feature) => ({ layer: "roads" as const, feature }))
  ];
  return <aside className="map-side-panel" aria-label="ข้อมูลแผนที่และเส้นทาง">
    {selection && <FeatureDetails selection={selection} onClose={onClose} />}
    {mode === "ROUTE" && <RoutePanel points={points} onPointChange={onPointChange} picking={picking} onPickingChange={onPickingChange} items={items} />}
    <section aria-labelledby="visible-features-title"><h2 id="visible-features-title">ข้อมูลในขอบเขตที่โหลด</h2>
      <p>เลือกจากรายการนี้ได้ด้วยคีย์บอร์ด หรือคลิกสัญลักษณ์และพื้นที่บนแผนที่</p>
      {!failed && failedLayers.length > 0 && <p className="map-partial" role="status">ข้อมูลบางส่วนไม่พร้อมใช้งาน: {failedLayers.map((key) => layerLabels[key]).join(", ")} <button type="button" onClick={onRetry}>ลองอีกครั้ง</button></p>}
      {truncated.length > 0 && <p role="status">ข้อมูลเกิน 100 รายการสำหรับ {truncated.map((key) => layerLabels[key]).join(", ")} · ซูมเข้าเพื่อดูขอบเขตย่อย</p>}
      {loading ? <p role="status">กำลังโหลดข้อมูลตามขอบเขตแผนที่</p> : failed ? <div role="alert"><p>ไม่สามารถโหลดข้อมูลแผนที่ได้ · สถานะพื้นที่ไม่ทราบ</p><button type="button" className="action-button" onClick={onRetry}>ลองอีกครั้ง</button></div> : selections.length ? <ul className="map-feature-list">{selections.map((item) => {
        const name = item.layer === "flood" ? item.feature.title : item.feature.name;
        const status = item.layer === "roads" ? effectiveRoadStatus(item.feature.currentCondition) : item.layer === "flood" ? effectiveFloodSeverity(item.feature) : item.layer === "risk" ? effectiveRiskLevel(item.feature) : "UNKNOWN";
        return <li key={`${item.layer}-${item.feature.id}`}><button type="button" onClick={() => onSelect(item)}><span>{name}<small>{layerLabels[item.layer]}</small></span><StatusBadge status={status} /></button></li>;
      })}</ul> : <EmptyState title="ไม่มีข้อมูลในขอบเขตที่โหลด">ไม่มีรายการจากชั้นข้อมูลที่โหลดสำเร็จ ไม่ได้หมายความว่าพื้นที่ปลอดภัย</EmptyState>}
      <p className="map-layer-summary">{layerKeys.filter((key) => visible[key]).map((key) => `${layerLabels[key]} ${items[key].length}`).join(" · ") || "ปิดทุกชั้นข้อมูล"}</p>
    </section>
  </aside>;
}
