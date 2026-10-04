import { useId } from "react";
import { Link } from "react-router";
import { useTranslation } from "../../i18n";
import th from "../../i18n/locales/th/common";
import en from "../../i18n/locales/en/common";
import { StatusBadge } from "../../shared/components/DataStates";
import { LandingIcon } from "./LandingIcon";

const messages = { th, en };
const roads = [
  "M-20 95 160 125 300 105 455 140 740 100",
  "M-20 300 155 260 320 285 500 255 740 285",
  "M145 -20 180 105 155 260 200 420",
  "M355 -20 320 110 365 225 320 285 345 420",
  "M580 -20 555 140 600 255 550 420",
  "M-20 205 160 185 355 225 565 195 740 215",
  "M65 -20 85 110 55 200 95 270 70 420",
  "M450 -20 445 140 475 205 445 265 470 420"
];

export function LandingMapPreview() {
  const { t } = useTranslation(messages);
  const illustrationId = useId();
  return <section className="landing-map" aria-labelledby="landing-map-title">
    <div className="landing-map__card">
      <header className="landing-map__heading"><span className="landing-map__icon"><LandingIcon kind="map" /></span><div><h2 id="landing-map-title">{t("waterSituationMap")}</h2><p className="landing-map__disclaimer">{t("landingDemoNotice")}</p></div><Link className="landing-map__expand" to="/map" aria-label={t("openFullMap")}><LandingIcon kind="expand" /></Link></header>
      <figure className="landing-map__canvas">
        <svg className="landing-map__illustration" viewBox="0 0 720 400" role="img" aria-labelledby={`${illustrationId}-title`} aria-describedby={`${illustrationId}-description`}>
          <title id={`${illustrationId}-title`}>{t("landingDemoMapTitle")}</title>
          <desc id={`${illustrationId}-description`}>{t("landingDemoMapDescription")}</desc>
          <defs><pattern id={`${illustrationId}-grid`} width="40" height="40" patternUnits="userSpaceOnUse"><path d="M40 0H0V40" className="landing-map__grid" /></pattern><pattern id={`${illustrationId}-hatch`} width="12" height="12" patternUnits="userSpaceOnUse" patternTransform="rotate(35)"><path d="M0 0V12" className="landing-map__hatch" /></pattern></defs>
          <rect width="720" height="400" className="landing-map__terrain" />
          <path d="M0 0H220L190 80 65 100 0 60ZM440 0H720V125L605 100 500 80ZM0 320 145 305 230 400H0ZM430 320 580 290 720 350V400H465Z" className="landing-map__blocks" />
          <rect width="720" height="400" fill={`url(#${illustrationId}-grid)`} />
          <g className="landing-map__road-border">{roads.map((road) => <path key={road} d={road} />)}</g>
          <g className="landing-map__roads">{roads.map((road) => <path key={road} d={road} />)}</g>
          <path d="M265-20C210 55 290 85 245 150S230 225 285 265s-30 90-5 155" className="landing-map__river-bank" />
          <path d="M265-20C210 55 290 85 245 150S230 225 285 265s-30 90-5 155" className="landing-map__river" />
          <path d="M370 145 465 165 490 235 385 245 345 205Z" className="landing-map__sample-area" />
          <path d="M370 145 465 165 490 235 385 245 345 205Z" fill={`url(#${illustrationId}-hatch)`} />
          <path d="M140 125 300 105 455 140 475 205 445 265 320 285" className="landing-map__sample-route" />
          <g className="landing-map__marker landing-map__marker--water" transform="translate(245 165)"><circle r="22" /><path d="M-10-4q5-5 10 0t10 0M-10 5q5-5 10 0t10 0" /></g>
          <g className="landing-map__marker landing-map__marker--event" transform="translate(410 195)"><circle r="22" /><path d="M0-10V3M0 9v1" /></g>
          <g className="landing-map__marker landing-map__marker--station" transform="translate(165 265)"><circle r="22" /><path d="M-8 8V-7H8V8ZM-3-3h6M-3 2h6" /></g>
          <g className="landing-map__marker landing-map__marker--point" transform="translate(550 285)"><circle r="22" /><circle r="6" /></g>
          <text x="64" y="165" className="landing-map__label">{t("landingDemoRoad")}</text>
          <text x="300" y="355" className="landing-map__label landing-map__label--water">{t("landingDemoWaterway")}</text>
        </svg>
      </figure>
      <div className="landing-map__risk-legend" aria-label={t("landingMapRiskLegend")}><strong>{t("landingDemoRiskLegend")}</strong><div>{["LOW", "MODERATE", "HIGH", "VERY_HIGH", "UNKNOWN"].map((status) => <StatusBadge key={status} status={status} />)}</div></div>
    </div>
    <aside className="landing-weather" aria-label={t("landingDemoWeather")}><span className="landing-weather__icon"><LandingIcon kind="cloud" /></span><div><span className="landing-weather__demo">{t("landingDemoWeather")}</span><strong className="landing-weather__temperature">28°C</strong><p>{t("landingDemoCloudy")}</p><small>{t("landingDemoWeatherNotice")}</small></div></aside>
    <div className="landing-map__evidence"><details><summary>{t("landingMapEvidence")}</summary><p>{t("landingDemoEvidence")}</p><ul className="landing-map__sample-key"><li><span aria-hidden="true">≋</span>{t("landingDemoWaterway")}</li><li><span aria-hidden="true">▧</span>{t("landingDemoArea")}</li><li><span aria-hidden="true">●</span>{t("landingDemoMarker")}</li></ul><Link to="/map">{t("landingDemoOpenMap")}</Link></details></div>
  </section>;
}
