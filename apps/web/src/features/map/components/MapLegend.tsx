import type { LayerVisibility } from "../types";
import { useTranslation } from "../../../i18n";
import th from "../../../i18n/locales/th/map";
import en from "../../../i18n/locales/en/map";

export function MapLegend({ visible }: { visible: LayerVisibility }) {
  const { t } = useTranslation({ th, en });
  return <div className="map-legend" aria-label={t("legend.label")}><span className="sr-only">{t("legend.prefix")}</span>
    {visible.water && <span><i className="map-layer-symbol map-layer-symbol--water" aria-hidden="true">≋</i> {t("layer.water")}</span>}
    {visible.risk && <span><i className="map-layer-symbol map-layer-symbol--risk" aria-hidden="true">⚠</i> {t("layer.risk")}</span>}
    {visible.flood && <span><i className="map-layer-symbol map-layer-symbol--flood" aria-hidden="true">△</i> {t("layer.flood")}</span>}
    {visible.reports && <span><i className="map-layer-symbol map-layer-symbol--reports" aria-hidden="true">●</i> {t("layer.reports")}</span>}
    {visible.roads && <span><i className="map-layer-symbol map-layer-symbol--roads" aria-hidden="true">━</i> {t("legend.roads")}</span>}
    {visible.reports && <span className="map-legend__area-group" aria-label={t("legend.areaDepth")}>
      <i className="map-legend__depth-dot map-legend__depth-dot--very-high" aria-hidden="true" /> {t("legend.depthVeryHigh")}
      <i className="map-legend__depth-dot map-legend__depth-dot--high" aria-hidden="true" /> {t("legend.depthHigh")}
      <i className="map-legend__depth-dot map-legend__depth-dot--moderate" aria-hidden="true" /> {t("legend.depthModerate")}
      <i className="map-legend__depth-dot map-legend__depth-dot--low" aria-hidden="true" /> {t("legend.depthLow")}
      <i className="map-legend__depth-dot map-legend__depth-dot--unknown" aria-hidden="true" /> {t("legend.depthUnknown")}
    </span>}
    {visible.reports && <span className="map-legend__trend-note"><i aria-hidden="true">↓</i> {t("legend.areaTrend")}</span>}
    {!Object.values(visible).some(Boolean) && <span>{t("allLayersOff")}</span>}
  </div>;
}
