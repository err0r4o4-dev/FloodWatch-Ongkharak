import { useEffect, useRef } from "react";
import type { ReactNode } from "react";
import { Link } from "react-router";
import { NotificationToggle } from "./NotificationToggle";
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

export function NotificationsPage() {
  const headingRef = useRef<HTMLHeadingElement>(null);

  useEffect(() => { headingRef.current?.focus(); }, []);

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
        <button type="button" className="notifications-read-all" disabled aria-describedby="notifications-read-unavailable"><NotificationIcon name="check" />อ่านทั้งหมดแล้ว</button>
        <span id="notifications-read-unavailable" className="sr-only">ยังไม่สามารถบันทึกสถานะอ่านแล้วได้</span>
        <span>รอระบบประวัติการแจ้งเตือน</span>
      </div>
    </section>

    <div className="notifications-page__layout">
      <section className="notifications-history" aria-labelledby="notifications-history-title">
        <span className="notifications-history__icon"><NotificationIcon name="bell" /></span>
        <h2 id="notifications-history-title">ประวัติการแจ้งเตือนยังไม่พร้อมใช้งาน</h2>
        <p>ระบบยังไม่เปิดให้เรียกดูประวัติและบันทึกสถานะอ่านแล้ว<br />คุณยังสามารถติดตามประกาศที่เผยแพร่ได้ตามปกติ</p>
        <Link to="/announcements" className="notifications-inline-link">ดูประกาศล่าสุด<NotificationIcon name="arrow" /></Link>
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
