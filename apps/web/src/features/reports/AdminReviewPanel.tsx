import { useEffect, useRef, useState } from "react";
import { useTranslation } from "../../i18n";
import th from "../../i18n/locales/th/reports";
import en from "../../i18n/locales/en/reports";
import { confirmAction, notifyError } from "../../shared/alerts";
import { ApiError, reportsApi } from "./api";
import { ReportCard } from "./ReportCard";
import type { CommunityReport } from "./types";

type ReviewError = { key: keyof typeof th };

function reviewErrorKey(cause: unknown): keyof typeof th {
  return cause instanceof ApiError && cause.status === 409 ? "review.conflict" : "review.saveError";
}

export function AdminReviewPanel() {
  const { t } = useTranslation({ th, en });
  const [items, setItems] = useState<CommunityReport[]>([]);
  const [selected, setSelected] = useState<CommunityReport | null>(null);
  const [reason, setReason] = useState("");
  const [error, setError] = useState<ReviewError | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const reviewPending = useRef(false);

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const page = await reportsApi.listPending();
      setItems(page.items);
    } catch {
      setError({ key: "review.loadError" });
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { void load(); }, []);

  const select = async (id: string) => {
    setError(null);
    try {
      setSelected(await reportsApi.getAdminReport(id));
      setReason("");
    } catch {
      setError({ key: "review.detailError" });
      void notifyError(t("review.detailErrorTitle"), t("review.detailErrorText"));
    }
  };

  const review = async (action: "verify" | "reject") => {
    if (!selected || reviewPending.current) return;
    reviewPending.current = true;
    setSaving(true);
    setError(null);
    try {
      await confirmAction({
        title: action === "verify" ? t("review.confirm.verifyTitle") : t("review.confirm.rejectTitle"),
        text: action === "verify" ? t("review.confirm.verifyText") : t("review.confirm.rejectText"),
        confirmText: action === "verify" ? t("review.verify") : t("review.reject"),
        danger: action === "reject",
        action: async () => {
          try {
            await reportsApi.review(selected.id, action, reason.trim(), selected.updatedAt);
            setItems((current) => current.filter((report) => report.id !== selected.id));
            setSelected(null);
            setReason("");
          } catch (cause) {
            const key = reviewErrorKey(cause);
            setError({ key });
            if (key === "review.conflict") setSelected(null);
            throw cause;
          }
        },
        successTitle: action === "verify" ? t("review.confirm.verifySuccess") : t("review.confirm.rejectSuccess"),
        errorText: (cause) => t(reviewErrorKey(cause))
      });
    } finally {
      reviewPending.current = false;
      setSaving(false);
    }
  };

  return <section className="data-section admin-review" aria-labelledby="admin-review-heading">
    <h2 id="admin-review-heading">{t("review.title")}</h2>
    <p className="section-caption">{t("review.description")}</p>
    {error && <p className="form-error" role="alert">{t(error.key)}</p>}
    {loading ? <p role="status">{t("review.loading")}</p> : items.length === 0 ? <p className="section-empty">{t("review.empty")}</p> : <ul className="data-list admin-review__list">{items.map((report) => <li key={report.id}><button type="button" onClick={() => void select(report.id)}>{t("review.open", { type: report.type })}</button></li>)}</ul>}
    {selected && <div className="admin-review__detail"><ReportCard report={selected} /><label>{t("review.reason")}<textarea value={reason} maxLength={500} onChange={(event) => setReason(event.target.value)} /></label><div className="admin-review__actions"><button type="button" className="action-button" disabled={saving} onClick={() => void review("verify")}>{t("review.verify")}</button><button type="button" className="action-button action-button--danger" disabled={saving} onClick={() => void review("reject")}>{t("review.reject")}</button></div></div>}
  </section>;
}
