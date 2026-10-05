import { useEffect, useId, useState } from "react";
import { formatNumber, useTranslation } from "../../i18n";
import th from "../../i18n/locales/th/manualAnnouncements";
import en from "../../i18n/locales/en/manualAnnouncements";
import { manualAnnouncementApi } from "./api";
import type { AnnouncementPage } from "./api";
import { AnnouncementCard } from "./AnnouncementCard";

const messages = { th, en };
type Result = { state: "loading" | "error" } | { state: "ready"; page: AnnouncementPage };
export function ManualAnnouncementsPanel() {
  const { locale, t } = useTranslation(messages);
  const headingId = useId();
  const [offset, setOffset] = useState(0);
  const [refresh, setRefresh] = useState(0);
  const [result, setResult] = useState<Result>({ state: "loading" });

  useEffect(() => {
    const controller = new AbortController();
    setResult({ state: "loading" });
    const timeout = window.setTimeout(() => {
      setResult({ state: "error" });
      controller.abort();
    }, 10000);
    void manualAnnouncementApi.listPublic(offset, controller.signal).then((page) => {
      if (!controller.signal.aborted) setResult({ state: "ready", page });
    }).catch(() => {
      if (!controller.signal.aborted) setResult({ state: "error" });
    }).finally(() => window.clearTimeout(timeout));
    return () => { controller.abort(); window.clearTimeout(timeout); };
  }, [offset, refresh]);

  useEffect(() => {
    const update = () => { if (!document.hidden) setRefresh((value) => value + 1); };
    const timer = window.setInterval(update, 60000);
    document.addEventListener("visibilitychange", update);
    return () => { window.clearInterval(timer); document.removeEventListener("visibilitychange", update); };
  }, []);

  return <section className="manual-announcements-panel data-section" aria-labelledby={headingId}>
    <header className="manual-announcements-panel__header"><div><h2 id={headingId}>{t("title")}</h2><p className="section-caption">{t("description")}</p></div>
      <button type="button" className="action-button" disabled={result.state === "loading"} onClick={() => setRefresh((value) => value + 1)}>{t("refresh")}</button>
    </header>
    {result.state === "loading" && <p role="status">{t("loading")}</p>}
    {result.state === "error" && <div role="alert"><p>{t("unavailable")}</p><p>{t("unavailableHelp")}</p><button type="button" className="action-button" onClick={() => setRefresh((value) => value + 1)}>{t("retry")}</button></div>}
    {result.state === "ready" && (result.page.items.length === 0 ? <div><p>{t("empty")}</p><p className="section-caption">{t("noSafety")}</p></div> : <>
      <ul className="manual-announcements-panel__list">{result.page.items.map((announcement) => <li key={announcement.id}><AnnouncementCard announcement={announcement} /></li>)}</ul>
      <nav className="manual-announcements-panel__pagination" aria-label={t("pagination")}>
        <button type="button" className="action-button" disabled={offset === 0} onClick={() => setOffset(Math.max(0, offset - result.page.limit))}>{t("previous")}</button>
        <span>{t("page", { from: formatNumber(offset + 1, locale), to: formatNumber(offset + result.page.items.length, locale), total: formatNumber(result.page.total, locale) })}</span>
        <button type="button" className="action-button" disabled={offset + result.page.items.length >= result.page.total || offset + result.page.limit > 10000} onClick={() => setOffset(offset + result.page.limit)}>{t("next")}</button>
      </nav>
    </>)}
  </section>;
}
