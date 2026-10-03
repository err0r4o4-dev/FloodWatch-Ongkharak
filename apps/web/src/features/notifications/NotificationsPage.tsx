import { useEffect, useRef, useState } from "react";
import type { ReactNode } from "react";
import { Link } from "react-router";
import { useOptionalAuth } from "../auth/AuthProvider";
import { NotificationToggle } from "./NotificationToggle";
import { NOTIFICATION_PAGE_SIZE, notificationApi } from "./api";
import type { NotificationItem, NotificationPage } from "./api";
import "./NotificationsPage.css";

type IconName = "bell" | "settings" | "clock" | "info" | "arrow" | "megaphone" | "check";

function NotificationIcon({ name }: { name: IconName }) {
  const paths: Record<IconName, ReactNode> = {
    bell: <><path d="M18 8a6 6 0 0 0-12 0c0 7-3 7-3 9h18c0-2-3-2-3-9" /><path d="M10 21h4" /></>,
    settings: <><path d="M3 7h4m4 0h10M3 17h10m4 0h4" /><circle cx="9" cy="7" r="2" /><circle cx="15" cy="17" r="2" /></>,
    clock: <><circle cx="12" cy="12" r="9" /><path d="M12 7v5l3 2" /></>,
    info: <><circle cx="12" cy="12" r="9" /><path d="M12 11v5M12 8h.01" /></>,
    arrow: <><path d="M4 12h16m-6-6 6 6-6 6" /></>,
    megaphone: <><path d="m4 10 14-5v14L4 14v-4ZM18 9l3-1v8l-3-1M7 15l2 5h3l-2-4" /></>,
    check: <><rect x="5" y="3" width="14" height="18" rx="2" /><path d="m8 12 3 3 5-6" /></>
  };
  return <svg viewBox="0 0 24 24" width="24" height="24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">{paths[name]}</svg>;
}

const severityLabels: Record<NotificationItem["severity"], string> = {
  INFO: "ข้อมูล",
  WATCH: "เฝ้าระวัง",
  WARNING: "เตือนภัย",
  CRITICAL: "วิกฤต"
};

const sourceLabels: Record<NotificationItem["sourceType"], string> = {
  OFFICIAL: "ทางการ",
  COMMUNITY: "ชุมชน",
  AI: "AI"
};

function formatNotificationDate(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return new Intl.DateTimeFormat("th-TH", { dateStyle: "medium", timeStyle: "short" }).format(date);
}

function notificationPageIsAtEnd(page: NotificationPage) {
  return page.pagination.offset + page.pagination.limit >= page.pagination.total;
}

