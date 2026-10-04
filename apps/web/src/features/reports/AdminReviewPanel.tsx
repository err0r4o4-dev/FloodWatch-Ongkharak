import { useCallback, useEffect, useRef, useState } from "react";
import { formatDate, formatNumber, useTranslation } from "../../i18n";
import th from "../../i18n/locales/th/reports";
import en from "../../i18n/locales/en/reports";
import { confirmAction, notifyError } from "../../shared/alerts";
import { ApiError, reportsApi, type ReportPage } from "./api";
import { ReportCard } from "./ReportCard";
import type { CommunityReport, ReportStatus } from "./types";

type ReviewError = { key: keyof typeof th };
type ReviewQueue = ReportStatus;
const queues: Array<{ status: ReviewQueue; key: keyof typeof th }> = [
  { status: "PENDING", key: "review.tab.unreviewed" },
  { status: "VERIFIED", key: "review.tab.reviewed" },
  { status: "REJECTED", key: "review.tab.removed" }
];
const pageSize = 20;

function reviewErrorKey(cause: unknown): keyof typeof th {
  return cause instanceof ApiError && cause.status === 409 ? "review.conflict" : "review.saveError";
}

function canRemove(report: CommunityReport): boolean {
  return report.status === "PENDING" || report.status === "VERIFIED";
}

