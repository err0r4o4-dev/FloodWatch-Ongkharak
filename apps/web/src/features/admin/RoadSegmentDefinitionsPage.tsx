import { useEffect, useMemo, useState, type FormEvent } from "react";
import { NavLink } from "react-router";
import { formatDate, useTranslation } from "../../i18n";
import th from "../../i18n/locales/th/roadSegments";
import en from "../../i18n/locales/en/roadSegments";
import { notifyError, notifySuccess } from "../../shared/alerts";
import { fetchRoadSegments, listRoadOptionPage, roadSegmentsApi } from "../road-segments/api";
import type { RoadOption, RoadSegment } from "../road-segments/types";
import "./RoadSegmentDefinitionsPage.css";

const ROAD_PAGE_SIZE = 100;
const SEGMENT_PAGE_SIZE = 100;
const OSM_WAY_PREFIX = "OSM-WAY";

type FormError = "required" | "geometry" | null;

function parseLineString(value: string): { type: "LineString"; coordinates: [number, number][] } | null {
  try {
    const parsed: unknown = JSON.parse(value);
    if (!parsed || typeof parsed !== "object" || !("type" in parsed) || !("coordinates" in parsed) || parsed.type !== "LineString" || !Array.isArray(parsed.coordinates) || parsed.coordinates.length < 2) return null;
    if (!parsed.coordinates.every((item) => Array.isArray(item) && item.length === 2 && typeof item[0] === "number" && Number.isFinite(item[0]) && item[0] >= -180 && item[0] <= 180 && typeof item[1] === "number" && Number.isFinite(item[1]) && item[1] >= -90 && item[1] <= 90)) return null;
    return { type: "LineString", coordinates: parsed.coordinates as [number, number][] };
  } catch {
    return null;
  }
}

function isOpenStreetMapRoad(road: RoadOption | undefined): boolean {
  return road?.code?.toUpperCase().startsWith(OSM_WAY_PREFIX) === true;
}

function roadLabel(road: RoadOption, osmLabel: string): string {
  return `${road.code ? `${road.code} · ` : ""}${road.name}${isOpenStreetMapRoad(road) ? ` · ${osmLabel}` : ""}`;
}

