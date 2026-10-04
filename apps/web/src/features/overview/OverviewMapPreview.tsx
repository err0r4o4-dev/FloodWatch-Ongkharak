import { useCallback, useRef, useState } from "react";
import { Link } from "react-router";
import { FeatureDetails } from "../map/components/FeatureDetails";
import { MapLegend } from "../map/components/MapLegend";
import { SituationMap } from "../map/components/SituationMap";
import { useMapData } from "../map/hooks/useMapData";
import { layerKeys } from "../map/types";
import type { LayerKey, LayerVisibility, MapBounds, MapSelection } from "../map/types";
import { useTranslation } from "../../i18n";
import th from "../../i18n/locales/th/overview";
import en from "../../i18n/locales/en/overview";

const initialVisibility: LayerVisibility = { flood: true, water: true, risk: true, roads: true, reports: true };
const emptyPoint = { longitude: "", latitude: "" };

function sameBounds(first: MapBounds | null, second: MapBounds) {
  return first?.west === second.west && first.south === second.south && first.east === second.east && first.north === second.north;
}

export function OverviewMapPreview() {
  const { t } = useTranslation({ th, en });
  const [bounds, setBounds] = useState<MapBounds | null>(null);
  const [visible, setVisible] = useState<LayerVisibility>(initialVisibility);
  const [selection, setSelection] = useState<MapSelection | null>(null);
  const [retry, setRetry] = useState(0);
  const boundsRef = useRef<MapBounds | null>(null);
  const data = useMapData(bounds, visible, retry);
  const onBoundsChange = useCallback((next: MapBounds) => {
    if (sameBounds(boundsRef.current, next)) return;
    boundsRef.current = next;
    setBounds(next);
    setSelection(null);
  }, []);
  const enabled = layerKeys.filter((key) => visible[key]);
  const allFailed = !data.loading && enabled.length > 0 && data.failed.length === enabled.length;
  const toggle = (key: LayerKey) => {
    setVisible((previous) => ({ ...previous, [key]: !previous[key] }));
    if (selection?.layer === key) setSelection(null);
  };

  return <section className="data-section overview-map" aria-labelledby="overview-map-title">
    <div className="overview-section-header"><div><h2 id="overview-map-title">{t("map.title")}</h2><p className="section-caption">{t("map.caption")}</p></div><Link to="/map">{t("map.link")}</Link></div>
    <div className="overview-map__layers" role="group" aria-label={t("map.layers")}>{layerKeys.map((key) => <button key={key} type="button" aria-pressed={visible[key]} onClick={() => toggle(key)}>{t(`map.layer.${key}`)}</button>)}</div>
    <div className="overview-map__canvas"><SituationMap items={data.items} visible={visible} mode="SITUATION" points={{ origin: emptyPoint, destination: emptyPoint }} picking={null} onBoundsChange={onBoundsChange} onMapPoint={() => undefined} onSelect={setSelection} />
      {data.loading && <p className="overview-map__status" role="status">{t("map.loading")}</p>}
      {allFailed && <div className="overview-map__status overview-map__status--error" role="alert">{t("map.error")} <button type="button" onClick={() => setRetry((previous) => previous + 1)}>{t("map.retry")}</button></div>}
      {!allFailed && data.failed.length > 0 && <p className="overview-map__status overview-map__status--error" role="status">{t("map.partial")}</p>}
    </div>
    <MapLegend visible={visible} />
    {selection && <FeatureDetails selection={selection} onClose={() => setSelection(null)} />}
    {data.truncated.length > 0 && <p className="section-caption">{t("map.truncated")}</p>}
  </section>;
}
