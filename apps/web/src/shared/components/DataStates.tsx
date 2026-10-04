import type { ReactNode } from "react";
import type { Freshness, SourceType } from "../types/flood";
import { formatDate, useTranslation } from "../../i18n";
import th from "../../i18n/locales/th/data";
import en from "../../i18n/locales/en/data";
const messages = { th, en };

export function SourceBadge({ type }: { type: SourceType }) {
  const { t } = useTranslation(messages);
  const labels: Record<SourceType, string> = { OFFICIAL: t("official"), COMMUNITY: t("community"), AI: t("ai") };
  return <span className={`badge badge--source-${type.toLowerCase()}`}>{labels[type] ?? t("sourceUnknown")}</span>;
}

export function FreshnessBadge({ freshness }: { freshness: Freshness }) {
  const { t } = useTranslation(messages);
  const labels: Record<Freshness, string> = { FRESH: t("fresh"), STALE: t("stale"), EXPIRED: t("expired"), UNKNOWN: t("freshnessUnknown") };
  return <span className={`badge badge--freshness-${freshness.toLowerCase()}`}>{labels[freshness] ?? t("freshnessUnknown")}</span>;
}

export function StatusBadge({ status }: { status: string }) {
  const { t } = useTranslation(messages);
  const statuses = ["INFO", "WATCH", "WARNING", "CRITICAL", "ACTIVE", "RESOLVED", "LOW", "MODERATE", "HIGH", "VERY_HIGH", "PASSABLE", "SMALL_VEHICLE_NOT_RECOMMENDED", "CLOSED", "UNKNOWN"] as const;
  const key = statuses.find((value) => value === status) ?? "UNKNOWN";
  return <span className={`badge badge--status-${status.toLowerCase().replace(/[^a-z_]/g, "")}`}>{t(key)}</span>;
}

export function DataTimestamp({ value, label }: { value: string | null; label?: string }) {
  const { locale, t } = useTranslation(messages);
  const valid = value && !Number.isNaN(new Date(value).getTime());
  return <span className="data-timestamp">{label ?? t("observedAt")}: {valid ? <time dateTime={value}>{formatDate(value, locale)}</time> : t("unspecified")}</span>;
}

export function OriginalContentNote() {
  const { t } = useTranslation(messages);
  return <p className="section-caption">{t("original")}</p>;
}

export function EmptyState({ title, children }: { title: string; children: ReactNode }) {
  return <div className="state-panel"><span className="state-panel__symbol" aria-hidden="true">○</span><h2>{title}</h2><p>{children}</p></div>;
}

export function ErrorState({ onRetry }: { onRetry: () => void }) {
  const { t } = useTranslation(messages);
  return <div className="state-panel" role="alert"><h2>{t("loadError")}</h2><p>{t("loadErrorHelp")}</p><button type="button" className="action-button" onClick={onRetry}>{t("retry")}</button></div>;
}

export function LoadingState() {
  const { t } = useTranslation(messages);
  return <div className="state-panel" role="status" aria-label={t("loading")}><h2>{t("loading")}</h2><p>{t("loadingHelp")}</p></div>;
}
