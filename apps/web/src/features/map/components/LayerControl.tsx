import { useEffect, useState } from "react";
import { layerKeys, layerLabels } from "../types";
import type { LayerKey, LayerVisibility } from "../types";

export function LayerControl({ visible, onToggle }: { visible: LayerVisibility; onToggle: (key: LayerKey) => void }) {
  const [open, setOpen] = useState(() => typeof window.matchMedia === "function" ? !window.matchMedia("(max-width: 55rem)").matches : true);
  useEffect(() => {
    if (typeof window.matchMedia !== "function") return;
    const media = window.matchMedia("(max-width: 55rem)");
    const update = () => setOpen(!media.matches);
    media.addEventListener("change", update);
    return () => media.removeEventListener("change", update);
  }, []);
  return <details className="map-layer-menu" open={open} onToggle={(event) => setOpen(event.currentTarget.open)}><summary><span aria-hidden="true">▱</span> ชั้นข้อมูล <span className="map-layer-menu__count">{layerKeys.filter((key) => visible[key]).length}/{layerKeys.length}</span></summary><fieldset className="map-layer-control"><legend className="sr-only">ชั้นข้อมูลที่แสดง</legend>
    <div className="map-layer-options">{layerKeys.map((key) => <label key={key}><input type="checkbox" checked={visible[key]} onChange={() => onToggle(key)} /><span className={`map-layer-symbol map-layer-symbol--${key}`} aria-hidden="true">{key === "water" ? "≋" : key === "risk" || key === "flood" ? "△" : key === "roads" ? "━" : "●"}</span><span>{layerLabels[key]}</span></label>)}</div>
  </fieldset></details>;
}
