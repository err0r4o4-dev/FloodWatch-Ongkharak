import { useEffect, useState } from "react";
import { Link } from "react-router";
import { useOptionalAuth } from "../auth/AuthProvider";
import { notificationApi } from "./api";
import { notifyError, notifySuccess } from "../../shared/alerts";
import "./NotificationToggle.css";

type SubscriptionStatus = "checking" | "ready" | "unavailable" | "error";

function applicationServerKey(publicKey: string) {
  const padding = "=".repeat((4 - publicKey.length % 4) % 4);
  const encoded = (publicKey + padding).replace(/-/g, "+").replace(/_/g, "/");
  return Uint8Array.from(window.atob(encoded), (character) => character.charCodeAt(0));
}

export function NotificationToggle() {
  const { account, status: authStatus } = useOptionalAuth();
  const [status, setStatus] = useState<SubscriptionStatus>("checking");
  const [subscribed, setSubscribed] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [revision, setRevision] = useState(0);
  const supported = "serviceWorker" in navigator && typeof PushManager !== "undefined";
  const permissionDenied = typeof Notification !== "undefined" && Notification.permission === "denied";
  const accountId = account?.id;

  useEffect(() => {
    if (!supported || !accountId) return;
    let active = true;
    setStatus("checking");
    setError(null);
    setSubscribed(false);

    const checkSubscription = async () => {
      try {
        const registration = await navigator.serviceWorker.getRegistration();
        if (!registration?.active) {
          if (active) setStatus("unavailable");
          return;
        }
        const subscription = await registration.pushManager.getSubscription();
        if (active) {
          setSubscribed(Boolean(subscription));
          setStatus("ready");
        }
      } catch {
        if (active) {
          setStatus("error");
          setError("ตรวจสอบการแจ้งเตือนไม่สำเร็จ กรุณาลองอีกครั้ง");
        }
      }
    };

    void checkSubscription();
    return () => { active = false; };
  }, [supported, accountId, revision]);

  const handleToggle = async () => {
    if (!account || loading || status !== "ready" || permissionDenied) return;
    setLoading(true);
    setError(null);

    try {
      const registration = await navigator.serviceWorker.getRegistration();
      if (!registration?.active) {
        setStatus("unavailable");
        return;
      }
      const existingSubscription = await registration.pushManager.getSubscription();

      if (subscribed) {
        if (existingSubscription) {
          await notificationApi.unsubscribe(existingSubscription.endpoint);
          if (!await existingSubscription.unsubscribe()) throw new Error("Browser unsubscribe failed");
        }
        setSubscribed(false);
        void notifySuccess("ปิดการแจ้งเตือนแล้ว", "เบราว์เซอร์นี้จะไม่รับ Push จาก Flood Watch");
      } else {
        const { publicKey } = await notificationApi.getPublicKey();
        const subscription = existingSubscription ?? await registration.pushManager.subscribe({
          userVisibleOnly: true,
          applicationServerKey: applicationServerKey(publicKey)
        });
        try {
          await notificationApi.subscribe(subscription);
        } catch (cause) {
          if (!existingSubscription) await subscription.unsubscribe();
          throw cause;
        }
        setSubscribed(true);
        void notifySuccess("เปิดการแจ้งเตือนแล้ว", "เบราว์เซอร์นี้พร้อมรับ Push จาก Flood Watch");
      }
    } catch {
      const message = "เปลี่ยนการตั้งค่าไม่สำเร็จ กรุณาตรวจสอบการเชื่อมต่อและสิทธิ์ของเบราว์เซอร์ แล้วลองอีกครั้ง";
      setError(message);
      void notifyError("เปลี่ยนการตั้งค่าไม่สำเร็จ", message);
    } finally {
      setLoading(false);
    }
  };

  if (authStatus === "loading") return <p className="notification-preference__message" role="status">กำลังตรวจสอบบัญชี…</p>;

  if (!account) return <div className="notification-preference notification-preference--guest">
    <p>เข้าสู่ระบบเพื่อจัดการการรับแจ้งเตือนบนอุปกรณ์นี้</p>
    <Link className="notification-preference__login" to="/login">เข้าสู่ระบบเพื่อเปิดแจ้งเตือน<span aria-hidden="true">→</span></Link>
    {!supported && <p className="notification-preference__message">เบราว์เซอร์นี้ไม่รองรับการแจ้งเตือนแบบ Push</p>}
  </div>;

  if (!supported) return <p className="notification-preference__message">เบราว์เซอร์นี้ไม่รองรับการแจ้งเตือนแบบ Push</p>;

  return <div className="notification-preference" aria-busy={loading || status === "checking"}>
    <label className="notification-preference__control">
      <input type="checkbox" checked={subscribed} onChange={() => void handleToggle()} disabled={loading || status !== "ready" || permissionDenied} aria-describedby="notification-preference-status" />
      <span className="notification-preference__switch" aria-hidden="true" />
      <span>รับการแจ้งเตือนผ่านเบราว์เซอร์</span>
    </label>
    <p id="notification-preference-status" className="notification-preference__message" role="status">
      {loading ? "กำลังบันทึกการตั้งค่า…" : permissionDenied ? "เบราว์เซอร์ปิดกั้นการแจ้งเตือนอยู่" : status === "checking" ? "กำลังตรวจสอบการแจ้งเตือน…" : status === "unavailable" ? "ระบบแจ้งเตือนบนเบราว์เซอร์ยังไม่พร้อมใช้งาน" : status === "error" ? "ยังไม่สามารถยืนยันการตั้งค่าบนเบราว์เซอร์นี้ได้" : subscribed ? "เปิดการแจ้งเตือนในเบราว์เซอร์นี้แล้ว" : "ยังไม่ได้เปิดการแจ้งเตือนในเบราว์เซอร์นี้"}
    </p>
    {permissionDenied && <p className="notification-preference__message">อนุญาตการแจ้งเตือนในการตั้งค่าเว็บไซต์ของเบราว์เซอร์ แล้วตรวจสอบอีกครั้ง</p>}
    {status === "unavailable" && <p className="notification-preference__message">ต้องมี Service Worker ที่พร้อมทำงานก่อนเปิดรับ Push โดยโหมดพัฒนายังไม่เปิดใช้งานส่วนนี้</p>}
    {error && <p className="notification-preference__error" role="alert">{error}</p>}
    {(status === "unavailable" || status === "error" || permissionDenied) && <button type="button" className="notification-preference__retry" disabled={loading || status === "checking"} onClick={() => setRevision((previous) => previous + 1)}>ตรวจสอบอีกครั้ง</button>}
  </div>;
}
