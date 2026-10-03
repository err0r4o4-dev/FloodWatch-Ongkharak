import type { LayerVisibility } from "../types";

export function MapLegend({ visible }: { visible: LayerVisibility }) {
  return <div className="map-legend" aria-label="คำอธิบายสัญลักษณ์"><span className="sr-only">คำอธิบาย:</span>
    {visible.water && <span><i className="map-layer-symbol map-layer-symbol--water" aria-hidden="true">≋</i> สถานีวัดน้ำ</span>}
    {visible.risk && <span><i className="map-layer-symbol map-layer-symbol--risk" aria-hidden="true">▧</i> พื้นที่เสี่ยง</span>}
    {visible.flood && <span><i className="map-layer-symbol map-layer-symbol--flood" aria-hidden="true">△</i> เหตุการณ์น้ำ</span>}
    {visible.reports && <span><i className="map-layer-symbol map-layer-symbol--reports" aria-hidden="true">●</i> รายงานชุมชนที่ยืนยันแล้ว</span>}
    {visible.roads && <span><i className="map-layer-symbol map-layer-symbol--roads" aria-hidden="true">━</i> สภาพถนน · เส้นประ/เทา = ไม่ทราบสถานะ</span>}
    {!Object.values(visible).some(Boolean) && <span>ปิดทุกชั้นข้อมูล</span>}
  </div>;
}
