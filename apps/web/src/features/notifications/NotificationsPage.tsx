import { useEffect, useRef, useState } from "react";
import type { ReactNode } from "react";
import { Link } from "react-router";
import { formatDate, formatNumber, useTranslation } from "../../i18n";
import th from "../../i18n/locales/th/notifications";
import en from "../../i18n/locales/en/notifications";
import { useRealtimeRevision } from "../realtime/RealtimeProvider";
import { NOTIFICATION_PAGE_SIZE, notificationApi } from "./api";
import type { PublicNotification, PublicNotificationPage } from "./api";
import "./NotificationsPage.css";

type IconName = "bell" | "settings" | "clock" | "info" | "arrow" | "megaphone";

function NotificationIcon({ name }: { name: IconName }) {
  const paths: Record<IconName, ReactNode> = {
    bell: <><path d="M18 8a6 6 0 0 0-12 0c0 7-3 7-3 9h18c0-2-3-2-3-9" /><path d="M10 21h4" /></>,
    settings: <><path d="M3 7h4m4 0h10M3 17h10m4 0h4" /><circle cx="9" cy="7" r="2" /><circle cx="15" cy="17" r="2" /></>,
    clock: <><circle cx="12" cy="12" r="9" /><path d="M12 7v5l3 2" /></>,
    info: <><circle cx="12" cy="12" r="9" /><path d="M12 11v5M12 8h.01" /></>,
    arrow: <><path d="M4 12h16m-6-6 6 6-6 6" /></>,
    megaphone: <><path d="m4 10 14-5v14L4 14v-4ZM18 9l3-1v8l-3-1M7 15l2 5h3l-2-4" /></>
  };
  return <svg viewBox="0 0 24 24" width="24" height="24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">{paths[name]}</svg>;
}

const publicEvents = ["alert.created", "report.verified"] as const;
const publicReadStorageKey = "floodwatch.public-notifications.read";
const publicAreaStorageKey = "floodwatch.public-notifications.area";
const publicFilters: Array<PublicNotification["severity"] | "ALL"> = ["ALL", "CRITICAL", "WARNING", "WATCH", "INFO"];

function readPublicNotificationIds(): Set<string> {
  try {
    const value: unknown = JSON.parse(localStorage.getItem(publicReadStorageKey) ?? "[]");
    return new Set(Array.isArray(value) && value.every((item) => typeof item === "string") ? value : []);
  } catch {
    return new Set<string>();
  }
}

function readPublicArea(): string {
  try {
    return localStorage.getItem(publicAreaStorageKey)?.trim() ?? "";
  } catch {
    return "";
  }
}

function publicPageIsAtEnd(page: PublicNotificationPage) {
  return page.pagination.offset + page.pagination.limit >= page.pagination.total;
}

function formatNotificationDate(value: string, locale: "th" | "en") {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : formatDate(date, locale);
}

