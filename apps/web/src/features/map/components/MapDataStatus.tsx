import { layerKeys } from "../types";
import type { LayerItems, LayerKey, LayerVisibility } from "../types";
import { useTranslation } from "../../../i18n";
import th from "../../../i18n/locales/th/map";
import en from "../../../i18n/locales/en/map";

export function MapDataStatus({ items, visible, loading, failed, failedLayers, previousLayers, truncated, onRetry }: {
  items: LayerItems;
  visible: LayerVisibility;
  loading: boolean;
  failed: boolean;
  failedLayers: LayerKey[];
  previousLayers: LayerKey[];
  truncated: LayerKey[];
  onRetry: () => void;
}) {
  const { t } = useTranslation({ th, en });
  const count = Object.values(items).reduce((total, collection) => total + collection.length, 0);
  const showingPrevious = previousLayers.length > 0;
  return <div className={`map-data-status${failed || failedLayers.length ? " map-data-status--unavailable" : ""}`}>
    <div className="map-data-status__label"><span aria-hidden="true">◎</span><strong>{t(loading ? (showingPrevious ? "mapStatus.previous" : "mapStatus.updating") : "mapStatus.loaded")}</strong></div>
    {loading ? <p role="status">{t(showingPrevious ? "mapStatus.updating" : "mapStatus.loading")}</p> : failed ? <p role="alert">{t("mapStatus.error")} <button type="button" onClick={onRetry}>{t("mapStatus.retry")}</button></p> : <>
      {failedLayers.length > 0 && <p role="status">{t("mapStatus.partial", { layers: failedLayers.map((key) => t(`layer.${key}`)).join(", ") })} <button type="button" onClick={onRetry}>{t("mapStatus.retry")}</button></p>}
      <p>{!Object.values(visible).some(Boolean) ? t("allLayersOff") : count ? t("mapStatus.count", { count }) : t("mapStatus.empty")}</p>
    </>}
    {loading && count > 0 && <p role="status">{t("mapStatus.progress", { count })}</p>}
    {loading && failedLayers.length > 0 && <p role="status">{t("mapStatus.partial", { layers: failedLayers.map((key) => t(`layer.${key}`)).join(", ") })} <button type="button" onClick={onRetry}>{t("mapStatus.retry")}</button></p>}
    {truncated.length > 0 && <p role="status">{t("mapStatus.truncated", { layers: truncated.map((key) => t(`layer.${key}`)).join(", ") })}</p>}
    <span className="sr-only">{layerKeys.filter((key) => visible[key]).map((key) => t("mapStatus.layerCount", { label: t(`layer.${key}`), count: items[key].length })).join(" · ")}</span>
  </div>;
}
