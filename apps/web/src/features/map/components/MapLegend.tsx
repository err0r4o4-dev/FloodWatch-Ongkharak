import type { LayerVisibility } from "../types";

export function MapLegend({ visible }: { visible: LayerVisibility }) {
  return <p className="map-legend">คำอธิบาย: {[
    visible.flood && "● เหตุการณ์น้ำ",
    visible.water && "◯ สถานีวัดน้ำ",
    visible.risk && "▧ พื้นที่เสี่ยง",
    visible.roads && "━ สภาพถนน · เส้นประ/เทา = ไม่ทราบสถานะ"
  ].filter(Boolean).join(" · ") || "ปิดทุกชั้นข้อมูล"}</p>;
}
