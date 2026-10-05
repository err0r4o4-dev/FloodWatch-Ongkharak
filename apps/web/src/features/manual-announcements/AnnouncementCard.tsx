import { useEffect, useState } from "react";
import { useTranslation } from "../../i18n";
import th from "../../i18n/locales/th/manualAnnouncements";
import en from "../../i18n/locales/en/manualAnnouncements";
import { DataTimestamp, StatusBadge } from "../../shared/components/DataStates";
import type { ManualAnnouncement } from "./api";
import { isPublicReferenceUrl } from "./validation";
import "./manual-announcements.css";

const messages = { th, en };
export function AnnouncementCard({ announcement, preview = false }: { announcement: ManualAnnouncement; preview?: boolean }) {
  const { t } = useTranslation(messages);
  const [now, setNow] = useState(Date.now);
  const expiry = announcement.expiresAt ? Date.parse(announcement.expiresAt) : NaN;
  const observed = announcement.observedAt ? Date.parse(announcement.observedAt) : NaN;
  useEffect(() => {
    setNow(Date.now());
    if (!Number.isFinite(expiry) || expiry <= Date.now()) return;
    const timer = window.setTimeout(() => setNow(Date.now()), Math.min(expiry - Date.now() + 10, 2147483647));
    const onVisibility = () => { if (!document.hidden) setNow(Date.now()); };
    document.addEventListener("visibilitychange", onVisibility);
    return () => { window.clearTimeout(timer); document.removeEventListener("visibilitychange", onVisibility); };
  }, [expiry, now >= expiry]);
  const expired = announcement.status === "EXPIRED" || (announcement.status === "PUBLISHED" && Number.isFinite(expiry) && expiry <= now);
  const withdrawn = announcement.status === "WITHDRAWN";
  const current = !expired && !withdrawn && announcement.status === "PUBLISHED" && announcement.effectiveStatus !== "UNKNOWN" && Number.isFinite(observed) && observed <= now && Number.isFinite(expiry) && expiry > now;
  const effective = preview ? announcement.severity : current ? announcement.effectiveStatus : "UNKNOWN";
  const displayStatus = expired ? "EXPIRED" : announcement.status;
  return <article className={`manual-announcement manual-announcement--${effective.toLowerCase()}`}>
    {preview && <p className="manual-announcement__notice">{t("preview")}</p>}
    <div className="manual-announcement__badges"><StatusBadge status={effective} /><span className="badge">{t(displayStatus)}</span></div>
    <p className="manual-announcement__publisher">{t("publisher")}</p>
    <h3>{announcement.title || t("untitled")}</h3>
    {!preview && !current && <div className="manual-announcement__notice"><p>{t(withdrawn ? "withdrawn" : expired ? "expired" : "unknown")}</p><p>{t("previousSeverity", { severity: t(announcement.severity) })}</p></div>}
    {withdrawn && <div><strong>{t("reason")}</strong><p className="manual-announcement__text">{announcement.withdrawalReason}</p><DataTimestamp label={t("withdrawnAt")} value={announcement.withdrawnAt} /></div>}
    {(current || preview) && <p className="manual-announcement__text">{announcement.summary}</p>}
    <div className="manual-announcement__times">
      <DataTimestamp label={t("observedAt")} value={announcement.observedAt} />
      <DataTimestamp label={t("expiresAt")} value={announcement.expiresAt} />
      {!preview && <><DataTimestamp label={t("publishedAt")} value={announcement.publishedAt} /><DataTimestamp label={t("updatedAt")} value={announcement.updatedAt} /></>}
    </div>
    <details className="manual-announcement__details" open={preview || undefined}>
      <summary>{t(current || preview ? "details" : "archiveDetails")}</summary>
      <p className="section-caption">{t("original")}</p>
      {!current && !preview && <p className="manual-announcement__text">{announcement.summary}</p>}
      <h4>{t("references")}</h4>
      <ul className="manual-announcement__references">{announcement.references.map((reference, index) => <li key={index}>
        {isPublicReferenceUrl(reference.url) ? <a href={reference.url} target="_blank" rel="noopener noreferrer">{reference.name || reference.url}</a> : <span>{reference.name || "—"}</span>}
        <DataTimestamp label={t("referenceTime")} value={reference.observedAt} />
      </li>)}</ul>
      {!preview && <DataTimestamp label={t("recordedAt")} value={announcement.createdAt} />}
    </details>
    <p className="manual-announcement__disclaimer">{t("disclaimer")}</p>
  </article>;
}
