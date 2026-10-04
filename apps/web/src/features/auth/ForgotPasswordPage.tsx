import { useEffect, useRef, useState } from "react";
import { Link, useLocation, useNavigate } from "react-router";
import { formatNumber, LanguageSwitcher, useTranslation } from "../../i18n";
import { ApiError, passwordRecoveryApi } from "./api";
import type { PasswordRecoveryApi } from "./api";
import { safeReturnTo } from "./google";
import { authMessage, authMessages, formatAuthMessage } from "./messages";
import type { AuthMessage, AuthMessageKey } from "./messages";

type RecoveryStep = 1 | 2 | 3 | "success";

const errorMessages: Record<string, AuthMessageKey> = {
  INVALID_REQUEST: "invalidRequest",
  INVALID_OTP: "invalidOtp",
  INVALID_RESET_TOKEN: "invalidResetToken",
  RECOVERY_RATE_LIMITED: "recoveryRateLimited",
  RATE_LIMITED: "recoveryRateLimited",
  RECOVERY_UNAVAILABLE: "recoveryUnavailable"
};

function recoveryError(cause: unknown): AuthMessage {
  if (cause instanceof ApiError && cause.code && errorMessages[cause.code]) return authMessage(errorMessages[cause.code]);
  return authMessage("recoveryFailed");
}

function passwordError(password: string): AuthMessage | null {
  const bytes = new TextEncoder().encode(password).byteLength;
  if (bytes < 12) return authMessage("passwordTooShort");
  if (bytes > 128) return authMessage("passwordTooLong");
  return null;
}

function validEmail(email: string): boolean {
  return /^[^\s@]+@[^\s@]+\.[^\s@]+$/u.test(email);
}

function Header({ heading, headingId, children, headingRef }: { heading: string; headingId?: string; children: React.ReactNode; headingRef: React.RefObject<HTMLHeadingElement | null> }) {
  return (
    <header className="auth-heading" style={{ textAlign: "left" }}>
      <h1 id={headingId} ref={headingRef} tabIndex={-1}>{heading}</h1>
      <p>{children}</p>
    </header>
  );
}

function Shell({ children }: { children: React.ReactNode }) {
  const { t } = useTranslation(authMessages);
  return (
    <main className="auth-page">
      <LanguageSwitcher />
      <Link to="/" className="auth-brand" aria-label={t("brandHomeAria")}>
        <img src="/flood-watch-mark.svg" alt="" />
        <div>
          <strong>Flood Watch</strong>
          <small>{t("brandTagline")}</small>
        </div>
      </Link>
      {children}
    </main>
  );
}

