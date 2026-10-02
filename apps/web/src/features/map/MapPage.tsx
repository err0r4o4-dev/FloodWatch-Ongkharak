import { useState } from "react";
import { EmptyState } from "../../shared/components/DataStates";

type Mode = "situation" | "route";

export function MapPage() {
  const [mode, setMode] = useState<Mode>("situation");
  return <section className="feature-page map-page"><div className="page-heading"><p className="eyebrow">MAP PREVIEW</p><h1>แผนที่สถานการณ์</h1><p>โครงหน้าสำหรับแผนที่และเส้นทาง · ยังไม่แสดงภูมิศาสตร์หรือคำแนะนำการเดินทาง</p></div>
    <div className="map-layout"><div className="map-workspace"><div className="map-toolbar" role="group" aria-label="โหมดแผนที่"><button type="button" aria-pressed={mode === "situation"} onClick={() => setMode("situation")}>สถานการณ์</button><button type="button" aria-pressed={mode === "route"} onClick={() => setMode("route")}>เส้นทาง</button></div>
      <div className="map-placeholder"><span aria-hidden="true" className="map-placeholder__symbol">⌖</span><h2>{mode === "situation" ? "ยังไม่มีแผนที่สถานการณ์" : "ยังไม่สามารถค้นหาเส้นทาง"}</h2><p>{mode === "situation" ? "ชั้นข้อมูลเชิงพื้นที่และการโต้ตอบจะเปิดใน Phase 5" : "ไม่มีการคำนวณหรือยืนยันเส้นทางปลอดภัยในระยะนี้"}</p></div></div>
      <aside className="map-side-panel" aria-label="แผงควบคุมแผนที่"><h2>ชั้นข้อมูล</h2><p>เตรียมพื้นที่สำหรับชั้นข้อมูล ไม่มีชั้นข้อมูลที่เปิดใช้งาน</p><div className="layer-list"><button type="button" disabled>พื้นที่เสี่ยง · ยังไม่เปิดใช้</button><button type="button" disabled>สภาพถนน · ยังไม่เปิดใช้</button><button type="button" disabled>สถานีวัดน้ำ · ยังไม่เปิดใช้</button></div><EmptyState title="ไม่มีข้อมูลแผนที่">แผนที่แบบโต้ตอบและเส้นทางอยู่ใน Phase 5</EmptyState></aside></div>
  </section>;
}
