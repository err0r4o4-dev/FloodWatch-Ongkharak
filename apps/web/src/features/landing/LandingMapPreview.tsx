import { useCallback, useRef, useState } from "react";
import { Link } from "react-router";
import { StatusBadge } from "../../shared/components/DataStates";
import { FeatureDetails } from "../map/components/FeatureDetails";
import { MapLegend } from "../map/components/MapLegend";
import { SituationMap } from "../map/components/SituationMap";
import { useMapData } from "../map/hooks/useMapData";
import type { LayerVisibility, MapBounds, MapSelection } from "../map/types";
import { layerKeys } from "../map/types";
import { mapSelections, selectionName } from "../map/utils";
import { LandingIcon } from "./LandingIcon";

const visibility: LayerVisibility = { flood: true, water: true, risk: true, roads: true, reports: true };
const emptyPoint = { longitude: "", latitude: "" };

export function LandingMapPreview() {
  const [bounds, setBounds] = useState<MapBounds | null>(null);
  const [retry, setRetry] = useState(0);
  const [selection, setSelection] = useState<MapSelection | null>(null);
  const previousBounds = useRef<MapBounds | null>(null);
  const onBoundsChange = useCallback((next: MapBounds) => {
    const previous = previousBounds.current;
    if (previous && previous.west === next.west && previous.south === next.south && previous.east === next.east && previous.north === next.north) return;
    previousBounds.current = next;
    setBounds(next);
    setSelection(null);
  }, []);
  const data = useMapData(bounds, visibility, retry);
  const features = mapSelections(data.items);
  const allFailed = !data.loading && data.failed.length === layerKeys.length;
  const selected = selection && features.find((item) => item.layer === selection.layer && item.feature.id === selection.feature.id);

  return <section className="landing-map" aria-labelledby="landing-map-title">
    <div className="landing-map__card">
      <header className="landing-map__heading"><span className="landing-map__icon"><LandingIcon kind="map" /></span><div><h2 id="landing-map-title">แผนที่สถานการณ์น้ำ</h2><p>ดูพื้นที่เสี่ยง สถานีวัดน้ำ และเหตุการณ์ที่เผยแพร่</p></div><Link className="landing-map__expand" to="/map" aria-label="เปิดแผนที่เต็มหน้า"><LandingIcon kind="expand" /></Link></header>
      <div className="landing-map__canvas"><SituationMap items={data.items} visible={visibility} mode="SITUATION" points={{ origin: emptyPoint, destination: emptyPoint }} picking={null} onBoundsChange={onBoundsChange} onMapPoint={() => undefined} onSelect={setSelection} />
        {data.loading && <p className="landing-map__status" role="status">กำลังโหลดข้อมูลแผนที่</p>}
        {allFailed && <div className="landing-map__status landing-map__status--error" role="alert"><p>ไม่สามารถโหลดข้อมูลแผนที่ได้ · สถานะไม่ทราบ</p><button type="button" onClick={() => setRetry((previous) => previous + 1)}>ลองอีกครั้ง</button></div>}
        {!data.loading && !allFailed && data.failed.length > 0 && <p className="landing-map__status" role="status">ข้อมูลบางชั้นไม่พร้อมใช้งาน · ไม่สามารถสรุปว่าปลอดภัย</p>}
        {!data.loading && data.failed.length === 0 && features.length === 0 && <p className="landing-map__status" role="status">ยังไม่มีข้อมูลสถานการณ์ในขอบเขตนี้ · ไม่ทราบสถานะ</p>}
      </div>
      <div className="landing-map__risk-legend" aria-label="คำอธิบายระดับความเสี่ยง"><strong>ระดับความเสี่ยงพื้นที่</strong><div>{["LOW", "MODERATE", "HIGH", "VERY_HIGH", "UNKNOWN"].map((status) => <StatusBadge key={status} status={status} />)}</div></div>
    </div>
    <aside className="landing-weather" aria-label="ข้อมูลสภาพอากาศ"><span className="landing-weather__icon"><LandingIcon kind="cloud" /></span><div><strong>องครักษ์</strong><p>ข้อมูลสภาพอากาศยังไม่พร้อมใช้งาน</p><small>ยังไม่มีแหล่งข้อมูลอุณหภูมิที่เชื่อมต่อ</small></div></aside>
    <div className="landing-map__evidence"><details><summary>สัญลักษณ์และข้อมูลในแผนที่</summary><p>ข้อมูลไม่พร้อมใช้งานไม่ได้หมายถึงพื้นที่ปลอดภัย · ตรวจแหล่งข้อมูลและเวลาสังเกตในรายละเอียด</p><MapLegend visible={visibility} />{features.length > 0 && <ul>{features.slice(0, 5).map((item) => <li key={`${item.layer}-${item.feature.id}`}><button type="button" onClick={() => setSelection(item)}>{selectionName(item)}</button></li>)}</ul>}<Link to="/overview">ดูข้อมูลแบบรายการในหน้าภาพรวม →</Link></details></div>
    {data.truncated.length > 0 && <p className="landing-map__limit">แสดงข้อมูลบางส่วนในขอบเขตนี้ โปรดขยายแผนที่หรือเปิดแผนที่เต็มหน้าเพื่อดูพื้นที่ย่อย</p>}
    {selected && <FeatureDetails selection={selected} onClose={() => setSelection(null)} />}
  </section>;
}
