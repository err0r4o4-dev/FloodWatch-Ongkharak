import { useEffect, useRef } from "react";
import { EmptyState, StatusBadge } from "../../../shared/components/DataStates";
import { FeatureDetails } from "./FeatureDetails";
import { RoutePanel } from "./RoutePanel";
import { RiskDirectory } from "./RiskDirectory";
import type { Picking, RoutePoint, RoutePoints } from "./RoutePanel";
import { effectiveFloodSeverity, effectiveRiskLevel, effectiveRoadStatus } from "../utils";
import { ReportAreaDepthBadge } from "../../../features/report-areas/ReportAreaSummary";
import { areaTrendSymbol, displayReportAreaTrend, effectiveReportAreaFreshness } from "../../../features/report-areas/presentation";
import { layerKeys } from "../types";
import type { LayerItems, LayerKey, LayerVisibility, MapMode, MapSelection } from "../types";
import { useTranslation } from "../../../i18n";
import th from "../../../i18n/locales/th/map";
import en from "../../../i18n/locales/en/map";

export function MapSidePanel({ mode, items, visible, loading, failed, failedLayers, previousLayers, truncated, onRetry, selection, onSelect, onClose, points, onPointChange, picking, onPickingChange, showList = true, showRisk = false, selectedRiskId = null, onRiskSelect = onSelect, onDismiss }: {
  mode: MapMode;
  items: LayerItems;
  visible: LayerVisibility;
  loading: boolean;
  failed: boolean;
  failedLayers: LayerKey[];
  previousLayers: LayerKey[];
  truncated: LayerKey[];
  onRetry: () => void;
  selection: MapSelection | null;
  onSelect: (selection: MapSelection) => void;
  onClose: () => void;
  points: RoutePoints;
  onPointChange: (key: keyof RoutePoints, point: RoutePoint) => void;
  picking: Picking;
  onPickingChange: (key: Picking) => void;
  showList?: boolean;
  showRisk?: boolean;
  selectedRiskId?: string | null;
  onRiskSelect?: (selection: MapSelection) => void;
  onDismiss?: () => void;
}) {
  const { t } = useTranslation({ th, en });
  const dismissButton = useRef<HTMLButtonElement>(null);
  useEffect(() => { if (!selection) dismissButton.current?.focus({ preventScroll: true }); }, []);
  const selections: MapSelection[] = [
    ...items.flood.map((feature) => ({ layer: "flood" as const, feature })),
    ...items.water.map((feature) => ({ layer: "water" as const, feature })),
    ...items.risk.map((feature) => ({ layer: "risk" as const, feature })),
    ...items.roads.map((feature) => ({ layer: "roads" as const, feature })),
    ...items.reports.map((feature) => ({ layer: "reports" as const, feature }))
  ];
  const showingPrevious = previousLayers.length > 0;
  return <aside id="map-information-panel" className="map-side-panel" aria-label={t("panel.label")} onKeyDown={(event) => {
    if (event.key === "Escape" && !selection) { event.stopPropagation(); onDismiss?.(); }
  }}>
    {onDismiss && !selection && <div className="map-side-panel__heading"><strong>{t(mode === "ROUTE" ? "panel.route" : showRisk ? "panel.risk" : "panel.list")}</strong><button ref={dismissButton} type="button" aria-label={t("panel.close")} onClick={onDismiss}>×</button></div>}
    {selection && <FeatureDetails selection={selection} onClose={onClose} />}
    {mode === "ROUTE" && <RoutePanel points={points} onPointChange={onPointChange} picking={picking} onPickingChange={onPickingChange} items={items} />}
    {showRisk && !selection && <RiskDirectory items={items.risk} loading={loading} failed={failedLayers.includes("risk")} previousLayers={previousLayers} truncated={truncated.includes("risk")} selectedId={selectedRiskId} onSelect={onRiskSelect} onRetry={onRetry} />}
    {showList && <section aria-labelledby="visible-features-title"><h2 id="visible-features-title">{t("panel.listTitle")}</h2>
      <p>{t("panel.listInstruction")}</p>
      {!failed && failedLayers.length > 0 && <p className="map-partial" role="status">{t("panel.partial", { layers: failedLayers.map((key) => t(`layer.${key}`)).join(", ") })} <button type="button" onClick={onRetry}>{t("mapStatus.retry")}</button></p>}
      {truncated.length > 0 && <p role="status">{t("panel.truncated", { layers: truncated.map((key) => t(`layer.${key}`)).join(", ") })}</p>}
      {loading && <p role="status">{t(showingPrevious ? "mapStatus.previous" : selections.length ? "mapStatus.updating" : "panel.loading")}</p>}
      {loading && selections.length > 0 && <p role="status">{t("mapStatus.progress", { count: selections.length })}</p>}
      {failed ? <div role="alert"><p>{t("panel.error")}</p><button type="button" className="action-button" onClick={onRetry}>{t("mapStatus.retry")}</button></div> : selections.length ? <ul className="map-feature-list">{selections.map((item) => {
        const name = item.layer === "flood" ? item.feature.title : item.feature.name;
        const status = item.layer === "roads" ? effectiveRoadStatus(item.feature.currentCondition) : item.layer === "flood" ? effectiveFloodSeverity(item.feature) : item.layer === "risk" ? effectiveRiskLevel(item.feature) : "UNKNOWN";
        const areaFreshness = item.layer === "reports" ? effectiveReportAreaFreshness(item.feature.freshness, item.feature.latestObservedAt) : null;
        const areaTrend = item.layer === "reports" ? displayReportAreaTrend(item.feature.trend, item.feature.freshness, item.feature.latestObservedAt) : null;
        return <li key={`${item.layer}-${item.feature.id}`}><button type="button" onClick={() => onSelect(item)}><span>{name}<small>{t(`layer.${item.layer}`)}</small></span>{item.layer === "reports" ? <span className="map-feature-list__area-meta"><ReportAreaDepthBadge depthBand={item.feature.depthBand} freshness={areaFreshness ?? "UNKNOWN"} latestObservedAt={item.feature.latestObservedAt} /><small><span aria-hidden="true">{areaTrendSymbol(areaTrend ?? "UNKNOWN")} </span>{t(`area.trend.${areaTrend ?? "UNKNOWN"}`)}</small></span> : <StatusBadge status={status} />}</button></li>;
      })}</ul> : loading ? null : <EmptyState title={t("panel.emptyTitle")}>{t("panel.emptyBody")}</EmptyState>}
      <p className="map-layer-summary">{layerKeys.filter((key) => visible[key]).map((key) => t("panel.layerSummary", { label: t(`layer.${key}`), count: items[key].length })).join(" · ") || t("allLayersOff")}</p>
    </section>}
  </aside>;
}
