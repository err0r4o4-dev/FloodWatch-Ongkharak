import { layerKeys, layerLabels } from "../types";
import type { LayerKey, LayerVisibility } from "../types";

export function LayerControl({ visible, onToggle }: { visible: LayerVisibility; onToggle: (key: LayerKey) => void }) {
  return <fieldset className="map-layer-control"><legend>ชั้นข้อมูลที่แสดง</legend>
    <div className="map-layer-options">{layerKeys.map((key) => <label key={key}><input type="checkbox" checked={visible[key]} onChange={() => onToggle(key)} /><span>{layerLabels[key]}</span></label>)}</div>
    <p>กล้อง CCTV · รายงานชุมชน · จุดปลอดภัย · ขอบเขตการปกครอง: ยังไม่มี API ชั้นข้อมูล</p>
  </fieldset>;
}
