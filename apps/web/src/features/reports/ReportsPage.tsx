import { useEffect, useRef, useState } from "react";
import { Link, useSearchParams } from "react-router";
import { formatNumber, useTranslation } from "../../i18n";
import th from "../../i18n/locales/th/reports";
import en from "../../i18n/locales/en/reports";
import { PageHeroArt } from "../../shared/components/PageHeroArt";
import { DataTimestamp, OriginalContentNote, SourceBadge } from "../../shared/components/DataStates";
import { useRealtimeRevision } from "../realtime/RealtimeProvider";
import { reportsApi } from "./api";
import { ReportCard } from "./ReportCard";
import { ReportMediaImage } from "./ReportMediaImage";
import "./ReportsPage.css";
import type { CommunityReport, ReportType } from "./types";

type LoadState = "loading" | "ready" | "error";
type ReportFilter = "ALL" | ReportType;

const reportEvents = ["report.verified"] as const;
const reportFilters: Array<{ value: ReportFilter; key: keyof typeof th }> = [
  { value: "ALL", key: "type.all" },
  { value: "FLOOD", key: "type.flood" },
  { value: "ROAD", key: "type.road" },
  { value: "OTHER", key: "type.other" }
];
const reportTypeKeys: Record<ReportType, keyof typeof th> = {
  FLOOD: "typeLabel.FLOOD",
  ROAD: "typeLabel.ROAD",
  OTHER: "typeLabel.OTHER"
};

function ReportsIcon({ kind }: { kind: ReportFilter | "SEARCH" | "PLUS" }) {
  const paths = {
    ALL: "M8 3h8v4H8z M6 5H4v16h16V5h-2 M8 12h8 M8 16h5",
    FLOOD: "M12 3s-6 6.5-6 11a6 6 0 0 0 12 0c0-4.5-6-11-6-11z M9 15a3 3 0 0 0 3 3",
    ROAD: "M7 3 3 21 M17 3l4 18 M12 4v3 M12 11v3 M12 18v3",
    OTHER: "M4 4h16v12H9l-5 4z M8 9h8 M8 12h5",
    SEARCH: "M16 16l5 5 M18 10a8 8 0 1 1-16 0 8 8 0 0 1 16 0",
    PLUS: "M12 5v14 M5 12h14"
  };
  return <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true"><path d={paths[kind]} /></svg>;
}

function ReportDetails({ report }: { report: CommunityReport }) {
  const { locale, t } = useTranslation({ th, en });
  const typeLabel = t(reportTypeKeys[report.type]);
  return <aside className="reports-detail" aria-labelledby="report-detail-heading">
    <div className="reports-detail__heading">
      <div>
        <p className="eyebrow">{t("detail.selected")}</p>
        <OriginalContentNote />
        <h2 id="report-detail-heading" tabIndex={-1}>{report.description}</h2>
      </div>
      <span className="badge">{typeLabel}</span>
    </div>
    <div className="badge-row">
      <SourceBadge type="COMMUNITY" />
      <span className="badge badge--report-verified">{t("status.verified")}</span>
    </div>
    <p className="reports-detail__description">{t("detail.verifiedDescription")}</p>
    {report.media.length > 0 && <section className="reports-gallery" aria-labelledby="report-gallery-heading">
      <h3 id="report-gallery-heading">{t("detail.mediaTitle")}</h3>
      <ul className="report-media" aria-label={t("detail.mediaLabel")}>
        {report.media.map((media) => <li key={media.id}><ReportMediaImage media={media} /></li>)}
      </ul>
    </section>}
    <dl className="reports-detail__facts">
      <div><dt>{t("detail.location")}</dt><dd>{report.locationName || t("detail.locationUnknown")}</dd></div>
      {report.locationNote && <div><dt>{t("detail.locationNote")}</dt><dd>{report.locationNote}</dd></div>}
      <div><dt>{t("detail.submitted")}</dt><dd><DataTimestamp value={report.createdAt} label={t("detail.submittedLabel")} /></dd></div>
      {report.verifiedAt && <div><dt>{t("detail.reviewed")}</dt><dd><DataTimestamp value={report.verifiedAt} label={t("detail.reviewedLabel")} /></dd></div>}
      {report.estimatedWaterDepthM !== null && <div><dt>{t("detail.depth")}</dt><dd>{formatNumber(report.estimatedWaterDepthM, locale, { minimumFractionDigits: 2, maximumFractionDigits: 2 })} {t("detail.meters")}</dd></div>}
    </dl>
    <p className="reports-detail__notice">{t("detail.notice")}</p>
  </aside>;
}

