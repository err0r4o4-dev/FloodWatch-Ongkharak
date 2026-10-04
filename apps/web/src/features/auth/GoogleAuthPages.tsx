import { useState } from "react";
import { Link, Navigate, useLocation } from "react-router";
import { LanguageSwitcher, useTranslation } from "../../i18n";
import { notifyError, notifySuccess } from "../../shared/alerts";
import { ApiError } from "./api";
import { useAuth } from "./AuthProvider";
import { googleAuthApi, googleErrorKey, safeReturnTo } from "./google";
import type { GoogleAuthApi } from "./google";
import { authMessage, authMessages, formatAuthMessage } from "./messages";
import type { AuthMessage } from "./messages";

const panelClass = "auth-form data-section";

export function GoogleCompletePage() {
  const { t } = useTranslation(authMessages);
  const { status } = useAuth();
  const location = useLocation();
  const returnTo = safeReturnTo(new URLSearchParams(location.search).get("returnTo"));
  if (status === "authenticated") return <Navigate to={returnTo} replace />;
  return <section className="auth-page"><LanguageSwitcher /><div className={panelClass}>
    <h1 className="text-2xl font-bold">{t("googleCompleteHeading")}</h1>
    {status === "loading" ? <p role="status">{t("googleCompleteLoading")}</p> : <>
      <p className="form-error" role="alert">{t("googleCompleteError")}</p>
      <Link className="action-button w-full justify-center" to={`/login?returnTo=${encodeURIComponent(returnTo)}`}>{t("backToSignIn")}</Link>
    </>}
  </div></section>;
}

export function GoogleLinkPage({ api = googleAuthApi, navigate = (url: string) => window.location.assign(url) }: { api?: GoogleAuthApi; navigate?: (url: string) => void }) {
  const { t } = useTranslation(authMessages);
  const [password, setPassword] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<AuthMessage | null>(null);
  const [blocked, setBlocked] = useState(false);
  const submit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!password || submitting || blocked) return;
    setSubmitting(true);
    setError(null);
    try {
      const result = await api.link(password);
      setPassword("");
      await notifySuccess(t("linkedGoogle"));
      navigate(`/auth/complete?returnTo=${encodeURIComponent(safeReturnTo(result.returnTo))}`);
    } catch (failure) {
      const code = failure instanceof ApiError ? failure.code ?? "GOOGLE_LOGIN_FAILED" : "GOOGLE_LOGIN_FAILED";
      const message = googleErrorKey(code) ?? authMessage("linkGoogleFailed");
      setError(message);
      void notifyError(t("linkGoogleFailed"), t(message.key, message.params));
      setBlocked(failure instanceof ApiError && [429, 409, 503].includes(failure.status));
      setSubmitting(false);
    }
  };
  return <section className="auth-page"><LanguageSwitcher /><form className={panelClass} onSubmit={submit}>
    <h1 className="text-2xl font-bold">{t("googleLinkHeading")}</h1>
    <p>{t("googleLinkDescription")}</p>
    {error && <p className="form-error" role="alert">{formatAuthMessage(error, t)}</p>}
    <label className="flex flex-col gap-1 text-sm font-bold text-[var(--color-ink-muted)]">{t("existingPassword")}
      <input type="password" autoComplete="current-password" required value={password} disabled={submitting || blocked} maxLength={128} onChange={(event) => setPassword(event.target.value)} className="w-full min-h-[var(--touch-target)] px-3 py-2 border border-[var(--color-border)] rounded-[var(--radius-sm)] bg-[var(--color-surface)] text-[var(--color-ink)] focus:outline-[3px] focus:outline-[var(--color-primary)] font-normal" />
    </label>
    <button type="submit" className="action-button w-full justify-center" disabled={!password || submitting || blocked}>{submitting ? t("linking") : t("linkGoogle")}</button>
    <Link className="text-[var(--color-primary-strong)] hover:underline" to="/login">{t("backToSignIn")}</Link>
  </form></section>;
}
