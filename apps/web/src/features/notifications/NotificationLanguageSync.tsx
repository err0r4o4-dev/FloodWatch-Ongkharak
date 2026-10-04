import { useEffect, useRef, useState } from "react";
import { useTranslation } from "../../i18n";
import th from "../../i18n/locales/th/notifications";
import en from "../../i18n/locales/en/notifications";
import { useOptionalAuth } from "../auth/AuthProvider";
import { notificationApi } from "./api";
import { ApiError } from "../auth/api";

const messages = { th, en };

export function NotificationLanguageSync() {
  const { locale, t } = useTranslation(messages);
  const { account, status } = useOptionalAuth();
  const [failed, setFailed] = useState(false);
  const [retry, setRetry] = useState(0);
  const pending = useRef<Promise<void>>(Promise.resolve());
  const accountId = status === "authenticated" ? account?.id : undefined;

  useEffect(() => {
    const reconcile = () => setRetry((value) => value + 1);
    window.addEventListener("floodwatch:push-enrolled", reconcile);
    navigator.serviceWorker?.addEventListener?.("controllerchange", reconcile);
    return () => {
      window.removeEventListener("floodwatch:push-enrolled", reconcile);
      navigator.serviceWorker?.removeEventListener?.("controllerchange", reconcile);
    };
  }, []);

  useEffect(() => {
    let active = true;
    setFailed(false);
    if (!accountId || !("serviceWorker" in navigator)) return;
    pending.current = pending.current.catch(() => undefined).then(async () => {
      if (!active) return;
      try {
        const registration = await navigator.serviceWorker.getRegistration();
        const subscription = await registration?.pushManager?.getSubscription();
        if (!active || !subscription) return;
        await notificationApi.updateLanguage(subscription.endpoint, locale);
      } catch (cause) {
        if (cause instanceof ApiError && cause.status === 404) return;
        if (active) setFailed(true);
      }
    });
    return () => { active = false; };
  }, [accountId, locale, retry]);

  if (!failed || !accountId) return null;
  return <aside className="state-panel notification-language-error" role="alert"><p>{t("languageSyncError")}</p><button type="button" className="action-button" onClick={() => setRetry((value) => value + 1)}>{t("tryAgain")}</button></aside>;
}