export function NotificationsPage() {
  const { locale, t } = useTranslation({ th, en });
  const realtimeRevision = useRealtimeRevision(publicEvents);
  const headingRef = useRef<HTMLHeadingElement>(null);
  const [page, setPage] = useState<PublicNotificationPage | null>(null);
  const [offset, setOffset] = useState(0);
  const [filter, setFilter] = useState<PublicNotification["severity"] | "ALL">("ALL");
  const [area, setArea] = useState(() => readPublicArea());
  const [areaInput, setAreaInput] = useState(() => readPublicArea());
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);
  const [retry, setRetry] = useState(0);
  const [readIds, setReadIds] = useState<Set<string>>(() => readPublicNotificationIds());

  useEffect(() => { headingRef.current?.focus(); }, []);

  useEffect(() => {
    const controller = new AbortController();
    setPage(null);
    setLoading(true);
    setError(false);
    void notificationApi.listPublic({ limit: NOTIFICATION_PAGE_SIZE, offset, bbox: area || undefined, signal: controller.signal }).then((nextPage) => {
      if (!controller.signal.aborted) setPage(nextPage);
    }).catch(() => {
      if (!controller.signal.aborted) setError(true);
    }).finally(() => {
      if (!controller.signal.aborted) setLoading(false);
    });
    return () => controller.abort();
  }, [area, offset, realtimeRevision, retry]);

  const markRead = (id: string) => {
    setReadIds((current) => {
      const next = new Set(current);
      next.add(id);
      try { localStorage.setItem(publicReadStorageKey, JSON.stringify([...next])); } catch { return next; }
      return next;
    });
  };

  const applyArea = (value: string) => {
    const next = value.trim();
    setAreaInput(next);
    setArea(next);
    setOffset(0);
    try {
      if (next) localStorage.setItem(publicAreaStorageKey, next);
      else localStorage.removeItem(publicAreaStorageKey);
    } catch { }
  };

  const visibleItems = page?.items.filter((item) => filter === "ALL" || item.severity === filter) ?? [];
  const renderControls = () => <div className="notifications-public-toolbar">
    <div className="notifications-public-filters" aria-label={t("public.pageFilters")}>
      {publicFilters.map((value) => <button key={value} type="button" className="notifications-page-button" aria-pressed={filter === value} onClick={() => setFilter(value)}>{value === "ALL" ? t("public.all") : t(({ CRITICAL: "critical", WARNING: "warning", WATCH: "watch", INFO: "information" } as const)[value])}</button>)}
    </div>
    <form className="notifications-public-area" onSubmit={(event) => { event.preventDefault(); applyArea(areaInput); }}>
      <label htmlFor="public-notifications-bbox">{t("public.area")}</label>
      <input id="public-notifications-bbox" aria-describedby="public-notifications-bbox-help" value={areaInput} onChange={(event) => setAreaInput(event.target.value)} placeholder="100,13,102,15" inputMode="decimal" />
      <small id="public-notifications-bbox-help">{t("public.areaHelp")}</small><button type="submit" className="notifications-page-button">{t("public.apply")}</button>
      {(areaInput || area) && <button type="button" className="notifications-page-button" onClick={() => applyArea("")}>{t("public.clearArea")}</button>}
    </form>
  </div>;
  const renderFeed = () => {
    if (loading && !page) return <>{renderControls()}<div className="notifications-history notifications-history--state"><span className="notifications-history__icon"><NotificationIcon name="clock" /></span><p role="status">{t("public.loading")}</p></div></>;
    if (error && !page) return <>{renderControls()}<div className="notifications-history notifications-history--state" role="alert"><span className="notifications-history__icon"><NotificationIcon name="info" /></span><h2>{t("public.unavailable")}</h2><button type="button" className="notifications-inline-button" onClick={() => setRetry((current) => current + 1)}>{t("tryAgain")}</button></div></>;
    if (!page || page.items.length === 0) return <>{renderControls()}<div className="notifications-history notifications-history--state"><span className="notifications-history__icon"><NotificationIcon name="bell" /></span><h2>{t("public.empty")}</h2><p>{t("public.emptyDescription")}</p></div></>;

    return <>
      {renderControls()}
      {visibleItems.length === 0 ? <div className="notifications-history notifications-history--state"><p>{t("public.empty")}</p><p>{t("public.noMatchingItems")}</p></div> : <ul className="notifications-list" aria-label={t("public.list")} aria-busy={loading}>
        {visibleItems.map((item) => {
          const read = readIds.has(item.id);
          return <li key={item.id} className={`notification-item${read ? " notification-item--read" : " notification-item--unread"}`}><article aria-labelledby={`public-notification-title-${item.id}`}>
            <div className="notification-item__meta"><span className={`notification-badge notification-badge--${item.severity.toLowerCase()}`}>{t(({ CRITICAL: "critical", WARNING: "warning", WATCH: "watch", INFO: "information" } as const)[item.severity])}</span><span className="notification-item__source">{t(({ OFFICIAL: "official", COMMUNITY: "community", AI: "aiEvidence" } as const)[item.sourceType])}</span><time dateTime={item.createdAt}>{formatNotificationDate(item.createdAt, locale)}</time></div>
            <h3 id={`public-notification-title-${item.id}`}><Link to={item.url}>{item.title}</Link></h3><p>{item.message}</p><p className="section-caption">{t("original")}</p>
            {item.location && <p className="section-caption">{item.location.coordinates[1].toFixed(5)}, {item.location.coordinates[0].toFixed(5)}</p>}
            <div className="notification-item__actions"><span className="notification-read-state">{read ? t("read") : t("unread")}</span>{!read && <button type="button" className="notifications-item-button" onClick={() => markRead(item.id)}>{t("markRead", { title: item.title })}</button>}</div>
          </article></li>;
        })}
      </ul>}
      <nav className="notifications-pagination" aria-label={t("public.pages")}><span>{t("pageRange", { start: formatNumber(page.pagination.total === 0 ? 0 : page.pagination.offset + 1, locale), end: formatNumber(Math.min(page.pagination.offset + page.items.length, page.pagination.total), locale), total: formatNumber(page.pagination.total, locale) })}</span><div className="notifications-pagination__actions"><button type="button" className="notifications-page-button" disabled={page.pagination.offset === 0 || loading} onClick={() => setOffset(Math.max(0, page.pagination.offset - page.pagination.limit))}>{t("previousPage")}</button><button type="button" className="notifications-page-button" disabled={publicPageIsAtEnd(page) || loading} onClick={() => setOffset(page.pagination.offset + page.pagination.limit)}>{t("nextPage")}</button></div></nav>
    </>;
  };

  return <div className="notifications-page">
    <section className="notifications-hero" aria-labelledby="notifications-title">
      <div className="notifications-hero__copy"><p className="eyebrow">{t("updatesForYou")}</p><h1 id="notifications-title" ref={headingRef} tabIndex={-1}>{t("notifications")}</h1><p>{t("public.description")}</p></div>
      <div className="notifications-hero__art" aria-hidden="true"><span className="notifications-hero__tile notifications-hero__tile--small"><NotificationIcon name="megaphone" /></span><span className="notifications-hero__tile notifications-hero__tile--bell"><NotificationIcon name="bell" /></span><span className="notifications-hero__tile notifications-hero__tile--small"><NotificationIcon name="info" /></span></div>
    </section>
    <div className="notifications-page__layout">
      <section className="notifications-history" aria-labelledby="notifications-history-title"><div className="notifications-history__header"><div><p className="notifications-history__eyebrow">{t("public.eyebrow")}</p><h2 id="notifications-history-title">{t("public.title")}</h2></div></div>{renderFeed()}</section>
      <aside className="notifications-page__sidebar" aria-label={t("public.important")}><section className="notifications-card" aria-labelledby="notifications-caution-title"><span className="notifications-card__icon"><NotificationIcon name="settings" /></span><h2 id="notifications-caution-title">{t("public.inPageTitle")}</h2><p>{t("public.inPageOnly")}</p><p>{t("noNotificationsDoesNotMeanTheArea")}</p><Link to="/map" className="notifications-inline-link">{t("viewWaterSituationMap")}<NotificationIcon name="arrow" /></Link></section></aside>
    </div>
  </div>;
}
