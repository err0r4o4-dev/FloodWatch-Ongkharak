import { useState } from "react";
import { FreshnessBadge, SourceBadge, StatusBadge } from "../../../shared/components/DataStates";
import type { MapSelection, RiskFeature } from "../types";
import { effectiveRiskLevel } from "../utils";
import { useTranslation } from "../../../i18n";
import th from "../../../i18n/locales/th/map";
import en from "../../../i18n/locales/en/map";

const levels = ["ALL", "VERY_HIGH", "HIGH", "MODERATE", "LOW", "UNKNOWN"] as const;
const levelKeys: Record<typeof levels[number], keyof typeof th> = {
  ALL: "risk.all", VERY_HIGH: "risk.veryHigh", HIGH: "risk.high", MODERATE: "risk.moderate", LOW: "risk.low", UNKNOWN: "risk.unknown"
};

export function RiskDirectory({ items, loading, failed, truncated, selectedId, onSelect, onRetry }: {
  items: RiskFeature[];
  loading: boolean;
  failed: boolean;
  truncated: boolean;
  selectedId: string | null;
  onSelect: (selection: MapSelection) => void;
  onRetry: () => void;
}) {
  const { t } = useTranslation({ th, en });
  const [filter, setFilter] = useState("ALL");
  const filtered = items.filter((item) => filter === "ALL" || effectiveRiskLevel(item) === filter);
  const ready = !loading && !failed;
  return <section className="map-risk-directory" aria-labelledby="map-risk-title">
    <h2 id="map-risk-title">{t("risk.title")}</h2>
    <p>{t("risk.subtitle")}</p>
    <div className="map-risk-filters" role="group" aria-label={t("risk.filterGroup")}>
      {levels.map((key) => { const label = t(levelKeys[key]); return <button key={key} type="button" aria-label={t("risk.filter", { label })} aria-pressed={filter === key} disabled={!ready} onClick={() => setFilter(key)}><span>{label}</span><strong>{ready ? (key === "ALL" ? items.length : items.filter((item) => effectiveRiskLevel(item) === key).length) : "—"}</strong></button>; })}
    </div>
    <p className="map-risk-directory__note">{t("risk.filterNote")}</p>
    {loading ? <p role="status">{t("risk.loading")}</p> : failed ? <div role="alert"><p>{t("risk.error")}</p><button className="action-button" type="button" onClick={onRetry}>{t("risk.retry")}</button></div> : items.length === 0 ? <p role="status">{t("risk.empty")}</p> : filtered.length === 0 ? <p role="status">{t("risk.noMatch")}</p> : <ul className="map-feature-list map-risk-list">{filtered.map((item) => <li key={item.id}><button type="button" aria-pressed={selectedId === item.id} onClick={() => onSelect({ layer: "risk", feature: item })}><span><strong>{item.name}</strong><span className="map-risk-list__badges"><StatusBadge status={effectiveRiskLevel(item)} /><FreshnessBadge freshness={item.freshness} /><SourceBadge type={item.source.sourceType} /></span><small>{item.source.name}</small></span><span aria-hidden="true">↗</span></button></li>)}</ul>}
    {truncated && <p role="status">{t("risk.truncated")}</p>}
    <p className="map-risk-directory__note">{t("risk.footer")}</p>
  </section>;
}
