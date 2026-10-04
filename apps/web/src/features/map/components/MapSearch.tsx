import { useRef, useState } from "react";
import type { LayerItems, MapSelection } from "../types";
import { mapSelections, selectionName } from "../utils";
import { useTranslation } from "../../../i18n";
import th from "../../../i18n/locales/th/map";
import en from "../../../i18n/locales/en/map";

export function MapSearch({ items, loading, onSelect }: { items: LayerItems; loading: boolean; onSelect: (selection: MapSelection) => void }) {
  const { t, locale } = useTranslation({ th, en });
  const [query, setQuery] = useState("");
  const input = useRef<HTMLInputElement>(null);
  const results = mapSelections(items).filter((item) => (item.layer === "water" || item.layer === "risk") && selectionName(item).toLocaleLowerCase(locale).includes(query.trim().toLocaleLowerCase(locale)));
  const resultContent = loading ? <p role="status">{t("search.loading")}</p> : results.length ? <ul>{results.map((selection) => <li key={`${selection.layer}-${selection.feature.id}`}><button type="button" onClick={() => { onSelect(selection); setQuery(""); input.current?.focus(); }}><strong>{selectionName(selection)}</strong><small>{t(`layer.${selection.layer}`)}</small></button></li>)}</ul> : <p role="status">{t("search.empty")}</p>;
  return <div className="map-search" role="search" onKeyDown={(event) => { if (event.key === "Escape") { setQuery(""); input.current?.focus(); } }}>
    <div className="map-search__input"><span aria-hidden="true">⌕</span><input ref={input} type="search" aria-label={t("search.label")} aria-describedby="map-search-scope" placeholder={t("search.placeholder")} value={query} onChange={(event) => setQuery(event.target.value)} />{query && <button type="button" aria-label={t("search.clear")} onClick={() => { setQuery(""); input.current?.focus(); }}>×</button>}</div>
    <span className="sr-only" id="map-search-scope">{t("search.scope")}</span>
    {query.trim() && <div className="map-search__results"><p>{t("search.scope")}</p>{resultContent}</div>}
  </div>;
}
