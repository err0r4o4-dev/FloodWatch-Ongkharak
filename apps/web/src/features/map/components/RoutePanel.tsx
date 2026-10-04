import type { LayerItems } from "../types";
import { effectiveRoadStatus, parseRoutePoint } from "../utils";
import { StatusBadge } from "../../../shared/components/DataStates";
import { formatNumber, useTranslation } from "../../../i18n";
import th from "../../../i18n/locales/th/map";
import en from "../../../i18n/locales/en/map";

export type RoutePoint = { longitude: string; latitude: string };
export type RoutePoints = { origin: RoutePoint; destination: RoutePoint };
export type Picking = keyof RoutePoints | null;

export function RoutePanel({ points, onPointChange, picking, onPickingChange, items }: {
  points: RoutePoints;
  onPointChange: (key: keyof RoutePoints, point: RoutePoint) => void;
  picking: Picking;
  onPickingChange: (key: Picking) => void;
  items: LayerItems;
}) {
  const { t, locale } = useTranslation({ th, en });
  const labels: Record<keyof RoutePoints, string> = { origin: t("route.origin"), destination: t("route.destination") };
  const attentionRoads = items.roads.filter((road) => effectiveRoadStatus(road.currentCondition) !== "PASSABLE");
  return <section className="route-panel" aria-labelledby="route-title"><h2 id="route-title">{t("route.title")}</h2>
    <p className="route-unavailable" role="status">{t("route.unavailable")}</p>
    {(Object.keys(labels) as (keyof RoutePoints)[]).map((key) => <fieldset key={key} className="route-point"><legend>{labels[key]}</legend>
      <label>{t("route.longitude")} <input type="text" inputMode="decimal" value={points[key].longitude} onChange={(event) => onPointChange(key, { ...points[key], longitude: event.target.value })} placeholder={t("route.longitudePlaceholder")} /></label>
      <label>{t("route.latitude")} <input type="text" inputMode="decimal" value={points[key].latitude} onChange={(event) => onPointChange(key, { ...points[key], latitude: event.target.value })} placeholder={t("route.latitudePlaceholder")} /></label>
      {(points[key].longitude || points[key].latitude) && !parseRoutePoint(points[key]) && <p role="alert">{t("route.coordinateError")}</p>}
      <button type="button" aria-pressed={picking === key} onClick={() => onPickingChange(picking === key ? null : key)}>{t("route.selectPoint", { label: labels[key] })}</button>
    </fieldset>)}
    {picking && <p role="status">{t("route.picking", { label: labels[picking] })}</p>}
    <div className="route-evidence"><h3>{t("route.roadEvidenceTitle")}</h3><p>{t("route.roadEvidenceNote")}</p>
      {attentionRoads.length ? <ul>{attentionRoads.map((road) => <li key={road.id}>{road.name} <StatusBadge status={effectiveRoadStatus(road.currentCondition)} /></li>)}</ul> : <p>{t("route.noRestrictions")}</p>}
      <p>{t("route.floodCount", { count: formatNumber(items.flood.length, locale) })}</p>
    </div>
  </section>;
}