export function RoadSegmentDefinitionsPage() {
  const { locale, t } = useTranslation({ th, en });
  const [roads, setRoads] = useState<RoadOption[]>([]);
  const [roadOffset, setRoadOffset] = useState(0);
  const [roadsHasMore, setRoadsHasMore] = useState(false);
  const [roadSearch, setRoadSearch] = useState("");
  const [roadLoadingMore, setRoadLoadingMore] = useState(false);
  const [roadLoadMoreError, setRoadLoadMoreError] = useState(false);
  const [segments, setSegments] = useState<RoadSegment[]>([]);
  const [segmentOffset, setSegmentOffset] = useState(0);
  const [segmentsHasMore, setSegmentsHasMore] = useState(false);
  const [segmentLoadingMore, setSegmentLoadingMore] = useState(false);
  const [segmentLoadMoreError, setSegmentLoadMoreError] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);
  const [retry, setRetry] = useState(0);
  const [saving, setSaving] = useState(false);
  const [roadId, setRoadId] = useState("");
  const [name, setName] = useState("");
  const [startLabel, setStartLabel] = useState("");
  const [endLabel, setEndLabel] = useState("");
  const [geometry, setGeometry] = useState("");
  const [supersedesId, setSupersedesId] = useState("");
  const [replacingSegmentId, setReplacingSegmentId] = useState<string | null>(null);
  const [formError, setFormError] = useState<FormError>(null);

  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    setError(false);
    setRoadLoadMoreError(false);
    setSegmentLoadMoreError(false);
    void Promise.all([
      listRoadOptionPage(ROAD_PAGE_SIZE, 0, controller.signal),
      fetchRoadSegments(null, SEGMENT_PAGE_SIZE, 0, controller.signal)
    ]).then(([roadPage, segmentPage]) => {
      if (controller.signal.aborted) return;
      setRoads(roadPage.items);
      setRoadOffset(roadPage.offset + roadPage.limit);
      setRoadsHasMore(roadPage.hasMore);
      setSegments(segmentPage.items.filter((segment) => segment.active));
      setSegmentOffset(segmentPage.offset + segmentPage.limit);
      setSegmentsHasMore(segmentPage.items.length === segmentPage.limit);
    }).catch(() => {
      if (!controller.signal.aborted) setError(true);
    }).finally(() => {
      if (!controller.signal.aborted) setLoading(false);
    });
    return () => controller.abort();
  }, [retry]);

  const visibleRoads = useMemo(() => {
    const query = roadSearch.trim().toLocaleLowerCase();
    const matches = query.length === 0 ? roads : roads.filter((road) => `${road.code ?? ""} ${road.name}`.toLocaleLowerCase().includes(query));
    const selected = roads.find((road) => road.id === roadId);
    if (selected && !matches.some((road) => road.id === selected.id)) return [selected, ...matches];
    return matches;
  }, [roadId, roadSearch, roads]);

  const selectedRoadIsLoaded = roads.some((road) => road.id === roadId);
  const hasOsmRoads = roads.some((road) => isOpenStreetMapRoad(road));

  const resetForm = () => {
    setRoadId("");
    setName("");
    setStartLabel("");
    setEndLabel("");
    setGeometry("");
    setSupersedesId("");
    setReplacingSegmentId(null);
    setFormError(null);
  };

  const startReplacement = (segment: RoadSegment) => {
    setRoadId(segment.roadId);
    setName(segment.name);
    setStartLabel(segment.startLabel);
    setEndLabel(segment.endLabel);
    setGeometry(JSON.stringify(segment.geometry));
    setSupersedesId(segment.id);
    setReplacingSegmentId(segment.id);
    setFormError(null);
  };

  const loadMoreRoads = async () => {
    setRoadLoadingMore(true);
    setRoadLoadMoreError(false);
    try {
      const page = await listRoadOptionPage(ROAD_PAGE_SIZE, roadOffset);
      setRoads((current) => {
        const byId = new Map(current.map((road) => [road.id, road]));
        page.items.forEach((road) => byId.set(road.id, road));
        return [...byId.values()];
      });
      setRoadOffset(page.offset + page.limit);
      setRoadsHasMore(page.hasMore);
    } catch {
      setRoadLoadMoreError(true);
    } finally {
      setRoadLoadingMore(false);
    }
  };

  const loadMoreSegments = async () => {
    setSegmentLoadingMore(true);
    setSegmentLoadMoreError(false);
    try {
      const page = await fetchRoadSegments(null, SEGMENT_PAGE_SIZE, segmentOffset);
      setSegments((current) => [...current, ...page.items.filter((segment) => segment.active)]);
      setSegmentOffset(page.offset + page.limit);
      setSegmentsHasMore(page.items.length === page.limit);
    } catch {
      setSegmentLoadMoreError(true);
    } finally {
      setSegmentLoadingMore(false);
    }
  };

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const parsedGeometry = parseLineString(geometry);
    if (!roadId || !name.trim() || !startLabel.trim() || !endLabel.trim()) {
      setFormError("required");
      return;
    }
    if (!parsedGeometry) {
      setFormError("geometry");
      return;
    }
    setFormError(null);
    setSaving(true);
    try {
      await roadSegmentsApi.create({ roadId, name: name.trim(), startLabel: startLabel.trim(), endLabel: endLabel.trim(), geometry: parsedGeometry, ...(supersedesId ? { supersedesId } : {}) });
      resetForm();
      setRetry((value) => value + 1);
      void notifySuccess(t("success"));
    } catch {
      void notifyError(t("error"), t("error"));
    } finally {
      setSaving(false);
    }
  };

  return <div className="page-container admin-page road-segment-definition__grid">
    <header className="page-header">
      <NavLink to="/admin" className="button">&larr; {t("back")}</NavLink>
      <h1>{t("title")}</h1>
      <p>{t("description")}</p>
      <p className="muted">{t("instructions")}</p>
    </header>
    {loading && <p role="status">{t("loading")}</p>}
    {error && <div className="panel error-state" role="alert"><p>{t("error")}</p><button type="button" className="action-button" onClick={() => setRetry((value) => value + 1)}>{t("retry")}</button></div>}
    {!loading && !error && <>
      {hasOsmRoads && <aside className="panel road-segment-definition__source" aria-label={t("source.title")}>
        <p><strong>{t("source.title")}</strong> {t("source.loaded")}</p>
        <p>{t("source.attribution")} <a href="https://www.openstreetmap.org/copyright" target="_blank" rel="noreferrer">{t("source.contributors")}</a> ({t("source.license")}).</p>
        <p className="muted">{t("source.evidenceNote")}</p>
      </aside>}
      <section className="panel" aria-labelledby="road-segment-form-title">
        <h2 id="road-segment-form-title">{replacingSegmentId ? t("form.replacementTitle") : t("form.title")}</h2>
        <form className="auth-form" onSubmit={submit} noValidate>
          <div className="road-segment-definition__road-picker">
            <label htmlFor="road-segment-road-search">{t("form.roadSearch")}</label>
            <input id="road-segment-road-search" type="search" value={roadSearch} placeholder={t("form.roadSearchPlaceholder")} onChange={(event) => setRoadSearch(event.target.value)} />
            <p className="muted small">{t("form.roadSearchNote", { count: roads.length })}</p>
            <label htmlFor="road-segment-road">{t("form.road")}</label>
            <select id="road-segment-road" value={roadId} required onChange={(event) => { setRoadId(event.target.value); setFormError(null); }}>
              <option value="">{t("form.roadPlaceholder")}</option>
              {roadId && !selectedRoadIsLoaded && <option value={roadId}>{t("form.unloadedRoad", { id: roadId })}</option>}
              {visibleRoads.map((road) => <option key={road.id} value={road.id}>{roadLabel(road, t("source.osmShort"))}</option>)}
            </select>
            {visibleRoads.length === 0 && <p className="muted small">{t("form.noRoadMatches")}</p>}
            {roadsHasMore && <button type="button" className="button button--small" onClick={() => void loadMoreRoads()} disabled={roadLoadingMore}>{roadLoadingMore ? t("form.loadingMoreRoads") : t("form.loadMoreRoads")}</button>}
            {roadLoadMoreError && <p className="error-text" role="alert">{t("form.loadMoreError")}</p>}
          </div>
          <label htmlFor="road-segment-name">{t("form.name")}<input id="road-segment-name" required value={name} placeholder={t("form.namePlaceholder")} onChange={(event) => { setName(event.target.value); setFormError(null); }} /></label>
          <div className="admin-form-grid"><label htmlFor="road-segment-start">{t("form.start")}<input id="road-segment-start" required value={startLabel} onChange={(event) => { setStartLabel(event.target.value); setFormError(null); }} /></label><label htmlFor="road-segment-end">{t("form.end")}<input id="road-segment-end" required value={endLabel} onChange={(event) => { setEndLabel(event.target.value); setFormError(null); }} /></label></div>
          <p className="muted small">{t("form.labelsNote")}</p>
          <label htmlFor="road-segment-geometry">{t("form.geometry")}<textarea id="road-segment-geometry" required value={geometry} placeholder={t("form.geometryPlaceholder")} onChange={(event) => { setGeometry(event.target.value); setFormError(null); }} rows={5} /></label>
          {replacingSegmentId ? <div className="road-segment-definition__replacement" role="status"><p>{t("form.replacementNote")}</p><button type="button" className="button button--small" onClick={resetForm}>{t("form.cancelReplacement")}</button></div> : <p className="muted small">{t("form.versionNote")}</p>}
          {formError && <p className="error-text" role="alert">{formError === "geometry" ? t("form.geometryInvalid") : t("form.required")}</p>}
          <button type="submit" className="action-button" disabled={saving}>{saving ? t("form.saving") : replacingSegmentId ? t("form.replacementSubmit") : t("form.submit")}</button>
        </form>
      </section>
      <section className="panel" aria-labelledby="road-segment-list-title">
        <h2 id="road-segment-list-title">{t("list.title")}</h2>
        {segments.length === 0 ? <p>{t("empty")}</p> : <div className="overflow-auto"><table><thead><tr><th>{t("form.name")}</th><th>{t("list.road")}</th><th>{t("list.range")}</th><th>{t("list.version")}</th><th>{t("list.created")}</th><th><span className="sr-only">{t("list.actions")}</span></th></tr></thead><tbody>{segments.map((segment) => {
          const road = roads.find((item) => item.id === segment.roadId);
          return <tr key={segment.id}><th scope="row">{segment.name}</th><td><span>{road?.name ?? t("list.roadUnavailable", { id: segment.roadId })}</span>{road?.code && <small className="road-segment-definition__road-meta">{road.code}{isOpenStreetMapRoad(road) && ` · ${t("source.osmShort")}`}</small>}</td><td>{segment.startLabel}&ndash;{segment.endLabel}{segment.supersedesId && <small className="road-segment-definition__history">{t("list.replaces", { id: segment.supersedesId })}</small>}</td><td>{segment.version}</td><td>{formatDate(segment.createdAt, locale)}</td><td><button type="button" className="button button--small" aria-label={t("list.createReplacementFor", { name: segment.name })} onClick={() => startReplacement(segment)}>{t("list.createReplacement")}</button></td></tr>;
        })}</tbody></table></div>}
        {segmentLoadMoreError && <p className="error-text" role="alert">{t("list.loadMoreError")}</p>}
        {segmentsHasMore && <button type="button" className="button" onClick={() => void loadMoreSegments()} disabled={segmentLoadingMore}>{segmentLoadingMore ? t("list.loadingMore") : t("list.loadMore")}</button>}
      </section>
    </>}
  </div>;
}
