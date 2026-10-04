import { useEffect, useRef, useState } from "react";
import { DataTimestamp, FreshnessBadge, OriginalContentNote, SourceBadge, StatusBadge } from "../../../shared/components/DataStates";
import { ReportMediaImage } from "../../reports/ReportMediaImage";
import { fetchLatestReading } from "../api";
import type { MapSelection, WaterReading } from "../types";
import { effectiveFloodSeverity, effectiveRiskLevel, effectiveRoadStatus } from "../utils";
import { formatNumber, useTranslation } from "../../../i18n";
import th from "../../../i18n/locales/th/map";
import en from "../../../i18n/locales/en/map";

function Verification({ value, confidence }: { value?: string; confidence?: number | null }) {
  const { t, locale } = useTranslation({ th, en });
  const status = value ? t(value === "VERIFIED" ? "details.verified" : value === "UNVERIFIED" ? "details.unverified" : value === "REJECTED" ? "details.rejected" : "details.unknown") : t("details.apiUnspecified");
  const certainty = confidence == null ? t("details.unspecified") : `${formatNumber(Math.round(confidence * 100), locale)}%`;
  return <p className="map-evidence">{t("details.verification", { status, confidence: certainty })}</p>;
}

export function WaterDetails({ stationId, compact = false }: { stationId: string; compact?: boolean }) {
  const { t, locale } = useTranslation({ th, en });
  const [result, setResult] = useState<{ state: "loading" | "error" | "ready"; reading: WaterReading | null }>({ state: "loading", reading: null });
  useEffect(() => {
    const controller = new AbortController();
    setResult({ state: "loading", reading: null });
    void fetchLatestReading(stationId, controller.signal).then((reading) => {
      if (!controller.signal.aborted) setResult({ state: "ready", reading });
    }).catch(() => { if (!controller.signal.aborted) setResult({ state: "error", reading: null }); });
    return () => controller.abort();
  }, [stationId]);
  if (result.state === "loading") return <p role="status">{t("details.loadingReading")}</p>;
  if (result.state === "error") return <p role="alert">{t("details.errorReading")}</p>;
  if (!result.reading) return <p>{t("details.emptyReading")}</p>;
  const reading = result.reading;
  const waterLevel = reading.waterLevelM != null ? t("details.waterLevel", { value: formatNumber(reading.waterLevelM, locale, { maximumFractionDigits: 2 }) }) : t("details.noWaterLevel");
  const rainfall = reading.rainfallMm != null ? t("details.rainfall", { value: formatNumber(reading.rainfallMm, locale, { maximumFractionDigits: 2 }) }) : t("details.noRainfall");
  if (compact) return <div className="map-reading">
    <p className="map-reading__value">{waterLevel}</p>
    <div className="badge-row"><SourceBadge type={reading.source.sourceType} /><FreshnessBadge freshness={reading.freshness} /><StatusBadge status="UNKNOWN" /></div>
    <p>{t("details.observedSource", { source: reading.source.name })}</p><DataTimestamp value={reading.observedAt} />
    <small>{t("details.notSafety")}</small>
  </div>;
  return <div className="map-reading"><p>{t("details.notSafety")}</p>
    <div className="badge-row"><SourceBadge type={reading.source.sourceType} /><FreshnessBadge freshness={reading.freshness} /><StatusBadge status="UNKNOWN" /></div>
    <p>{t("details.observedSource", { source: reading.source.name })}</p><DataTimestamp value={reading.observedAt} />
    <Verification value={reading.verificationStatus} confidence={reading.confidence} />
    <p>{waterLevel} · {rainfall}</p>
  </div>;
}

export function FeatureDetails({ selection, onClose }: { selection: MapSelection; onClose: () => void }) {
  const { t, locale } = useTranslation({ th, en });
  const closeButton = useRef<HTMLButtonElement>(null);
  useEffect(() => {
    const previousFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    closeButton.current?.focus();
    return () => { if (previousFocus?.isConnected) previousFocus.focus(); };
  }, [selection.layer, selection.feature.id]);
  const title = selection.layer === "flood" ? selection.feature.title : selection.layer === "reports" ? t("layer.reports") : selection.feature.name;
  const source = selection.layer === "roads" ? selection.feature.currentCondition.source : selection.layer === "reports" ? null : selection.feature.source;
  const freshness = selection.layer === "water" || selection.layer === "reports" ? "UNKNOWN" : selection.layer === "roads" ? selection.feature.currentCondition.freshness : selection.feature.freshness;
  const status = selection.layer === "flood" ? effectiveFloodSeverity(selection.feature)
    : selection.layer === "risk" ? effectiveRiskLevel(selection.feature)
      : selection.layer === "roads" ? effectiveRoadStatus(selection.feature.currentCondition) : selection.layer === "reports" ? "INFO" : "UNKNOWN";
  return <section className="map-selection" aria-label={t("details.label")} onKeyDown={(event) => { if (event.key === "Escape") { event.stopPropagation(); onClose(); } }}><div className="map-selection__heading"><h3>{title}</h3><button ref={closeButton} type="button" aria-label={t("details.close")} onClick={onClose}>×</button></div>
    <div className="badge-row"><StatusBadge status={status} /><FreshnessBadge freshness={freshness} />{selection.layer === "reports" ? <SourceBadge type="COMMUNITY" /> : source && <SourceBadge type={source.sourceType} />}</div>
    <p>{t("details.source", { source: selection.layer === "reports" ? t("details.communitySource") : source?.name ?? t("details.unknownSource") })}</p>
    {selection.layer === "flood" && <><DataTimestamp value={selection.feature.observedAt} /><Verification value={selection.feature.verificationStatus} confidence={selection.feature.confidence} /><OriginalContentNote /><p>{selection.feature.description}</p></>}
    {selection.layer === "risk" && <><DataTimestamp label={t("timestamp.validFrom")} value={selection.feature.validFrom} /><p><DataTimestamp label={t("timestamp.validTo")} value={selection.feature.validTo} /></p><Verification /><OriginalContentNote /><p>{selection.feature.description}</p></>}
    {selection.layer === "roads" && <><DataTimestamp value={selection.feature.currentCondition.observedAt} /><Verification value={selection.feature.currentCondition.verificationStatus} confidence={selection.feature.currentCondition.confidence} /><OriginalContentNote /><p>{selection.feature.currentCondition.description ?? t("details.noDescription")}</p></>}
    {selection.layer === "water" && <WaterDetails stationId={selection.feature.id} />}
    {selection.layer === "reports" && <><OriginalContentNote /><p>{selection.feature.description}</p><DataTimestamp label={t("timestamp.submitted")} value={selection.feature.createdAt} /><p>{t("details.reportVerified")}</p>{selection.feature.verifiedAt && <DataTimestamp label={t("timestamp.verified")} value={selection.feature.verifiedAt} />}{selection.feature.estimatedWaterDepthM !== null && <p>{t("details.reportDepth", { value: formatNumber(selection.feature.estimatedWaterDepthM, locale, { minimumFractionDigits: 2, maximumFractionDigits: 2 }) })}</p>}{selection.feature.media.length > 0 && <ul className="report-media" aria-label={t("details.media")}>{selection.feature.media.map((media) => <li key={media.id}><ReportMediaImage media={media} /></li>)}</ul>}</>}
    {status === "UNKNOWN" && <p className="uncertainty">{t("details.unknownWarning")}</p>}
  </section>;
}
