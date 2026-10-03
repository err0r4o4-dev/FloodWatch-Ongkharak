import { useCallback, useEffect, useRef, useState } from "react";
import { useSearchParams } from "react-router";
import { PageHeroArt } from "../../shared/components/PageHeroArt";
import { LayerControl } from "./components/LayerControl";
import { MapSidePanel } from "./components/MapSidePanel";
import { MapToolbar } from "./components/MapToolbar";
import { MapLegend } from "./components/MapLegend";
import { MapDataStatus } from "./components/MapDataStatus";
import { SituationMap } from "./components/SituationMap";
import { useMapData } from "./hooks/useMapData";
import { useRealtimeRevision } from "../realtime/RealtimeProvider";
import { layerKeys } from "./types";
import { validBounds } from "./utils";
import type { LayerKey, LayerVisibility, MapBounds, MapMode, MapSelection, Position } from "./types";
import type { Picking, RoutePoint, RoutePoints } from "./components/RoutePanel";
import "./MapPage.css";

const defaultVisibility: LayerVisibility = { flood: true, water: true, risk: true, roads: true, reports: true };
const emptyPoint: RoutePoint = { longitude: "", latitude: "" };
const mapEvents = ["water_level.updated", "flood_event.created", "flood_event.updated", "road.status.changed", "report.verified"] as const;

function parseInitialBounds(value: string | null): MapBounds | undefined {
  if (!value) return undefined;
  const coordinates = value.split(",").map((part) => part.trim() ? Number(part) : NaN);
  if (coordinates.length !== 4) return undefined;
  const [west, south, east, north] = coordinates;
  const bounds = { west, south, east, north };
  return validBounds(bounds) ? bounds : undefined;
}

