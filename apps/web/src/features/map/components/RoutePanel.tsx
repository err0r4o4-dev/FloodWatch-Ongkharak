import type { LayerItems } from "../types";
import { effectiveRoadStatus, parseRoutePoint } from "../utils";
import { StatusBadge } from "../../../shared/components/DataStates";

export type RoutePoint = { longitude: string; latitude: string };
export type RoutePoints = { origin: RoutePoint; destination: RoutePoint };
export type Picking = keyof RoutePoints | null;

const labels: Record<keyof RoutePoints, string> = { origin: "จุดเริ่มต้น", destination: "จุดหมาย" };

export function RoutePanel({ points, onPointChange, picking, onPickingChange, items }: {
  points: RoutePoints;
  onPointChange: (key: keyof RoutePoints, point: RoutePoint) => void;
  picking: Picking;
  onPickingChange: (key: Picking) => void;
  items: LayerItems;
}) {
  const attentionRoads = items.roads.filter((road) => effectiveRoadStatus(road.currentCondition) !== "PASSABLE");
  return <section className="route-panel" aria-labelledby="route-title"><h2 id="route-title">วางแผนเส้นทาง</h2>
    <p className="route-unavailable" role="status">ยังไม่มีบริการคำนวณเส้นทาง ระบบไม่สามารถแนะนำหรือรับรองเส้นทางปลอดภัย</p>
    {(Object.keys(labels) as (keyof RoutePoints)[]).map((key) => <fieldset key={key} className="route-point"><legend>{labels[key]}</legend>
      <label>ลองจิจูด <input type="text" inputMode="decimal" value={points[key].longitude} onChange={(event) => onPointChange(key, { ...points[key], longitude: event.target.value })} placeholder="เช่น 101.0" /></label>
      <label>ละติจูด <input type="text" inputMode="decimal" value={points[key].latitude} onChange={(event) => onPointChange(key, { ...points[key], latitude: event.target.value })} placeholder="เช่น 14.1" /></label>
      {(points[key].longitude || points[key].latitude) && !parseRoutePoint(points[key]) && <p role="alert">กรุณาระบุลองจิจูด −180 ถึง 180 และละติจูด −90 ถึง 90</p>}
      <button type="button" aria-pressed={picking === key} onClick={() => onPickingChange(picking === key ? null : key)}>เลือก{labels[key]}บนแผนที่</button>
    </fieldset>)}
    {picking && <p role="status">เลือก{labels[picking]}ด้วยการแตะหรือคลิกบนแผนที่ หรือกรอกพิกัดในช่องด้านบน</p>}
    <div className="route-evidence"><h3>ข้อมูลถนนในขอบเขตแผนที่</h3><p>รายการนี้ไม่ใช่ถนนบนเส้นทาง และไม่ได้ตรวจการตัดผ่านกับเหตุการณ์น้ำ</p>
      {attentionRoads.length ? <ul>{attentionRoads.map((road) => <li key={road.id}>{road.name} <StatusBadge status={effectiveRoadStatus(road.currentCondition)} /></li>)}</ul> : <p>ไม่มีถนนที่มีข้อจำกัดหรือสถานะไม่ทราบในรายการที่โหลด · ไม่ได้หมายความว่าถนนผ่านได้</p>}
      <p>เหตุการณ์น้ำในขอบเขตที่โหลด: {items.flood.length} รายการ · ไม่มีการประเมินผลกระทบต่อเส้นทาง</p>
    </div>
  </section>;
}
