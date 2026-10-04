import { useEffect, useRef, useState } from "react";
import { Link } from "react-router";
import { formatDate, formatNumber, useTranslation } from "../../i18n";
import th from "../../i18n/locales/th/admin";
import en from "../../i18n/locales/en/admin";
import { ApiError } from "../auth/api";
import { ReportMediaImage } from "../reports/ReportMediaImage";
import { confirmAction } from "../../shared/alerts";
import { AI_EVIDENCE_PAGE_SIZE, aiApi } from "./ai-api";
import type { AIEvidence, AIEvidencePage, RoadOption } from "./ai-api";
import { formatAdminDate, formatAdminNumber } from "./localization";
import type { AdminErrorState } from "./localization";

type LoadState = "loading" | "ready" | "error";

const evidenceStatusLabels: Record<AIEvidence["status"], string> = {
  UNVERIFIED: "ai.status.unverified",
  VERIFIED: "ai.status.verified",
  REJECTED: "ai.status.rejected"
};

const statusKeys: Record<string, keyof typeof th> = {
  INFO: "ai.status.INFO",
  WATCH: "ai.status.WATCH",
  WARNING: "ai.status.WARNING",
  CRITICAL: "ai.status.CRITICAL",
  ACTIVE: "ai.status.ACTIVE",
  RESOLVED: "ai.status.RESOLVED",
  LOW: "ai.status.LOW",
  MODERATE: "ai.status.MODERATE",
  HIGH: "ai.status.HIGH",
  VERY_HIGH: "ai.status.VERY_HIGH",
  PASSABLE: "ai.status.PASSABLE",
  SMALL_VEHICLE_NOT_RECOMMENDED: "ai.status.SMALL_VEHICLE_NOT_RECOMMENDED",
  CLOSED: "ai.status.CLOSED",
  UNKNOWN: "ai.status.UNKNOWN",
};

function confidenceLabel(value: number, locale: "th" | "en") {
  return `${formatNumber(Math.round(value * 100), locale)}%`;
}

function depthLabel(value: number | null, locale: "th" | "en", t: (key: keyof typeof th) => string) {
  return value === null ? t("common.unknown") : `${formatNumber(value, locale, { minimumFractionDigits: 2, maximumFractionDigits: 2 })} ${t("common.meters")}`;
}

function evidenceStatus(status: AIEvidence["status"], t: (key: keyof typeof th) => string) {
  return <span className="badge badge--status-unknown">{t(evidenceStatusLabels[status] as keyof typeof th)}</span>;
}

function evidenceResultStatus(status: string, t: (key: keyof typeof th) => string) {
  return <span className={`badge badge--status-${status.toLowerCase().replace(/[^a-z_]/g, "")}`}>{t(statusKeys[status] ?? "ai.status.UNKNOWN")}</span>;
}