export function MapPage() {
  const [params] = useSearchParams();
  const initialBounds = parseInitialBounds(params.get("bbox"));
  const initialRiskId = params.get("risk");
  const [mode, setMode] = useState<MapMode>("SITUATION");
  const [visible, setVisible] = useState<LayerVisibility>(defaultVisibility);
  const [bounds, setBounds] = useState<MapBounds | null>(null);
  const [selection, setSelection] = useState<MapSelection | null>(null);
  const [focusSelection, setFocusSelection] = useState<MapSelection | null>(null);
  const [listOpen, setListOpen] = useState(false);
  const [riskOpen, setRiskOpen] = useState(params.get("panel") === "risk");
  const [selectedRiskId, setSelectedRiskId] = useState<string | null>(initialRiskId);
  const [expanded, setExpanded] = useState(false);
  const listButton = useRef<HTMLButtonElement>(null);
  const riskButton = useRef<HTMLButtonElement>(null);
  const expandButton = useRef<HTMLButtonElement>(null);
  const workspace = useRef<HTMLElement>(null);
  const [retry, setRetry] = useState(0);
  const realtimeRevision = useRealtimeRevision(mapEvents);
  const [points, setPoints] = useState<RoutePoints>({ origin: emptyPoint, destination: emptyPoint });
  const [picking, setPicking] = useState<Picking>(null);
  const data = useMapData(bounds, visible, retry + realtimeRevision);
  const enabled = layerKeys.filter((key) => visible[key]);
  const count = Object.values(data.items).reduce((total, collection) => total + collection.length, 0);
  const allFailed = !data.loading && enabled.length > 0 && data.failed.length === enabled.length;
  const panelOpen = listOpen || riskOpen || selection !== null || mode === "ROUTE";
  const onBoundsChange = useCallback((next: MapBounds) => {
    setBounds((previous) => previous && previous.west === next.west && previous.south === next.south && previous.east === next.east && previous.north === next.north ? previous : next);
    setSelection(null);
  }, []);
  const onMapPoint = useCallback((position: Position) => {
    if (mode !== "ROUTE" || !picking) return;
    setPoints((previous) => ({ ...previous, [picking]: { longitude: position[0].toFixed(6), latitude: position[1].toFixed(6) } }));
    setPicking(null);
  }, [mode, picking]);
  const toggle = (key: LayerKey) => {
    setVisible((previous) => ({ ...previous, [key]: !previous[key] }));
    if (key === "risk") { setRiskOpen(false); setSelectedRiskId(null); }
    if (selection?.layer === key) setSelection(null);
    if (focusSelection?.layer === key) setFocusSelection(null);
  };
  const dismissPanel = () => {
    setSelection(null);
    setListOpen(false);
    setRiskOpen(false);
    setPicking(null);
    setMode("SITUATION");
    (riskOpen ? riskButton : listButton).current?.focus();
  };
  useEffect(() => {
    if (params.get("panel") !== "risk") return;
    setRiskOpen(true);
    setListOpen(false);
    setMode("SITUATION");
    setVisible((previous) => ({ ...previous, risk: true }));
    setSelectedRiskId(params.get("risk"));
  }, [params]);
  useEffect(() => {
    if (!expanded) return;
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    expandButton.current?.focus();
    return () => { document.body.style.overflow = previousOverflow; expandButton.current?.focus(); };
  }, [expanded]);

  return <section ref={workspace} className={`feature-page map-page${expanded ? " is-expanded" : ""}`} role={expanded ? "dialog" : undefined} aria-modal={expanded || undefined} aria-labelledby={expanded ? "situation-map-title" : undefined} onKeyDown={(event) => {
    if (!expanded) return;
    if (event.key === "Escape") { event.preventDefault(); setExpanded(false); }
    if (event.key === "Tab") {
      const controls = Array.from(workspace.current?.querySelectorAll<HTMLElement>('button:not([disabled]), input, summary, a[href], [tabindex="0"]') ?? []).filter((element) => element.getClientRects().length > 0);
      const first = controls[0];
      const last = controls[controls.length - 1];
      if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last?.focus(); }
      else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first?.focus(); }
    }
  }}>
    <header className="page-hero map-page__hero">
      <div className="page-hero__copy">
        <p className="eyebrow">SITUATION MAP · ONGKHARAK</p>
        <h1 id="situation-map-title">แผนที่สถานการณ์น้ำ</h1>
        <p>สำรวจสถานีวัดน้ำ พื้นที่เสี่ยง และรายงานจากชุมชนในแผนที่เดียว</p>
      </div>
      <PageHeroArt kind="map" />
    </header>
    <div className={`map-page__workspace${panelOpen ? " has-panel" : ""}`}>
      <SituationMap key={`${initialRiskId ?? ""}:${params.get("bbox") ?? ""}`} items={data.items} visible={visible} mode={mode} points={points} picking={picking} onBoundsChange={onBoundsChange} onMapPoint={onMapPoint} onSelect={setSelection} onPreview={(item) => { setSelectedRiskId(item?.layer === "risk" ? item.feature.id : null); }} enablePopups focusSelection={focusSelection} initialBounds={initialBounds} initialRiskId={initialRiskId} />
      <div className="map-page__layers"><LayerControl visible={visible} onToggle={toggle} /></div>
      <div className="map-page__tools"><MapToolbar mode={mode} onModeChange={(next) => { setMode(next); setPicking(null); setSelection(null); setRiskOpen(false); }} /><button ref={riskButton} type="button" className="map-tool-button" aria-label="พื้นที่เสี่ยง" aria-expanded={riskOpen} aria-controls="map-information-panel" onClick={() => { setRiskOpen((previous) => !previous); setListOpen(false); setMode("SITUATION"); setPicking(null); setSelection(null); setVisible((previous) => ({ ...previous, risk: true })); }}><span aria-hidden="true">◇</span><span>พื้นที่เสี่ยง</span></button><button ref={listButton} type="button" className="map-tool-button map-tool-button--list" aria-expanded={listOpen} aria-controls="map-information-panel" onClick={() => { setListOpen((previous) => !previous); setRiskOpen(false); }}><span aria-hidden="true">☷</span><span>รายการในขอบเขต</span><span className="map-tool-button__count">{data.loading ? "…" : count}</span></button><button ref={expandButton} type="button" className="map-tool-button map-tool-button--square" aria-label={expanded ? "ออกจากแผนที่เต็มหน้าจอ" : "แสดงแผนที่เต็มหน้าจอ"} title={expanded ? "ออกจากแผนที่เต็มหน้าจอ" : "แสดงแผนที่เต็มหน้าจอ"} aria-pressed={expanded} onClick={() => setExpanded((previous) => !previous)}><span aria-hidden="true">⛶</span></button></div>
      {panelOpen && <MapSidePanel mode={mode} items={data.items} visible={visible} loading={data.loading} failed={allFailed} failedLayers={data.failed} truncated={data.truncated} onRetry={() => setRetry((previous) => previous + 1)} selection={selection} onSelect={setSelection} onClose={() => { setSelection(null); if (!listOpen && !riskOpen && mode === "SITUATION") workspace.current?.querySelector<HTMLElement>(".leaflet-map")?.focus({ preventScroll: true }); }} onDismiss={dismissPanel} showList={listOpen} showRisk={riskOpen} selectedRiskId={selectedRiskId} onRiskSelect={(item) => { setSelectedRiskId(item.feature.id); setFocusSelection({ ...item }); }} points={points} onPointChange={(key, point) => setPoints((previous) => ({ ...previous, [key]: point }))} picking={picking} onPickingChange={setPicking} />}
      <div className="map-page__legend"><MapLegend visible={visible} /></div>
    </div>
    <MapDataStatus items={data.items} visible={visible} loading={data.loading} failed={allFailed} failedLayers={data.failed} truncated={data.truncated} onRetry={() => setRetry((previous) => previous + 1)} />
    {initialRiskId && !data.loading && !data.items.risk.some((item) => item.id === initialRiskId) && <p className="map-page__caution" role="status">ไม่พบพื้นที่ที่เลือกในข้อมูลของขอบเขตนี้ อาจไม่พร้อมใช้งานหรือเกินจำนวนรายการที่โหลด</p>}
    <p className="map-page__caution">ข้อมูลที่ไม่พร้อมใช้งานหรือเก่าไม่ใช่การยืนยันว่าพื้นที่ปลอดภัย · เลือกสัญลักษณ์เพื่อดูแหล่งที่มาและเวลาของข้อมูล</p>
  </section>;
}
