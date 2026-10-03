import { layerKeys, layerLabels } from "../types";
import type { LayerItems, LayerKey, LayerVisibility } from "../types";

export function MapDataStatus({ items, visible, loading, failed, failedLayers, truncated, onRetry }: {
  items: LayerItems;
  visible: LayerVisibility;
  loading: boolean;
  failed: boolean;
  failedLayers: LayerKey[];
  truncated: LayerKey[];
  onRetry: () => void;
}) {
  const count = Object.values(items).reduce((total, collection) => total + collection.length, 0);
  return <div className={`map-data-status${failed || failedLayers.length ? " map-data-status--unavailable" : ""}`}>
    <div className="map-data-status__label"><span aria-hidden="true">◎</span><strong>ข้อมูลในขอบเขตที่โหลด</strong></div>
    {loading ? <p role="status">กำลังโหลดข้อมูลตามขอบเขตแผนที่</p> : failed ? <p role="alert">ไม่สามารถโหลดข้อมูลแผนที่ได้ · สถานะพื้นที่ไม่ทราบ <button type="button" onClick={onRetry}>ลองอีกครั้ง</button></p> : <>
      {failedLayers.length > 0 && <p role="status">ข้อมูลบางส่วนไม่พร้อมใช้งาน: {failedLayers.map((key) => layerLabels[key]).join(", ")} <button type="button" onClick={onRetry}>ลองอีกครั้ง</button></p>}
      <p>{!Object.values(visible).some(Boolean) ? "ปิดทุกชั้นข้อมูล" : count ? `${count} รายการ · เลือกสัญลักษณ์เพื่อดูข้อมูล` : "ไม่มีรายการจากชั้นข้อมูลที่โหลดสำเร็จ ไม่ได้หมายความว่าพื้นที่ปลอดภัย"}</p>
    </>}
    {truncated.length > 0 && <p role="status">ข้อมูลเกิน 100 รายการสำหรับ {truncated.map((key) => layerLabels[key]).join(", ")} · ซูมเข้าเพื่อดูขอบเขตย่อย</p>}
    <span className="sr-only">{layerKeys.filter((key) => visible[key]).map((key) => `${layerLabels[key]} ${items[key].length}`).join(" · ")}</span>
  </div>;
}