export function NotificationsPage() {
  const { account, status: authStatus } = useOptionalAuth();
  const headingRef = useRef<HTMLHeadingElement>(null);
  const [page, setPage] = useState<NotificationPage | null>(null);
  const [unreadCount, setUnreadCount] = useState<number | null>(null);
  const [offset, setOffset] = useState(0);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(false);
  const [retry, setRetry] = useState(0);
  const [markingId, setMarkingId] = useState<string | null>(null);
  const [markingAll, setMarkingAll] = useState(false);
  const [actionError, setActionError] = useState(false);

  useEffect(() => { headingRef.current?.focus(); }, []);

  useEffect(() => {
    if (authStatus !== "authenticated" || !account) return;
    const controller = new AbortController();
    let active = true;
    setLoading(true);
    setError(false);
    setActionError(false);

    Promise.all([
      notificationApi.list({ limit: NOTIFICATION_PAGE_SIZE, offset, signal: controller.signal }),
      notificationApi.getUnreadCount()
    ]).then(([nextPage, nextUnreadCount]) => {
      if (!active) return;
      setPage(nextPage);
      setUnreadCount(nextUnreadCount.count);
    }).catch(() => {
      if (active) setError(true);
    }).finally(() => {
      if (active) setLoading(false);
    });

    return () => {
      active = false;
      controller.abort();
    };
  }, [account, authStatus, offset, retry]);

  const handleMarkRead = async (item: NotificationItem) => {
    if (item.readAt || markingId || markingAll) return;
    setMarkingId(item.id);
    setActionError(false);
    try {
      await notificationApi.markRead(item.id);
      setPage((current) => current ? { ...current, items: current.items.map((candidate) => candidate.id === item.id ? { ...candidate, readAt: new Date().toISOString() } : candidate) } : current);
      setUnreadCount((current) => current === null ? current : Math.max(0, current - 1));
    } catch {
      setActionError(true);
    } finally {
      setMarkingId(null);
    }
  };

  const handleMarkAllRead = async () => {
    if (unreadCount === null || markingAll || markingId) return;
    setMarkingAll(true);
    setActionError(false);
    try {
      await notificationApi.markAllRead();
      setPage((current) => current ? { ...current, items: current.items.map((item) => ({ ...item, readAt: item.readAt ?? new Date().toISOString() })) } : current);
      setUnreadCount(0);
    } catch {
      setActionError(true);
    } finally {
      setMarkingAll(false);
    }
  };

  const renderInbox = () => {
    if (authStatus === "loading") return <div className="notifications-history notifications-history--state"><span className="notifications-history__icon"><NotificationIcon name="clock" /></span><p role="status">กำลังตรวจสอบบัญชี…</p></div>;
    if (!account || authStatus === "anonymous") return <div className="notifications-history notifications-history--state"><span className="notifications-history__icon"><NotificationIcon name="bell" /></span><h2>ลงชื่อเข้าใช้เพื่อดูการแจ้งเตือน</h2><p>ประวัติการแจ้งเตือนและสถานะอ่านแล้วมีให้สำหรับบัญชีของคุณ</p><Link to="/login" className="notifications-inline-link">เข้าสู่ระบบ<NotificationIcon name="arrow" /></Link></div>;
    if (loading && !page) return <div className="notifications-history notifications-history--state"><span className="notifications-history__icon"><NotificationIcon name="clock" /></span><p role="status">กำลังโหลดการแจ้งเตือน…</p></div>;
    if (error) return <div className="notifications-history notifications-history--state"><span className="notifications-history__icon"><NotificationIcon name="info" /></span><h2>ไม่สามารถโหลดการแจ้งเตือนได้</h2><p role="alert">โหลดการแจ้งเตือนไม่สำเร็จ กรุณาลองอีกครั้ง</p><button type="button" className="notifications-inline-button" onClick={() => setRetry((current) => current + 1)}>ลองอีกครั้ง</button></div>;
    if (!page || page.items.length === 0) return <div className="notifications-history notifications-history--state"><span className="notifications-history__icon"><NotificationIcon name="bell" /></span><h2>ยังไม่มีการแจ้งเตือน</h2><p>เมื่อมีการแจ้งเตือนใหม่ รายการจะแสดงที่นี่</p></div>;

    return <>
      <ul className="notifications-list" aria-label="รายการแจ้งเตือน" aria-busy={loading}>
        {page.items.map((item) => <li key={item.id} className={`notification-item${item.readAt ? " notification-item--read" : " notification-item--unread"}`}>
          <article aria-labelledby={`notification-title-${item.id}`}>
            <div className="notification-item__meta">
              <span className={`notification-badge notification-badge--${item.severity.toLowerCase()}`}>{severityLabels[item.severity]}</span>
              <span className="notification-item__source">แหล่งที่มา: {sourceLabels[item.sourceType]}</span>
              <time dateTime={item.createdAt}>{formatNotificationDate(item.createdAt)}</time>
            </div>
            <h3 id={`notification-title-${item.id}`}>{item.title}</h3>
            <p>{item.message}</p>
            <div className="notification-item__actions">
              <span className="notification-read-state">{item.readAt ? "อ่านแล้ว" : "ยังไม่ได้อ่าน"}</span>
              {!item.readAt && <button type="button" className="notifications-item-button" disabled={markingId === item.id || markingAll} aria-busy={markingId === item.id} onClick={() => void handleMarkRead(item)}>{markingId === item.id ? "กำลังบันทึก…" : `ทำเครื่องหมายว่าอ่านแล้ว: ${item.title}`}</button>}
            </div>
          </article>
        </li>)}
      </ul>
      <nav className="notifications-pagination" aria-label="การแบ่งหน้าการแจ้งเตือน">
        <span>แสดง {page.pagination.offset + 1}–{Math.min(page.pagination.offset + page.items.length, page.pagination.total)} จาก {page.pagination.total}</span>
        <div className="notifications-pagination__actions">
          <button type="button" className="notifications-page-button" aria-label="หน้าก่อนหน้า" disabled={page.pagination.offset === 0 || loading} onClick={() => setOffset(Math.max(0, page.pagination.offset - page.pagination.limit))}>หน้าก่อนหน้า</button>
          <button type="button" className="notifications-page-button" aria-label="หน้าถัดไป" disabled={notificationPageIsAtEnd(page) || loading} onClick={() => setOffset(page.pagination.offset + page.pagination.limit)}>หน้าถัดไป</button>
        </div>
      </nav>
    </>;
  };

  return <div className="notifications-page">
    <section className="notifications-hero" aria-labelledby="notifications-title">
      <div className="notifications-hero__copy">
        <p className="eyebrow">อัปเดตสำหรับคุณ</p>
        <h1 id="notifications-title" ref={headingRef} tabIndex={-1}>การแจ้งเตือน</h1>
        <p>ติดตามการแจ้งเตือนและจัดการการรับข่าวสารจาก Flood Watch ในที่เดียว</p>
      </div>
      <div className="notifications-hero__art" aria-hidden="true">
        <span className="notifications-hero__tile notifications-hero__tile--small"><NotificationIcon name="megaphone" /></span>
        <span className="notifications-hero__tile notifications-hero__tile--bell"><NotificationIcon name="bell" /></span>
        <span className="notifications-hero__tile notifications-hero__tile--small"><NotificationIcon name="check" /></span>
      </div>
      <div className="notifications-hero__actions">
        <button type="button" className="notifications-read-all" disabled={!account || authStatus !== "authenticated" || unreadCount === null || unreadCount === 0 || loading || markingAll || markingId !== null} aria-busy={markingAll} onClick={handleMarkAllRead}><NotificationIcon name="check" />อ่านทั้งหมดแล้ว{unreadCount === null ? "" : ` (${unreadCount})`}</button>
        <span aria-live="polite">{account && authStatus === "authenticated" ? unreadCount === null ? "จำนวนที่ยังไม่ได้อ่านยังไม่พร้อมใช้งาน" : `ยังไม่ได้อ่าน ${unreadCount} รายการ` : "ลงชื่อเข้าใช้เพื่อดูสถานะการอ่าน"}</span>
      </div>
    </section>

    <div className="notifications-page__layout">
      <section className="notifications-history" aria-labelledby="notifications-history-title">
        <div className="notifications-history__header">
          <div>
            <p className="notifications-history__eyebrow">กล่องข้อความ</p>
            <h2 id="notifications-history-title">การแจ้งเตือนของคุณ</h2>
          </div>
          {actionError && <p className="notifications-history__error" role="alert">บันทึกสถานะอ่านแล้วไม่สำเร็จ กรุณาลองอีกครั้ง</p>}
        </div>
        {renderInbox()}
      </section>

      <aside className="notifications-page__sidebar" aria-label="การตั้งค่าและข้อควรทราบเกี่ยวกับการแจ้งเตือน">
        <section className="notifications-card" aria-labelledby="notifications-preferences-title">
          <span className="notifications-card__icon"><NotificationIcon name="settings" /></span>
          <h2 id="notifications-preferences-title">ตั้งค่าการแจ้งเตือน</h2>
          <p>เลือกรับการแจ้งเตือนผ่านเบราว์เซอร์ที่คุณใช้งาน โดยเปิดหรือปิดได้ด้วยตัวเอง</p>
          <NotificationToggle />
          <div className="notifications-note"><NotificationIcon name="clock" /><p>การรับ Push และประวัติการแจ้งเตือนเป็นคนละส่วน การเปิดรับ Push ไม่ได้เปิดใช้ประวัติหรือสถานะอ่านแล้ว</p></div>
        </section>

        <section className="notifications-card" aria-labelledby="notifications-caution-title">
          <span className="notifications-card__icon"><NotificationIcon name="info" /></span>
          <h2 id="notifications-caution-title">ติดตามสถานการณ์อย่างรอบด้าน</h2>
          <p>ไม่มีการแจ้งเตือนไม่ได้หมายความว่าพื้นที่ปลอดภัย</p>
          <p>ตรวจสอบแหล่งที่มา เวลา และข้อมูลล่าสุดจากหน่วยงานทางการก่อนตัดสินใจเดินทาง</p>
          <Link to="/map" className="notifications-inline-link">ดูแผนที่สถานการณ์น้ำ<NotificationIcon name="arrow" /></Link>
        </section>
      </aside>
    </div>
  </div>;
}
