import { useCallback, useState } from "react";
import { Link } from "react-router";
import { FeatureDetails } from "../map/components/FeatureDetails";
import { MapLegend } from "../map/components/MapLegend";
import { SituationMap } from "../map/components/SituationMap";
import { useMapData } from "../map/hooks/useMapData";
import { layerKeys, layerLabels } from "../map/types";
import type { LayerKey, LayerVisibility, MapBounds, MapSelection } from "../map/types";

const initialVisibility: LayerVisibility = { flood: true, water: true, risk: true, roads: true, reports: true };
const emptyPoint = { longitude: "", latitude: "" };

export function OverviewMapPreview() {
  const [bounds, setBounds] = useState<MapBounds | null>(null);
  const [visible, setVisible] = useState<LayerVisibility>(initialVisibility);
  const [selection, setSelection] = useState<MapSelection | null>(null);
  const [retry, setRetry] = useState(0);
  const data = useMapData(bounds, visible, retry);
  const onBoundsChange = useCallback((next: MapBounds) => { setBounds(next); setSelection(null); }, []);
  const enabled = layerKeys.filter((key) => visible[key]);
  const allFailed = !data.loading && enabled.length > 0 && data.failed.length === enabled.length;
  const toggle = (key: LayerKey) => {
    setVisible((previous) => ({ ...previous, [key]: !previous[key] }));
    if (selection?.layer === key) setSelection(null);
  };

  return <section className="data-section overview-map" aria-labelledby="overview-map-title">
    <div className="overview-section-header"><div><h2 id="overview-map-title">แผนที่สถานการณ์น้ำ</h2><p className="section-caption">ข้อมูลเชิงพื้นที่ตามขอบเขตที่กำลังดู · ไม่ใช่การยืนยันว่าพื้นที่ปลอดภัย</p></div><Link to="/map">ดูแผนที่เต็มหน้า →</Link></div>
    <div className="overview-map__layers" role="group" aria-label="ชั้นข้อมูลแผนที่ภาพรวม">{layerKeys.map((key) => <button key={key} type="button" aria-pressed={visible[key]} onClick={() => toggle(key)}>{layerLabels[key]}</button>)}</div>
    <div className="overview-map__canvas"><SituationMap items={data.items} visible={visible} mode="SITUATION" points={{ origin: emptyPoint, destination: emptyPoint }} picking={null} onBoundsChange={onBoundsChange} onMapPoint={() => undefined} onSelect={setSelection} />
      {data.loading && <p className="overview-map__status" role="status">กำลังโหลดชั้นข้อมูลแผนที่</p>}
      {allFailed && <div className="overview-map__status overview-map__status--error" role="alert">ไม่สามารถโหลดชั้นข้อมูลแผนที่ได้ · สถานะไม่ทราบ <button type="button" onClick={() => setRetry((previous) => previous + 1)}>ลองอีกครั้ง</button></div>}
      {!allFailed && data.failed.length > 0 && <p className="overview-map__status overview-map__status--error" role="status">ชั้นข้อมูลบางส่วนไม่พร้อมใช้งาน · ไม่ถือว่าปลอดภัย</p>}
    </div>
    <MapLegend visible={visible} />
    {selection && <FeatureDetails selection={selection} onClose={() => setSelection(null)} />}
    {data.truncated.length > 0 && <p className="section-caption">รายการบางชั้นข้อมูลเกินขอบเขตการโหลด โปรดขยายแผนที่เพื่อตรวจสอบพื้นที่ย่อย</p>}
  </section>;
}