function PublicReportListState({ state, onRetry }: { state: LoadState; onRetry: () => void }) {
  const { t } = useTranslation({ th, en });
  if (state === "loading") return <div className="reports-state-panel" role="status"><h3>{t("loading.title")}</h3><p>{t("loading.description")}</p></div>;
  if (state === "error") return <div className="reports-state-panel" role="alert"><h3>{t("error.title")}</h3><p>{t("error.description")}</p><button type="button" className="action-button" onClick={onRetry}>{t("error.retry")}</button></div>;
  return <div className="reports-state-panel"><h3>{t("empty.title")}</h3><p>{t("empty.description")}</p></div>;
}

function ReportDetailsState({ state }: { state: LoadState }) {
  const { t } = useTranslation({ th, en });
  const message = state === "loading" ? t("detail.waiting") : state === "error" ? t("detail.unavailable") : t("detail.availableLater");
  return <aside className="reports-detail reports-detail--state" aria-labelledby="report-detail-heading">
    <p className="eyebrow">{t("detail.title")}</p>
    <h2 id="report-detail-heading">{state === "error" ? t("detail.unavailableTitle") : t("detail.emptyTitle")}</h2>
    <p>{message}</p>
  </aside>;
}

export function ReportsPage() {
  const { locale, t } = useTranslation({ th, en });
  const pageRef = useRef<HTMLElement>(null);
  const [params] = useSearchParams();
  const realtimeRevision = useRealtimeRevision(reportEvents);
  const [publicReports, setPublicReports] = useState<CommunityReport[]>([]);
  const [publicState, setPublicState] = useState<LoadState>("loading");
  const [filter, setFilter] = useState<ReportFilter>("ALL");
  const [query, setQuery] = useState("");
  const [selectedReportId, setSelectedReportId] = useState<string | null>(params.get("report"));
  const publicRequest = useRef(0);
  useEffect(() => { setSelectedReportId(params.get("report")); }, [params]);

  const loadPublic = async () => {
    const requestId = ++publicRequest.current;
    setPublicState("loading");
    try {
      const page = await reportsApi.listPublic();
      if (requestId !== publicRequest.current) return;
      setPublicReports(page.items.filter((report) => report.status === "VERIFIED" && report.verificationStatus === "VERIFIED" && report.sourceType === "COMMUNITY"));
      setPublicState("ready");
    } catch {
      if (requestId === publicRequest.current) setPublicState("error");
    }
  };

  useEffect(() => {
    void loadPublic();
    return () => { publicRequest.current += 1; };
  }, [realtimeRevision]);

  const normalizedQuery = query.trim().toLocaleLowerCase(locale);
  const visibleReports = publicReports.filter((report) =>
    (filter === "ALL" || report.type === filter) &&
    (!normalizedQuery || report.description.toLocaleLowerCase(locale).includes(normalizedQuery))
  );
  const selectedReport = visibleReports.find((report) => report.id === selectedReportId) ?? visibleReports[0] ?? null;
  const requestedReportUnavailable = params.get("report") !== null && selectedReportId === params.get("report") && !publicReports.some((report) => report.id === selectedReportId);
  const showReportDetails = (reportId: string) => {
    setSelectedReportId(reportId);
    requestAnimationFrame(() => pageRef.current?.querySelector<HTMLElement>("#report-detail-heading")?.focus());
  };

  return <section ref={pageRef} className="feature-page reports-page">
    <header className="page-hero reports-page__heading">
      <div className="page-hero__copy">
        <p className="eyebrow">{t("eyebrow")}</p>
        <h1>{t("title")}</h1>
        <p>{t("description")}</p>
        <Link className="reports-create-button" to="/reports/new"><ReportsIcon kind="PLUS" />{t("create")}</Link>
      </div>
      <PageHeroArt kind="report" />
    </header>
    <section className="reports-summary" aria-label={t("summary.aria")}>
      <dl className="reports-summary__grid">
        {reportFilters.map((item) => <div key={item.value} className={`reports-summary__item${item.value === "ALL" ? " reports-summary__item--total" : ""}`}>
          <dt><span className="reports-summary__icon"><ReportsIcon kind={item.value} /></span>{t(item.key)}</dt>
          <dd>{publicState === "ready" ? formatNumber(publicReports.filter((report) => item.value === "ALL" || report.type === item.value).length, locale) : "—"}<span> {t("summary.unit")}</span></dd>
        </div>)}
      </dl>
      <p className="reports-summary__caption">{publicState === "ready" ? t("summary.caption") : publicState === "loading" ? t("summary.loading") : t("summary.unavailable")}</p>
    </section>
    <section className="data-section reports-public" aria-labelledby="public-reports-heading">
      <div className="reports-public__heading">
        <div><p className="eyebrow">{t("public.eyebrow")}</p><h2 id="public-reports-heading">{t("public.title")}</h2><p className="section-caption">{t("public.caption")}</p></div>
        <label className="reports-search"><span>{t("public.search.label")}</span><span className="reports-search__field"><ReportsIcon kind="SEARCH" /><input type="search" value={query} onChange={(event) => setQuery(event.target.value)} placeholder={t("public.search.placeholder")} /></span></label>
      </div>
      <div className="reports-toolbar">
        <div className="reports-filters" aria-label={t("public.filters.label")}>
          {reportFilters.map((item) => <button key={item.value} type="button" aria-pressed={filter === item.value} onClick={() => setFilter(item.value)}><ReportsIcon kind={item.value} />{t(item.key)}<span className="reports-filter-count" aria-hidden="true">{publicState === "ready" ? formatNumber(publicReports.filter((report) => item.value === "ALL" || report.type === item.value).length, locale) : "—"}</span></button>)}
        </div>
        <p className="reports-result-count" aria-live="polite">{publicState === "ready" ? t("public.resultCount", { visible: formatNumber(visibleReports.length, locale), total: formatNumber(publicReports.length, locale) }) : t("public.resultUnavailable")}</p>
      </div>
      <div className="reports-browser">
        {publicState === "ready" && publicReports.length > 0
          ? visibleReports.length > 0
            ? <ul className="data-list reports-list">{visibleReports.map((report) => <li key={report.id}><ReportCard report={report} showDetailsControl isSelected={!requestedReportUnavailable && selectedReport?.id === report.id} onShowDetails={showReportDetails} /></li>)}</ul>
            : <div className="reports-state-panel"><h3>{t("public.noMatch.title")}</h3><p>{t("public.noMatch.description")}</p><button type="button" className="action-button" onClick={() => { setQuery(""); setFilter("ALL"); }}>{t("public.clearFilters")}</button></div>
          : <div className="reports-list reports-list--state"><PublicReportListState state={publicState} onRetry={() => void loadPublic()} /></div>}
        {publicState !== "ready" || publicReports.length === 0
          ? publicState === "ready" && requestedReportUnavailable
            ? <aside className="reports-detail reports-detail--state" aria-labelledby="report-detail-heading"><h2 id="report-detail-heading">{t("public.selectedUnavailable.title")}</h2><p>{t("public.selectedUnavailable.missing")}</p></aside>
            : <ReportDetailsState state={publicState} />
          : requestedReportUnavailable
            ? <aside className="reports-detail reports-detail--state" aria-labelledby="report-detail-heading"><h2 id="report-detail-heading">{t("public.selectedUnavailable.title")}</h2><p>{t("public.selectedUnavailable.chooseOther")}</p></aside>
          : visibleReports.length === 0
            ? <aside className="reports-detail reports-detail--state" aria-labelledby="report-detail-heading"><h2 id="report-detail-heading" className="section-empty reports-filter-empty">{t("public.filterUnavailable")}</h2></aside>
            : selectedReport && <ReportDetails report={selectedReport} />}
      </div>
    </section>
  </section>;
}
