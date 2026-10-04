import { useEffect, useRef } from "react";
import { DataTimestamp, FreshnessBadge, OriginalContentNote, SourceBadge, StatusBadge } from "../../../shared/components/DataStates";
import { WaterDetails } from "./FeatureDetails";
import type { MapSelection } from "../types";
import { effectiveFloodSeverity, effectiveRiskLevel, effectiveRoadStatus, selectionName } from "../utils";
import { formatNumber, useTranslation } from "../../../i18n";
import th from "../../../i18n/locales/th/map";
import en from "../../../i18n/locales/en/map";

export function FeaturePopup({ selection, onDetails, onClose, restoreFocus }: {
  selection: MapSelection;
  onDetails: (selection: MapSelection) => void;
  onClose: () => void;
  restoreFocus?: () => void;
}) {
  const { t, locale } = useTranslation({ th, en });
  const closeButton = useRef<HTMLButtonElement>(null);
  useEffect(() => {
    const previousFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    closeButton.current?.focus({ preventScroll: true });
    return () => { if (previousFocus?.isConnected) previousFocus.focus({ preventScroll: true }); else restoreFocus?.(); };
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

  return <section className={`map-popup map-popup--${selection.layer}`} aria-label={t("popup.label")} onKeyDown={(event) => {
    if (event.key === "Escape") { event.stopPropagation(); onClose(); }
  }}>
    <div className="map-popup__heading"><div><p className="map-popup__kind">{t(`layer.${selection.layer}`)}</p><OriginalContentNote /><h3>{selectionName(selection)}</h3></div><button ref={closeButton} className="map-popup__close" type="button" aria-label={t("popup.close")} onClick={onClose}>×</button></div>
    <div className="map-popup__body"><div className="badge-row">{selection.layer !== "water" && <StatusBadge status={status} />}{freshness && <FreshnessBadge freshness={freshness} />}{selection.layer === "reports" ? <SourceBadge type="COMMUNITY" /> : source && <SourceBadge type={source.sourceType} />}</div>
    <p className="map-popup__source">{source?.name ?? (selection.layer === "reports" ? t("popup.communitySource") : t("popup.unknownSource"))}</p>
    {selection.layer === "water" ? <WaterDetails key={selection.feature.id} stationId={selection.feature.id} compact /> : <DataTimestamp label={selection.layer === "risk" ? t("timestamp.validFrom") : selection.layer === "reports" ? t("timestamp.verified") : t("timestamp.observed")} value={observedAt} />}
    {selection.layer === "risk" && <DataTimestamp label={t("timestamp.validTo")} value={selection.feature.validTo} />}
    {selection.layer === "reports" && selection.feature.estimatedWaterDepthM !== null && <p>{t("popup.reportDepth", { value: formatNumber(selection.feature.estimatedWaterDepthM, locale, { minimumFractionDigits: 2, maximumFractionDigits: 2 }) })}</p>}
    {status === "UNKNOWN" && selection.layer !== "water" && <p className="uncertainty">{t("popup.unknownWarning")}</p>}
    </div>
    <button className="map-popup__details" type="button" onClick={() => onDetails(selection)}>{t("popup.details")} <span aria-hidden="true">→</span></button>
  </section>;
}