export function AIEvidenceReview() {
  const { locale, t } = useTranslation({ th, en });
  const [page, setPage] = useState<AIEvidencePage | null>(null);
  const [listState, setListState] = useState<LoadState>("loading");
  const [listError, setListError] = useState(false);
  const [offset, setOffset] = useState(0);
  const [retry, setRetry] = useState(0);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [detail, setDetail] = useState<AIEvidence | null>(null);
  const [detailState, setDetailState] = useState<LoadState>("ready");
  const [roadOptions, setRoadOptions] = useState<RoadOption[]>([]);
  const [roadState, setRoadState] = useState<LoadState>("ready");
  const [roadId, setRoadId] = useState("");
  const [rejectionReasons, setRejectionReasons] = useState<Record<string, string>>({});
  const [validationError, setValidationError] = useState<AdminErrorState | null>(null);
  const [conflictId, setConflictId] = useState<string | null>(null);
  const [reviewingId, setReviewingId] = useState<string | null>(null);
  const listRequest = useRef(0);
  const detailRequest = useRef(0);
  const reviewPending = useRef(false);

  useEffect(() => {
    const requestId = ++listRequest.current;
    const controller = new AbortController();
    setListState("loading");
    setListError(false);

    void aiApi.list({ limit: AI_EVIDENCE_PAGE_SIZE, offset, signal: controller.signal })
      .then((nextPage) => {
        if (requestId !== listRequest.current) return;
        setPage(nextPage);
        setListState("ready");
      })
      .catch(() => {
        if (requestId !== listRequest.current) return;
        setListError(true);
        setListState("error");
      });

    return () => {
      controller.abort();
    };
  }, [offset, retry]);

  const loadRoadOptions = async () => {
    setRoadState("loading");
    try {
      setRoadOptions(await aiApi.listRoads({ limit: 20, offset: 0 }));
      setRoadState("ready");
    } catch {
      setRoadState("error");
    }
  };

  const openDetails = async (id: string) => {
    const requestId = ++detailRequest.current;
    setSelectedId(id);
    setDetail(null);
    setDetailState("loading");
    setRoadOptions([]);
    setRoadState("ready");
    setRoadId("");
    setValidationError(null);
    setConflictId(null);
    try {
      const nextDetail = await aiApi.get(id);
      if (requestId !== detailRequest.current) return;
      setDetail(nextDetail);
      setDetailState("ready");
      if (nextDetail.road) void loadRoadOptions();
    } catch {
      if (requestId === detailRequest.current) setDetailState("error");
    }
  };

  const review = async (action: "verify" | "reject") => {
    if (!detail || detail.status !== "UNVERIFIED" || reviewPending.current) return;
    const reason = rejectionReasons[detail.id]?.trim() ?? "";
    if (action === "reject" && !reason) {
      setValidationError({ key: "ai.validation.rejectionReason" });
      return;
    }
    if (action === "verify" && detail.road && (!roadId || roadState !== "ready")) {
      setValidationError({ key: "ai.validation.road" });
      return;
    }

    reviewPending.current = true;
    setReviewingId(detail.id);
    setValidationError(null);
      setConflictId(null);
      try {
        const confirmed = await confirmAction({
        title: action === "verify" ? t("ai.confirm.verify.title") : t("ai.confirm.reject.title"),
        text: action === "verify" ? t("ai.confirm.verify.text") : t("ai.confirm.reject.text"),
        confirmText: action === "verify" ? t("ai.confirm.verify.confirm") : t("ai.confirm.reject.confirm"),
        danger: action === "reject",
        action: async () => {
          try {
            if (action === "verify") {
              await aiApi.verify(detail.id, detail.road ? { roadId } : {});
            } else {
              await aiApi.reject(detail.id, { reason });
            }
          } catch (cause) {
            if (cause instanceof ApiError && cause.status === 409) setConflictId(detail.id);
            throw cause;
          }
        },
        successTitle: action === "verify" ? t("ai.confirm.verify.success") : t("ai.confirm.reject.success"),
        errorText: t("ai.confirm.error")
      });
      if (confirmed) {
        setSelectedId(null);
        setDetail(null);
        setRetry((current) => current + 1);
      }
    } finally {
      reviewPending.current = false;
      setReviewingId(null);
    }
  };

  const renderDetail = () => {
    if (!selectedId) return null;
    if (detailState === "loading") return <p className="panel" role="status">{t("ai.detail.loading")}</p>;
    if (detailState === "error" || !detail) return <div className="panel error-state" role="alert"><p>{t("ai.detail.error")}</p><button type="button" className="action-button" onClick={() => void openDetails(selectedId)}>{t("ai.detail.retry")}</button></div>;

    const canReview = detail.status === "UNVERIFIED";
    const selectedReason = rejectionReasons[detail.id] ?? "";
    return <section className="panel space-y-4" aria-labelledby="ai-evidence-detail-heading" aria-busy={reviewingId === detail.id}>
      <div className="flex items-start justify-between gap-4">
        <div>
        <p className="eyebrow">{t("ai.eyebrow")}</p>
          <h2 id="ai-evidence-detail-heading">{t("ai.detail.title")}</h2>
        </div>
        {evidenceStatus(detail.status, t)}
      </div>
      <div className="badge-row"><span className="badge badge--source-ai">{t("ai.status.sourceLabel")}</span><span className="badge">{t("ai.modelVersion", { version: detail.modelVersion })}</span></div>
      <div className="report-media"><ReportMediaImage media={{ id: detail.mediaId, url: `/api/v1/reports/${encodeURIComponent(detail.reportId)}/media/${encodeURIComponent(detail.mediaId)}` }} /></div>
      <dl className="grid gap-2 sm:grid-cols-2">
        <div><dt>{t("ai.sourceReportTime")}</dt><dd><time dateTime={detail.observedAt}>{formatAdminDate(detail.observedAt, locale, formatDate)}</time></dd></div>
        <div><dt>{t("ai.receivedTime")}</dt><dd><time dateTime={detail.receivedAt}>{formatAdminDate(detail.receivedAt, locale, formatDate)}</time></dd></div>
      </dl>
      <p><Link to={`/reports?report=${encodeURIComponent(detail.reportId)}`}>{t("ai.relatedReport")}</Link></p>

      <div className="grid gap-4 md:grid-cols-2">
        <section aria-labelledby="ai-flood-result-heading" className="panel">
          <h3 id="ai-flood-result-heading">{t("ai.flood.title")}</h3>
          {detail.flood ? <dl className="grid gap-2"><div><dt>{t("ai.level")}</dt><dd>{evidenceResultStatus(detail.flood.severity, t)}</dd></div><div><dt>{t("ai.waterDepth")}</dt><dd>{depthLabel(detail.flood.water_depth_m, locale, t)}</dd></div><div><dt>{t("ai.confidence")}</dt><dd>{confidenceLabel(detail.flood.confidence, locale)}</dd></div><div><dt>{t("ai.boundingBox")}</dt><dd>{detail.flood.bounding_box ? t("ai.hasBoundingBox") : t("ai.noBoundingBox")}</dd></div></dl> : <p>{t("ai.noFloodResult")}</p>}
        </section>
        <section aria-labelledby="ai-road-result-heading" className="panel">
          <h3 id="ai-road-result-heading">{t("ai.road.title")}</h3>
          {detail.road ? <dl className="grid gap-2"><div><dt>{t("ai.roadStatus")}</dt><dd>{evidenceResultStatus(detail.road.status, t)}</dd></div><div><dt>{t("ai.confidence")}</dt><dd>{confidenceLabel(detail.road.confidence, locale)}</dd></div><div><dt>{t("ai.boundingBox")}</dt><dd>{detail.road.bounding_box ? t("ai.hasBoundingBox") : t("ai.noBoundingBox")}</dd></div></dl> : <p>{t("ai.noRoadResult")}</p>}
        </section>
      </div>

      <section aria-labelledby="ai-limitations-heading">
        <h3 id="ai-limitations-heading">{t("ai.limitations.title")}</h3>
        {detail.limitations.length ? <ul>{detail.limitations.map((limitation, index) => <li key={`${detail.id}-limitation-${index}`}>{limitation}</li>)}</ul> : <p>{t("ai.noLimitations")}</p>}
      </section>

      {detail.road && canReview && <div className="grid gap-2"><label htmlFor="ai-road-match">{t("ai.roadMatch.label")}</label><select id="ai-road-match" value={roadId} disabled={roadState !== "ready" || reviewingId === detail.id} onChange={(event) => { setRoadId(event.target.value); setValidationError(null); }}><option value="">{t("ai.roadMatch.placeholder")}</option>{roadOptions.map((road) => <option key={road.id} value={road.id}>{road.name}</option>)}</select>{roadState === "loading" && <p role="status">{t("ai.roadMatch.loading")}</p>}{roadState === "error" && <div role="alert"><p>{t("ai.roadMatch.error")}</p><button type="button" className="action-button" onClick={() => void loadRoadOptions()}>{t("ai.roadMatch.retry")}</button></div>}<p className="muted">{t("ai.roadMatch.note")}</p></div>}

      {canReview && <div className="grid gap-3"><label htmlFor="ai-rejection-reason">{t("ai.rejection.label")}</label><textarea id="ai-rejection-reason" value={selectedReason} maxLength={500} disabled={reviewingId === detail.id} onChange={(event) => { setRejectionReasons((current) => ({ ...current, [detail.id]: event.target.value })); setValidationError(null); }} rows={3} placeholder={t("ai.rejection.placeholder")} />{validationError && <p role="alert">{t(validationError.key, validationError.params)}</p>}<div className="flex flex-wrap gap-2"><button type="button" className="action-button" disabled={!canReview || reviewingId === detail.id || Boolean(detail.road && (!roadId || roadState !== "ready"))} onClick={() => void review("verify")}>{t("ai.verify")}</button><button type="button" className="action-button action-button--danger" disabled={!canReview || reviewingId === detail.id} onClick={() => void review("reject")}>{t("ai.reject")}</button></div></div>}
    </section>;
  };

  return <div className="page-container admin-page space-y-4">
    <header className="page-header"><p className="eyebrow">{t("ai.eyebrow")}</p><h1>{t("ai.title")}</h1><p>{t("ai.description")}</p></header>
    {conflictId && <div className="panel error-state" role="alert"><p>{t("ai.conflict")}</p><button type="button" className="action-button" onClick={() => { setConflictId(null); setSelectedId(null); setDetail(null); setRetry((current) => current + 1); }}>{t("common.reload")}</button></div>}
    {listState === "loading" && <p role="status">{t("ai.loading")}</p>}
    {listError && <div className="panel error-state" role="alert"><p>{t("ai.error")}</p><button type="button" className="action-button" onClick={() => setRetry((current) => current + 1)}>{t("ai.retry")}</button></div>}
    {listState === "ready" && page?.items.length === 0 && <div className="panel"><p>{t("ai.empty")}</p></div>}
    {listState === "ready" && page && page.items.length > 0 && <section aria-labelledby="ai-evidence-list-heading"><h2 id="ai-evidence-list-heading">{t("ai.list.title")}</h2><div className="grid gap-4">{page.items.map((item) => <article key={item.id} className="panel"><div className="flex items-start justify-between gap-4"><div><h3>{t("ai.item", { id: item.id })}</h3><div className="badge-row"><span className="badge badge--source-ai">{t("ai.status.sourceLabel")}</span>{evidenceStatus(item.status, t)}</div></div><span className="badge">{t("ai.modelVersion", { version: item.modelVersion })}</span></div><p>{item.flood ? t("ai.floodSummary", { severity: t(statusKeys[item.flood.severity] ?? "ai.status.UNKNOWN"), confidence: confidenceLabel(item.flood.confidence, locale) }) : t("ai.noFloodShort")}{item.road ? ` · ${t("ai.roadSummary", { status: t(statusKeys[item.road.status] ?? "ai.status.UNKNOWN"), confidence: confidenceLabel(item.road.confidence, locale) })}` : ` · ${t("ai.noRoadShort")}`}</p><p><time dateTime={item.receivedAt}>{t("ai.receivedLabel")}: {formatAdminDate(item.receivedAt, locale, formatDate)}</time></p><div className="flex flex-wrap items-center gap-3"><Link to={`/reports?report=${encodeURIComponent(item.reportId)}`}>{t("ai.relatedReport")}</Link><button type="button" className="action-button" onClick={() => void openDetails(item.id)}>{t("ai.openDetails", { id: item.id })}</button></div></article>)}</div></section>}
    {listState === "ready" && page && page.pagination.total > 0 && <nav className="flex flex-wrap items-center justify-between gap-3" aria-label={t("ai.paginationLabel")}><span>{t("common.showingShort", { from: formatAdminNumber(page.items.length ? page.pagination.offset + 1 : 0, locale, formatNumber), to: formatAdminNumber(Math.min(page.pagination.offset + page.items.length, page.pagination.total), locale, formatNumber), total: formatAdminNumber(page.pagination.total, locale, formatNumber) })}</span><div className="flex gap-2"><button type="button" className="action-button" aria-label={t("common.pagePrevious")} disabled={page.pagination.offset === 0} onClick={() => { setOffset(Math.max(0, page.pagination.offset - page.pagination.limit)); setSelectedId(null); setDetail(null); }}>{t("common.pagePrevious")}</button><button type="button" className="action-button" aria-label={t("common.pageNext")} disabled={page.pagination.offset + page.pagination.limit >= page.pagination.total} onClick={() => { setOffset(page.pagination.offset + page.pagination.limit); setSelectedId(null); setDetail(null); }}>{t("common.pageNext")}</button></div></nav>}
    {renderDetail()}
  </div>;
}