export function AdminReviewPanel() {
  const { locale, t } = useTranslation({ th, en });
  const [queue, setQueue] = useState<ReviewQueue>("PENDING");
  const [offset, setOffset] = useState(0);
  const [page, setPage] = useState<ReportPage | null>(null);
  const [selected, setSelected] = useState<CommunityReport | null>(null);
  const [reason, setReason] = useState("");
  const [error, setError] = useState<ReviewError | null>(null);
  const [loading, setLoading] = useState(true);
  const [detailLoading, setDetailLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const mounted = useRef(false);
  const listRevision = useRef(0);
  const listAbort = useRef<AbortController | null>(null);
  const detailRevision = useRef(0);
  const detailAbort = useRef<AbortController | null>(null);
  const lastDetailOpener = useRef<HTMLElement | null>(null);
  const detailClose = useRef<HTMLButtonElement | null>(null);
  const reviewPending = useRef(false);
  const queueRef = useRef(queue);
  const offsetRef = useRef(offset);
  queueRef.current = queue;
  offsetRef.current = offset;

  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
      listRevision.current += 1;
      detailRevision.current += 1;
      listAbort.current?.abort();
      detailAbort.current?.abort();
    };
  }, []);

  const invalidateList = () => {
    listRevision.current += 1;
    listAbort.current?.abort();
    listAbort.current = null;
  };

  const invalidateDetail = () => {
    detailRevision.current += 1;
    detailAbort.current?.abort();
    detailAbort.current = null;
  };

  const load = useCallback(async (nextQueue: ReviewQueue = queue, nextOffset = offset) => {
    invalidateList();
    const revision = listRevision.current;
    const controller = new AbortController();
    listAbort.current = controller;
    if (!mounted.current) return;
    setLoading(true);
    setError(null);
    setPage(null);
    invalidateDetail();
    setSelected(null);
    setDetailLoading(false);
    setReason("");
    try {
      const result = await reportsApi.listAdmin(nextQueue, pageSize, nextOffset, controller.signal);
      if (!mounted.current || controller.signal.aborted || revision !== listRevision.current) return;
      setPage(result);
    } catch {
      if (mounted.current && !controller.signal.aborted && revision === listRevision.current) {
        setPage(null);
        setError({ key: "review.loadError" });
      }
    } finally {
      if (mounted.current && revision === listRevision.current) {
        setLoading(false);
        listAbort.current = null;
      }
    }
  }, [offset, queue]);

  useEffect(() => { void load(); }, [load]);

  const changeQueue = (nextQueue: ReviewQueue) => {
    if (nextQueue === queue || saving) return;
    invalidateList();
    invalidateDetail();
    setQueue(nextQueue);
    setOffset(0);
    setLoading(true);
    setPage(null);
    setSelected(null);
    setDetailLoading(false);
    setReason("");
    setError(null);
  };

  const changeOffset = (nextOffset: number) => {
    if (nextOffset === offset || saving) return;
    invalidateList();
    invalidateDetail();
    setOffset(nextOffset);
    setLoading(true);
    setPage(null);
    setSelected(null);
    setDetailLoading(false);
    setReason("");
    setError(null);
  };

  const select = async (id: string, opener?: HTMLElement) => {
    if (saving) return;
    invalidateDetail();
    const revision = detailRevision.current;
    const controller = new AbortController();
    detailAbort.current = controller;
    if (opener) lastDetailOpener.current = opener;
    else if (document.activeElement instanceof HTMLElement) lastDetailOpener.current = document.activeElement;
    setSelected(null);
    setDetailLoading(true);
    setError(null);
    setReason("");
    try {
      const result = await reportsApi.getAdminReport(id);
      if (!mounted.current || controller.signal.aborted || revision !== detailRevision.current) return;
      setSelected(result);
      setDetailLoading(false);
    } catch {
      if (mounted.current && !controller.signal.aborted && revision === detailRevision.current) {
        setDetailLoading(false);
        setError({ key: "review.detailError" });
        void notifyError(t("review.detailErrorTitle"), t("review.detailErrorText"));
      }
    }
  };

  const validateReason = () => {
    const trimmed = reason.trim();
    if (!trimmed) {
      setError({ key: "review.reasonRequired" });
      return null;
    }
    if (Array.from(trimmed).length > 500) {
      setError({ key: "review.reasonTooLong" });
      return null;
    }
    return trimmed;
  };

  const review = async (action: "verify" | "remove" | "restore") => {
    if (!mounted.current || !selected || reviewPending.current) return;
    const selectedReport = selected;
    const actionRevision = detailRevision.current;
    const actionReason = action === "verify" ? "" : validateReason();
    if (action !== "verify" && !actionReason) return;
    if (action === "verify" && selectedReport.status !== "PENDING") return;
    if (action === "remove" && !canRemove(selectedReport)) return;
    if (action === "restore" && selectedReport.status !== "REJECTED") return;

    reviewPending.current = true;
    setSaving(true);
    setError(null);
    try {
      await confirmAction({
        title: action === "verify" ? t("review.confirm.verifyTitle") : action === "remove" ? t("review.confirm.rejectTitle") : t("review.confirm.restoreTitle"),
        text: action === "verify" ? t("review.confirm.verifyText") : action === "remove" ? t("review.confirm.rejectText") : t("review.confirm.restoreText"),
        confirmText: action === "verify" ? t("review.verify") : action === "remove" ? t("review.reject") : t("review.restore"),
        danger: action !== "verify",
        action: async () => {
          if (!mounted.current || actionRevision !== detailRevision.current) throw new Error("Review context changed");
          try {
            if (action === "restore") await reportsApi.restore(selectedReport.id, actionReason!, selectedReport.updatedAt);
            else await reportsApi.review(selectedReport.id, action === "remove" ? "reject" : "verify", actionReason ?? "", selectedReport.updatedAt);
            if (!mounted.current || actionRevision !== detailRevision.current) return;
            setSelected(null);
            setReason("");
            await load(queueRef.current, offsetRef.current);
          } catch (cause) {
            const key = reviewErrorKey(cause);
            if (mounted.current && actionRevision === detailRevision.current) {
              setError({ key });
              if (key === "review.conflict") setSelected(null);
            }
            throw cause;
          }
        },
        successTitle: action === "verify" ? t("review.confirm.verifySuccess") : action === "remove" ? t("review.confirm.rejectSuccess") : t("review.confirm.restoreSuccess"),
        errorText: (cause) => t(reviewErrorKey(cause))
      });
    } finally {
      reviewPending.current = false;
      if (mounted.current) setSaving(false);
    }
  };

  const closeDetail = () => {
    if (saving) return;
    invalidateDetail();
    setSelected(null);
    setDetailLoading(false);
    setReason("");
    lastDetailOpener.current?.focus({ preventScroll: true });
  };

  useEffect(() => {
    if (!selected) return;
    detailClose.current?.focus({ preventScroll: true });
  }, [selected]);

  const items = page?.items ?? [];
  const hasNext = page ? page.total !== undefined ? offset + page.limit < page.total : items.length === page.limit : false;
  const range = page && items.length > 0 ? t("review.pagination", {
    from: formatNumber(offset + 1, locale),
    to: formatNumber(offset + items.length, locale)
  }) : null;

  return <section className="data-section admin-review" aria-labelledby="admin-review-heading">
    <h2 id="admin-review-heading">{t("review.title")}</h2>
    <p className="section-caption">{t("review.description")}</p>
    <div className="admin-review__tabs" role="group" aria-label={t("review.tabsLabel")}>
      {queues.map((item) => <button key={item.status} type="button" aria-pressed={queue === item.status} disabled={saving} onClick={() => changeQueue(item.status)}>{t(item.key)}</button>)}
    </div>
    {error && <p className="form-error" role="alert">{t(error.key)}</p>}
      {loading ? <p role="status">{t("review.loading")}</p> : items.length === 0 ? <p className="section-empty">{t("review.empty")}</p> : <ul className="data-list admin-review__list">{items.map((report) => <li key={report.id}><button type="button" aria-pressed={selected?.id === report.id} disabled={saving} onClick={(event) => void select(report.id, event.currentTarget)}>{t("review.open", { type: report.type })}</button></li>)}</ul>}
    {page && (offset > 0 || hasNext) && <nav className="admin-review__pagination" aria-label={t("review.tabsLabel")}><span>{range}</span><div><button type="button" className="action-button" disabled={offset === 0 || loading || saving} onClick={() => changeOffset(Math.max(0, offset - page.limit))}>{t("review.previous")}</button><button type="button" className="action-button" disabled={!hasNext || loading || saving} onClick={() => changeOffset(offset + page.limit)}>{t("review.next")}</button></div></nav>}
    {detailLoading && <div className="admin-review__detail admin-review__detail--loading" role="status"><p>{t("review.detailLoading")}</p></div>}
    {selected && <div className="admin-review__detail" role="dialog" aria-labelledby="admin-review-detail-heading" aria-busy={saving} onKeyDown={(event) => { if (event.key === "Escape") { event.preventDefault(); closeDetail(); } }}>
      <div className="admin-review__detail-heading"><h3 id="admin-review-detail-heading">{t("review.open", { type: selected.type })}</h3><button ref={detailClose} type="button" className="action-button" disabled={saving} onClick={closeDetail}>{t("review.close")}</button></div>
      <ReportCard report={selected} />
      {selected.reviewHistory && selected.reviewHistory.length > 0 && <section className="admin-review__history" aria-labelledby="review-history-heading"><h3 id="review-history-heading">{t("review.historyTitle")}</h3><ol>{selected.reviewHistory.map((entry) => <li key={entry.id}><strong>{t(`review.historyAction.${entry.action}` as keyof typeof th)}</strong><p>{t("review.historyActor", { actor: entry.actorId })}</p><p>{t(entry.reason ? "review.historyReason" : "review.historyNoReason", entry.reason ? { reason: entry.reason } : undefined)}</p><time dateTime={entry.createdAt}>{t("review.historyTime")}: {formatDate(entry.createdAt, locale)}</time></li>)}</ol></section>}
      {(canRemove(selected) || selected.status === "REJECTED") && <label htmlFor="report-review-reason">{selected.status === "REJECTED" ? t("review.restoreReason") : t("review.reason")}<textarea id="report-review-reason" value={reason} maxLength={500} placeholder={t("review.reasonPlaceholder")} onChange={(event) => { setReason(event.target.value); setError(null); }} /></label>}
      <div className="admin-review__actions">
        {selected.status === "PENDING" && <button type="button" className="action-button" disabled={saving} onClick={() => void review("verify")}>{t("review.verify")}</button>}
        {canRemove(selected) && <button type="button" className="action-button action-button--danger" disabled={saving} onClick={() => void review("remove")}>{t("review.reject")}</button>}
        {selected.status === "REJECTED" && <button type="button" className="action-button" disabled={saving} onClick={() => void review("restore")}>{t("review.restore")}</button>}
      </div>
    </div>}
  </section>;
}
