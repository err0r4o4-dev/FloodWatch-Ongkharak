import { useRef, useState } from "react";
import { layerLabels } from "../types";
import type { LayerItems, MapSelection } from "../types";
import { mapSelections, selectionName } from "../utils";

export function MapSearch({ items, loading, onSelect }: { items: LayerItems; loading: boolean; onSelect: (selection: MapSelection) => void }) {
  const [query, setQuery] = useState("");
  const input = useRef<HTMLInputElement>(null);
  const results = mapSelections(items).filter((item) => (item.layer === "water" || item.layer === "risk") && selectionName(item).toLocaleLowerCase("th").includes(query.trim().toLocaleLowerCase("th")));
  return <div className="map-search" role="search" onKeyDown={(event) => { if (event.key === "Escape") { setQuery(""); input.current?.focus(); } }}>
    <div className="map-search__input"><span aria-hidden="true">⌕</span><input ref={input} type="search" aria-label="ค้นหาสถานีหรือพื้นที่ในขอบเขตที่โหลด" aria-describedby="map-search-scope" placeholder="ค้นหาสถานี หรือพื้นที่เสี่ยง…" value={query} onChange={(event) => setQuery(event.target.value)} />{query && <button type="button" aria-label="ล้างคำค้นหา" onClick={() => { setQuery(""); input.current?.focus(); }}>×</button>}</div>
    <span className="sr-only" id="map-search-scope">ค้นหาเฉพาะสถานีและพื้นที่ในขอบเขตที่โหลด</span>
    {query.trim() && <div className="map-search__results"><p>ค้นหาเฉพาะสถานีและพื้นที่ในขอบเขตที่โหลด</p>{loading ? <p role="status">กำลังโหลดข้อมูลเพื่อค้นหา</p> : results.length ? <ul>{results.map((selection) => <li key={`${selection.layer}-${selection.feature.id}`}><button type="button" onClick={() => { onSelect(selection); setQuery(""); input.current?.focus(); }}><strong>{selectionName(selection)}</strong><small>{layerLabels[selection.layer]}</small></button></li>)}</ul> : <p role="status">ไม่พบสถานีหรือพื้นที่ในข้อมูลที่โหลด</p>}</div>}
  </div>;
}
