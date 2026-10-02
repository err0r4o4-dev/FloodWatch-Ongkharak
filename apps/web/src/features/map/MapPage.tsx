import { useCallback, useState } from "react";
import { LayerControl } from "./components/LayerControl";
import { MapSidePanel } from "./components/MapSidePanel";
import { MapToolbar } from "./components/MapToolbar";
import { MapLegend } from "./components/MapLegend";
import { SituationMap } from "./components/SituationMap";
import { useMapData } from "./hooks/useMapData";
import { useRealtimeRevision } from "../realtime/RealtimeProvider";
import { layerKeys } from "./types";
import type { LayerKey, LayerVisibility, MapBounds, MapMode, MapSelection, Position } from "./types";
import type { Picking, RoutePoint, RoutePoints } from "./components/RoutePanel";
import "./MapPage.css";

const defaultVisibility: LayerVisibility = { flood: true, water: true, risk: true, roads: true, reports: true };
const emptyPoint: RoutePoint = { longitude: "", latitude: "" };
const mapEvents = ["water_level.updated", "flood_event.created", "flood_event.updated", "road.status.changed", "report.verified"] as const;

export function MapPage() {
  const [mode, setMode] = useState<MapMode>("SITUATION");
  const [visible, setVisible] = useState<LayerVisibility>(defaultVisibility);
  const [bounds, setBounds] = useState<MapBounds | null>(null);
  const [selection, setSelection] = useState<MapSelection | null>(null);
  const [retry, setRetry] = useState(0);
  const realtimeRevision = useRealtimeRevision(mapEvents);
  const [points, setPoints] = useState<RoutePoints>({ origin: emptyPoint, destination: emptyPoint });
  const [picking, setPicking] = useState<Picking>(null);
  const data = useMapData(bounds, visible, retry + realtimeRevision);
  const enabled = layerKeys.filter((key) => visible[key]);
  const allFailed = !data.loading && enabled.length > 0 && data.failed.length === enabled.length;
  const onBoundsChange = useCallback((next: MapBounds) => { setBounds(next); setSelection(null); }, []);
  const onMapPoint = useCallback((position: Position) => {
    if (mode !== "ROUTE" || !picking) return;
    setPoints((previous) => ({ ...previous, [picking]: { longitude: position[0].toFixed(6), latitude: position[1].toFixed(6) } }));
    setPicking(null);
  }, [mode, picking]);
  const toggle = (key: LayerKey) => {
    setVisible((previous) => ({ ...previous, [key]: !previous[key] }));
    if (selection?.layer === key) setSelection(null);
  };

  return <section className="feature-page map-page">
    <header className="map-page__hero">
      <div className="page-heading"><p className="eyebrow">SITUATION MAP · ONGKHARAK</p><h1>แผนที่สถานการณ์น้ำ</h1><p>ข้อมูลเชิงพื้นที่จาก API สาธารณะตามขอบเขตที่เห็น พร้อมแหล่งที่มาและความใหม่ของหลักฐาน</p></div>
      <p className="map-page__caution">ข้อมูลที่ไม่พร้อมใช้งานหรือเก่าไม่ใช่การยืนยันว่าพื้นที่ปลอดภัย</p>
    </header>
    <div className="map-page__workspace">
      <div className="map-primary-controls map-page__controls"><MapToolbar mode={mode} onModeChange={(next) => { setMode(next); setPicking(null); setSelection(null); }} /><LayerControl visible={visible} onToggle={toggle} /></div>
      <div className="map-layout map-page__layout"><div className="map-workspace">
        <SituationMap items={data.items} visible={visible} mode={mode} points={points} picking={picking} onBoundsChange={onBoundsChange} onMapPoint={onMapPoint} onSelect={setSelection} />
        <MapLegend visible={visible} />
      </div><MapSidePanel mode={mode} items={data.items} visible={visible} loading={data.loading} failed={allFailed} failedLayers={data.failed} truncated={data.truncated} onRetry={() => setRetry((previous) => previous + 1)} selection={selection} onSelect={setSelection} onClose={() => setSelection(null)} points={points} onPointChange={(key, point) => setPoints((previous) => ({ ...previous, [key]: point }))} picking={picking} onPickingChange={setPicking} /></div>
    </div>
  </section>;
}
