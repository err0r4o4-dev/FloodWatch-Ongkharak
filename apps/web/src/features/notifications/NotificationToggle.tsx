import { useTranslation } from "../../i18n";
import th from "../../i18n/locales/th/notifications";
import en from "../../i18n/locales/en/notifications";
const messages = { th, en };
import { useEffect, useRef, useState } from "react";
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
  const { t } = useTranslation(messages);
  const { account, status: authStatus } = useOptionalAuth();
  const [status, setStatus] = useState<SubscriptionStatus>("checking");
  const [subscribed, setSubscribed] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<keyof typeof th | null>(null);
  const [revision, setRevision] = useState(0);
  const supported = "serviceWorker" in navigator && typeof PushManager !== "undefined";
  const permissionDenied = typeof Notification !== "undefined" && Notification.permission === "denied";
  const accountId = account?.id;
  const currentAccount = useRef(accountId);
  currentAccount.current = accountId;
  const mounted = useRef(true);
  const togglePending = useRef(false);

  useEffect(() => {
    mounted.current = true;
    return () => { mounted.current = false; };
  }, []);

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
          setError("couldNotCheckBrowserNotificationsPleaseTry");
        }
      }
    };

    void checkSubscription();
    return () => { active = false; };
  }, [supported, accountId, revision]);

  const handleToggle = async () => {
    if (!account || loading || togglePending.current || status !== "ready" || permissionDenied) return;
    const mutationAccount = account.id;
    const stillCurrent = () => mounted.current && currentAccount.current === mutationAccount;
    togglePending.current = true;
    setLoading(true);
    setError(null);

    try {
      const registration = await navigator.serviceWorker.getRegistration();
      if (!stillCurrent()) return;
      if (!registration?.active) {
        setStatus("unavailable");
        return;
      }
      const existingSubscription = await registration.pushManager.getSubscription();
      if (!stillCurrent()) return;

      if (subscribed) {
        if (existingSubscription) {
          await notificationApi.unsubscribe(existingSubscription.endpoint);
          if (!stillCurrent()) return;
          if (!await existingSubscription.unsubscribe()) throw new Error("Browser unsubscribe failed");
        }
        setSubscribed(false);
        void notifySuccess(t("browserAlertsTurnedOff"), t("thisBrowserWillNoLongerReceiveFlood"));
      } else {
        const { publicKey } = await notificationApi.getPublicKey();
        if (!stillCurrent()) return;
        const subscription = existingSubscription ?? await registration.pushManager.subscribe({
          userVisibleOnly: true,
          applicationServerKey: applicationServerKey(publicKey)
        });
        if (!stillCurrent()) return;
        try {
          await notificationApi.subscribe(subscription);
        } catch (cause) {
          if (!existingSubscription) await subscription.unsubscribe();
          throw cause;
        }
        if (!stillCurrent()) return;
        setSubscribed(true);
        window.dispatchEvent(new Event("floodwatch:push-enrolled"));
        void notifySuccess(t("browserAlertsTurnedOn"), t("thisBrowserCanNowReceiveFloodWatch"));
      }
    } catch {
      if (!stillCurrent()) return;
      const message = t("couldNotSaveYourSettingsCheckYour");
      setError("couldNotSaveYourSettingsCheckYour");
      void notifyError(t("couldNotSaveSettings"), message);
    } finally {
      togglePending.current = false;
      if (mounted.current) setLoading(false);
    }
  };

  if (authStatus === "loading") return <p className="notification-preference__message" role="status">{t("checkingYourAccount")}</p>;

  if (!account) return <div className="notification-preference notification-preference--guest">
    <p>{t("public.guestPush")}</p>
    {!supported && <p className="notification-preference__message">{t("thisBrowserDoesNotSupportBrowserNotifications")}</p>}
  </div>;

  if (!supported) return <p className="notification-preference__message">{t("thisBrowserDoesNotSupportBrowserNotifications")}</p>;

  return <div className="notification-preference" aria-busy={loading || status === "checking"}>
    <label className="notification-preference__control">
      <input type="checkbox" checked={subscribed} onChange={() => void handleToggle()} disabled={loading || status !== "ready" || permissionDenied} aria-describedby="notification-preference-status" />
      <span className="notification-preference__switch" aria-hidden="true" />
      <span>{t("receiveBrowserNotifications")}</span>
    </label>
    <p id="notification-preference-status" className="notification-preference__message" role="status">
      {loading ? t("savingSettings") : permissionDenied ? t("notificationsAreBlockedInYourBrowser") : status === "checking" ? t("checkingBrowserNotifications") : status === "unavailable" ? t("browserNotificationsAreNotReadyYet") : status === "error" ? t("couldNotCheckThisBrowserSNotification") : subscribed ? t("notificationsAreOnInThisBrowser") : t("notificationsAreOffInThisBrowser")}
    </p>
    {permissionDenied && <p className="notification-preference__message">{t("allowNotificationsInYourBrowserSSite")}</p>}
    {status === "unavailable" && <p className="notification-preference__message">{t("browserNotificationsAreNotReadyThisFeature")}</p>}
    {error && <p className="notification-preference__error" role="alert">{t(error)}</p>}
    {(status === "unavailable" || status === "error" || permissionDenied) && <button type="button" className="notification-preference__retry" disabled={loading || status === "checking"} onClick={() => setRevision((previous) => previous + 1)}>{t("checkAgain")}</button>}
  </div>;
}
