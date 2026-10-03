import { useEffect, useRef } from "react";
import { DataTimestamp, FreshnessBadge, SourceBadge, StatusBadge } from "../../../shared/components/DataStates";
import { WaterDetails } from "./FeatureDetails";
import { layerLabels } from "../types";
import type { MapSelection } from "../types";
import { effectiveFloodSeverity, effectiveRiskLevel, effectiveRoadStatus, selectionName } from "../utils";

export function FeaturePopup({ selection, onDetails, onClose }: {
  selection: MapSelection;
  onDetails: (selection: MapSelection) => void;
  onClose: () => void;
}) {
  const closeButton = useRef<HTMLButtonElement>(null);
  useEffect(() => {
    const previousFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    closeButton.current?.focus({ preventScroll: true });
    return () => { if (previousFocus?.isConnected) previousFocus.focus({ preventScroll: true }); };
  }, []);
  const source = selection.layer === "roads" ? selection.feature.currentCondition.source : selection.layer === "reports" ? null : selection.feature.source;
  const status = selection.layer === "flood" ? effectiveFloodSeverity(selection.feature)
    : selection.layer === "risk" ? effectiveRiskLevel(selection.feature)
      : selection.layer === "roads" ? effectiveRoadStatus(selection.feature.currentCondition) : selection.layer === "reports" ? "INFO" : "UNKNOWN";
  const freshness = selection.layer === "roads" ? selection.feature.currentCondition.freshness
    : selection.layer === "flood" || selection.layer === "risk" ? selection.feature.freshness : null;
  const observedAt = selection.layer === "flood" ? selection.feature.observedAt
    : selection.layer === "risk" ? selection.feature.validFrom
      : selection.layer === "roads" ? selection.feature.currentCondition.observedAt
        : selection.layer === "reports" ? selection.feature.verifiedAt : null;

  return <section className="map-popup" aria-label="ข้อมูลสรุปบนแผนที่" onKeyDown={(event) => {
    if (event.key === "Escape") { event.stopPropagation(); onClose(); }
  }}>
    <div className="map-popup__heading"><div><p className="map-popup__kind">{layerLabels[selection.layer]}</p><h3>{selectionName(selection)}</h3></div><button ref={closeButton} className="map-popup__close" type="button" aria-label="ปิด popup" onClick={onClose}>×</button></div>
    <div className="badge-row">{selection.layer !== "water" && <StatusBadge status={status} />}{freshness && <FreshnessBadge freshness={freshness} />}{selection.layer === "reports" ? <SourceBadge type="COMMUNITY" /> : source && <SourceBadge type={source.sourceType} />}</div>
    <p className="map-popup__source">{source?.name ?? (selection.layer === "reports" ? "รายงานจากชุมชน · ยืนยันแล้ว" : "ไม่ระบุแหล่งข้อมูล")}</p>
    {selection.layer === "water" ? <WaterDetails key={selection.feature.id} stationId={selection.feature.id} compact /> : <DataTimestamp label={selection.layer === "risk" ? "เริ่มมีผล" : selection.layer === "reports" ? "ยืนยันเมื่อ" : "เวลาสังเกต"} value={observedAt} />}
    {selection.layer === "risk" && <DataTimestamp label="สิ้นสุด" value={selection.feature.validTo} />}
    {selection.layer === "reports" && selection.feature.estimatedWaterDepthM !== null && <p>ระดับน้ำโดยประมาณ {selection.feature.estimatedWaterDepthM.toFixed(2)} ม.</p>}
    {status === "UNKNOWN" && selection.layer !== "water" && <p className="uncertainty">ข้อมูลไม่เพียงพอ ไม่สามารถสรุปว่าปลอดภัยหรือผ่านได้</p>}
    <button className="map-popup__details" type="button" onClick={() => onDetails(selection)}>ดูรายละเอียด <span aria-hidden="true">→</span></button>
  </section>;
}
