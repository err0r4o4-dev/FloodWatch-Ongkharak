import { useEffect, useId, useState } from "react";
import { useTranslation } from "../../i18n";
import en from "../../i18n/locales/en/auth";
import th from "../../i18n/locales/th/auth";
import { apiConfig } from "../../shared/config/api";
import { googleAuthApi, safeReturnTo } from "./google";
import type { GoogleAuthApi } from "./google";

const authMessages = { th, en };

export function GoogleSignInButton({ returnTo, api = googleAuthApi, navigate = (url: string) => window.location.assign(url) }: { returnTo: string; api?: GoogleAuthApi; navigate?: (url: string) => void }) {
  const { t } = useTranslation(authMessages);
  const [state, setState] = useState<"loading" | "enabled" | "disabled" | "error" | "redirecting">("loading");
  const descriptionID = useId();
  useEffect(() => {
    let active = true;
    void api.available().then((enabled) => { if (active) setState(enabled ? "enabled" : "disabled"); }).catch(() => { if (active) setState("error"); });
    return () => { active = false; };
  }, [api]);
  const description = state === "disabled" ? t("googleDisabledDescription") : state === "error" ? t("googleAvailabilityError") : state === "loading" ? t("googleChecking") : state === "redirecting" ? t("googleRedirecting") : t("googleEnabledDescription");
  const start = () => {
    setState("redirecting");
    try { navigate(`${apiConfig.baseUrl}/auth/google/start?returnTo=${encodeURIComponent(safeReturnTo(returnTo))}`); } catch { setState("error"); }
  };
  return <div className="flex flex-col gap-2">
    <button type="button" className="auth-google" disabled={state !== "enabled"} aria-describedby={descriptionID} onClick={start}>{t("googleButton")}</button>
    <p id={descriptionID} role="status" className="text-center text-sm text-[var(--color-ink-muted)]">{description}</p>
  </div>;
}
