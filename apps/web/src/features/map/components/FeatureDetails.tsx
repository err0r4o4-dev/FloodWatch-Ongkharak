import { useEffect, useRef, useState } from "react";
import { DataTimestamp, FreshnessBadge, SourceBadge, StatusBadge } from "../../../shared/components/DataStates";
import { ReportMediaImage } from "../../reports/ReportMediaImage";
import { fetchLatestReading } from "../api";
import type { MapSelection, WaterReading } from "../types";
import { effectiveFloodSeverity, effectiveRiskLevel, effectiveRoadStatus } from "../utils";

const verificationLabels: Record<string, string> = {
  VERIFIED: "ตรวจสอบแล้ว", UNVERIFIED: "ยังไม่ตรวจสอบ", REJECTED: "ไม่ผ่านการตรวจสอบ"
};

function Verification({ value, confidence }: { value?: string; confidence?: number | null }) {
  return <p className="map-evidence">การตรวจสอบ: {value ? verificationLabels[value] ?? "ไม่ทราบ" : "API ไม่ระบุ"} · ความเชื่อมั่น: {confidence == null ? "ไม่ระบุ" : `${Math.round(confidence * 100)}%`}</p>;
}

export function WaterDetails({ stationId, compact = false }: { stationId: string; compact?: boolean }) {
  const [result, setResult] = useState<{ state: "loading" | "error" | "ready"; reading: WaterReading | null }>({ state: "loading", reading: null });
  useEffect(() => {
    const controller = new AbortController();
    setResult({ state: "loading", reading: null });
    void fetchLatestReading(stationId, controller.signal).then((reading) => {
      if (!controller.signal.aborted) setResult({ state: "ready", reading });
    }).catch(() => { if (!controller.signal.aborted) setResult({ state: "error", reading: null }); });
    return () => controller.abort();
  }, [stationId]);
  if (result.state === "loading") return <p role="status">กำลังโหลดค่าที่สังเกตล่าสุด</p>;
  if (result.state === "error") return <p role="alert">ไม่สามารถโหลดค่าที่สังเกตได้ · สถานะไม่ทราบ</p>;
  if (!result.reading) return <p>ยังไม่มีค่าที่สังเกตได้ · ไม่ทราบสถานะ</p>;
  const reading = result.reading;
  if (compact) return <div className="map-reading">
    <p className="map-reading__value">{reading.waterLevelM != null ? `ระดับน้ำ ${reading.waterLevelM} ม.` : "ไม่ระบุระดับน้ำ"}</p>
    <div className="badge-row"><SourceBadge type={reading.source.sourceType} /><FreshnessBadge freshness={reading.freshness} /><StatusBadge status="UNKNOWN" /></div>
    <p>แหล่งค่าที่สังเกต: {reading.source.name}</p><DataTimestamp value={reading.observedAt} />
    <small>ค่าที่รายงาน ไม่ใช่ระดับความปลอดภัย</small>
  </div>;
  return <div className="map-reading"><p>ค่าที่รายงาน ไม่ใช่ระดับความปลอดภัย</p>
    <div className="badge-row"><SourceBadge type={reading.source.sourceType} /><FreshnessBadge freshness={reading.freshness} /><StatusBadge status="UNKNOWN" /></div>
    <p>แหล่งค่าที่สังเกต: {reading.source.name}</p><DataTimestamp value={reading.observedAt} />
    <Verification value={reading.verificationStatus} confidence={reading.confidence} />
    <p>{reading.waterLevelM != null ? `ระดับน้ำ ${reading.waterLevelM} ม.` : "ไม่ระบุระดับน้ำ"} · {reading.rainfallMm != null ? `ฝน ${reading.rainfallMm} มม.` : "ไม่ระบุปริมาณฝน"}</p>
  </div>;
}

export function FeatureDetails({ selection, onClose }: { selection: MapSelection; onClose: () => void }) {
  const closeButton = useRef<HTMLButtonElement>(null);
  useEffect(() => {
    const previousFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    closeButton.current?.focus();
    return () => { if (previousFocus?.isConnected) previousFocus.focus(); };
  }, [selection.layer, selection.feature.id]);
  const title = selection.layer === "flood" ? selection.feature.title : selection.layer === "reports" ? "รายงานชุมชนที่ยืนยันแล้ว" : selection.feature.name;
  const source = selection.layer === "roads" ? selection.feature.currentCondition.source : selection.layer === "reports" ? null : selection.feature.source;
  const freshness = selection.layer === "water" || selection.layer === "reports" ? "UNKNOWN" : selection.layer === "roads" ? selection.feature.currentCondition.freshness : selection.feature.freshness;
  const status = selection.layer === "flood" ? effectiveFloodSeverity(selection.feature)
    : selection.layer === "risk" ? effectiveRiskLevel(selection.feature)
      : selection.layer === "roads" ? effectiveRoadStatus(selection.feature.currentCondition) : selection.layer === "reports" ? "INFO" : "UNKNOWN";
  return <section className="map-selection" aria-label="ข้อมูลตำแหน่งที่เลือก" onKeyDown={(event) => { if (event.key === "Escape") { event.stopPropagation(); onClose(); } }}><div className="map-selection__heading"><h3>{title}</h3><button ref={closeButton} type="button" aria-label="ปิดข้อมูลตำแหน่ง" onClick={onClose}>×</button></div>
    <div className="badge-row"><StatusBadge status={status} /><FreshnessBadge freshness={freshness} />{selection.layer === "reports" ? <SourceBadge type="COMMUNITY" /> : source && <SourceBadge type={source.sourceType} />}</div>
    <p>แหล่งข้อมูล: {selection.layer === "reports" ? "รายงานจากชุมชน" : source?.name ?? "ไม่ระบุ"}</p>
    {selection.layer === "flood" && <><DataTimestamp value={selection.feature.observedAt} /><Verification value={selection.feature.verificationStatus} confidence={selection.feature.confidence} /><p>{selection.feature.description}</p></>}
    {selection.layer === "risk" && <><DataTimestamp label="เริ่มมีผล" value={selection.feature.validFrom} /><p><DataTimestamp label="สิ้นสุด" value={selection.feature.validTo} /></p><Verification /><p>{selection.feature.description}</p></>}
    {selection.layer === "roads" && <><DataTimestamp value={selection.feature.currentCondition.observedAt} /><Verification value={selection.feature.currentCondition.verificationStatus} confidence={selection.feature.currentCondition.confidence} /><p>{selection.feature.currentCondition.description ?? "ไม่มีคำอธิบายเพิ่มเติม"}</p></>}
    {selection.layer === "water" && <WaterDetails stationId={selection.feature.id} />}
    {selection.layer === "reports" && <><p>{selection.feature.description}</p><DataTimestamp label="ส่งรายงาน" value={selection.feature.createdAt} /><p>การตรวจสอบ: ตรวจสอบแล้ว</p>{selection.feature.verifiedAt && <DataTimestamp label="ยืนยันเมื่อ" value={selection.feature.verifiedAt} />}{selection.feature.estimatedWaterDepthM !== null && <p>ระดับน้ำโดยประมาณ {selection.feature.estimatedWaterDepthM.toFixed(2)} ม.</p>}{selection.feature.media.length > 0 && <ul className="report-media" aria-label="ภาพประกอบ">{selection.feature.media.map((media) => <li key={media.id}><ReportMediaImage media={media} /></li>)}</ul>}</>}
    {status === "UNKNOWN" && <p className="uncertainty">ไม่ทราบสถานะ ไม่สามารถสรุปว่าปลอดภัยหรือผ่านได้</p>}
  </section>;
}