export function ForgotPasswordPage({ api = passwordRecoveryApi }: { api?: PasswordRecoveryApi }) {
  const { locale, t } = useTranslation(authMessages);
  const location = useLocation();
  const navigate = useNavigate();
  const returnTo = safeReturnTo(new URLSearchParams(location.search).get("returnTo"));
  const loginPath = `/login?returnTo=${encodeURIComponent(returnTo)}`;
  const [step, setStep] = useState<RecoveryStep>(1);
  const [email, setEmail] = useState("");
  const [otp, setOtp] = useState("");
  const [challengeId, setChallengeId] = useState<string | null>(null);
  const [resetToken, setResetToken] = useState<string | null>(null);
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [requestMessage, setRequestMessage] = useState("");
  const [resendRemaining, setResendRemaining] = useState(0);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<AuthMessage | null>(null);
  const headingRef = useRef<HTMLHeadingElement>(null);

  useEffect(() => {
    headingRef.current?.focus();
  }, [step]);

  useEffect(() => {
    if (resendRemaining <= 0) return undefined;
    const timer = window.setInterval(() => {
      setResendRemaining((remaining) => Math.max(0, remaining - 1));
    }, 1000);
    return () => window.clearInterval(timer);
  }, [resendRemaining]);

  const clearGrants = () => {
    setChallengeId(null);
    setResetToken(null);
    setOtp("");
    setNewPassword("");
    setConfirmPassword("");
    setResendRemaining(0);
    setRequestMessage("");
  };

  const handleEmailChange = (value: string) => {
    setEmail(value);
    clearGrants();
    if (step !== 1) setStep(1);
    setError(null);
  };

  const requestOtp = async () => {
    const normalizedEmail = email.trim();
    if (!normalizedEmail) return setError(authMessage("emailRequired"));
    if (!validEmail(normalizedEmail)) return setError(authMessage("invalidEmail"));
    if (loading || (step === 2 && resendRemaining > 0)) return;
    setLoading(true);
    setError(null);
    try {
      const response = await api.requestOtp(normalizedEmail);
      setEmail(normalizedEmail);
      setChallengeId(response.challengeId);
      setResetToken(null);
      setOtp("");
      setRequestMessage(response.message);
      setResendRemaining(Math.max(0, response.resendAfter));
      setStep(2);
    } catch (cause) {
      setError(recoveryError(cause));
    } finally {
      setLoading(false);
    }
  };

  const handleRequestOtp = (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    void requestOtp();
  };

  const handleVerifyOtp = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (loading) return;
    if (!/^\d{6}$/u.test(otp)) return setError(authMessage("otpInvalid"));
    if (!challengeId) return setError(authMessage("recoveryExpired"));
    setLoading(true);
    setError(null);
    try {
      const response = await api.verifyOtp({ challengeId, otp });
      setResetToken(response.resetToken);
      setStep(3);
    } catch (cause) {
      setError(recoveryError(cause));
    } finally {
      setLoading(false);
    }
  };

  const handleResetPassword = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (loading) return;
    if (!resetToken) {
      setError(authMessage("resetExpired"));
      return;
    }
    const newPasswordError = passwordError(newPassword);
    if (newPasswordError) return setError(newPasswordError);
    if (newPassword !== confirmPassword) return setError(authMessage("passwordMismatch"));
    setLoading(true);
    setError(null);
    try {
      await api.resetPassword({ resetToken, password: newPassword });
      setResetToken(null);
      setChallengeId(null);
      setNewPassword("");
      setConfirmPassword("");
      setStep("success");
    } catch (cause) {
      if (cause instanceof ApiError && cause.code === "INVALID_RESET_TOKEN") {
        clearGrants();
        setStep(1);
      }
      setError(recoveryError(cause));
    } finally {
      setLoading(false);
    }
  };

  const editEmail = () => {
    clearGrants();
    setError(null);
    setStep(1);
  };

  if (step === "success") {
    return (
      <Shell>
        <section className="auth-form data-section" aria-labelledby="recovery-success-heading">
          <Header heading={t("resetSuccessHeading")} headingId="recovery-success-heading" headingRef={headingRef}>{t("resetSuccessDescription")}</Header>
          <button type="button" className="action-button" disabled={loading} onClick={() => navigate(loginPath, { replace: true })} style={{ minHeight: "3rem", justifyContent: "center" }}>
            {t("backToLogin")}
          </button>
        </section>
      </Shell>
    );
  }

  if (step === 1) {
    return (
      <Shell>
        <form className="auth-form data-section" onSubmit={handleRequestOtp} noValidate>
          <Header heading={t("forgotHeading")} headingRef={headingRef}>{t("forgotDescription")}</Header>
          {error && <p className="form-error" role="alert">{formatAuthMessage(error, t)}</p>}
          <label>
            {t("email")}
            <input type="email" autoComplete="email" value={email} onChange={(event) => handleEmailChange(event.target.value)} disabled={loading} placeholder={t("emailPlaceholder")} />
          </label>
          <div className="auth-actions" style={{ marginTop: "0.5rem" }}>
            <button type="submit" className="action-button" disabled={loading} style={{ width: "100%", justifyContent: "center" }}>
              {loading ? t("checking") : t("requestOtp")}
            </button>
          </div>
          <p className="section-caption">
            {t("rememberPassword")} <Link to={loginPath}>{t("backToLogin")}</Link>
          </p>
        </form>
      </Shell>
    );
  }

  if (step === 2) {
    return (
      <Shell>
        <form className="auth-form data-section" onSubmit={handleVerifyOtp} noValidate>
          <Header heading={t("otpHeading")} headingRef={headingRef}>{t("otpDescription", { name: email })}</Header>
          {requestMessage && <p role="status" aria-label={t("recoveryInfo")}>{requestMessage}</p>}
          {error && <p className="form-error" role="alert">{formatAuthMessage(error, t)}</p>}
          <label>
            {t("otp")}
            <input type="text" inputMode="numeric" pattern="[0-9]{6}" maxLength={6} autoComplete="one-time-code" value={otp} onChange={(event) => setOtp(event.target.value)} disabled={loading} placeholder={t("otpPlaceholder")} />
          </label>
          <div className="auth-actions" style={{ marginTop: "0.5rem" }}>
            <button type="submit" className="action-button" disabled={loading} style={{ width: "100%", justifyContent: "center" }}>
              {loading ? t("checking") : t("verifyOtp")}
            </button>
          </div>
          <p role="timer" aria-live="off" aria-label={t("resendTimer")}>
            {resendRemaining > 0 ? t("resendIn", { seconds: formatNumber(resendRemaining, locale) }) : t("resendAvailable")}
          </p>
          <div className="auth-actions" style={{ marginTop: "0.5rem" }}>
            <button type="button" className="action-button" disabled={loading || resendRemaining > 0} onClick={() => void requestOtp()} style={{ width: "100%", justifyContent: "center", backgroundColor: "transparent", color: "var(--color-ink-muted)", border: "1px solid var(--color-border)" }}>
              {t("resendOtp")}
            </button>
          </div>
          <button type="button" className="auth-text-action" disabled={loading} onClick={editEmail}>{t("editEmail")}</button>
        </form>
      </Shell>
    );
  }

  return (
    <Shell>
      <form className="auth-form data-section" onSubmit={handleResetPassword} noValidate>
        <Header heading={t("newPasswordHeading")} headingRef={headingRef}>{t("newPasswordDescription")}</Header>
        {error && <p className="form-error" role="alert">{formatAuthMessage(error, t)}</p>}
        <label>
          {t("newPassword")}
          <input type="password" autoComplete="new-password" value={newPassword} onChange={(event) => setNewPassword(event.target.value)} disabled={loading} placeholder={t("newPasswordPlaceholder")} />
        </label>
        <label>
          {t("confirmPassword")}
          <input type="password" autoComplete="new-password" value={confirmPassword} onChange={(event) => setConfirmPassword(event.target.value)} disabled={loading} placeholder={t("confirmPasswordPlaceholder")} />
        </label>
        <p className="section-caption" style={{ textAlign: "left" }}>{t("authPasswordHint")}</p>
        <details>
          <summary>{t("passwordTechnicalDetailsSummary")}</summary>
          <p className="section-caption" style={{ textAlign: "left" }}>{t("passwordTechnicalDetails")}</p>
        </details>
        <div className="auth-actions" style={{ marginTop: "0.5rem" }}>
          <button type="submit" className="action-button" disabled={loading} style={{ width: "100%", justifyContent: "center" }}>
            {loading ? t("savingPassword") : t("changePassword")}
          </button>
        </div>
        <button type="button" className="auth-text-action" disabled={loading} onClick={editEmail}>{t("startRecoveryAgain")}</button>
      </form>
    </Shell>
  );
}
