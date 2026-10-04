import { useState } from "react";
import { Link, useLocation, useNavigate } from "react-router";
import { LanguageSwitcher, useTranslation } from "../../i18n";
import { notifyError, notifySuccess } from "../../shared/alerts";
import { safeAdminReturnTo } from "../admin/RequireAdmin";
import { ApiError } from "./api";
import { useAuth } from "./AuthProvider";
import { GoogleSignInButton } from "./GoogleSignInButton";
import { googleErrorKey, safeReturnTo } from "./google";
import { authMessage, authMessages, formatAuthMessage, translateAuthMessage } from "./messages";
import type { AuthMessage, AuthMessageKey } from "./messages";

function destination(search: string, adminMode: boolean): string {
  const value = new URLSearchParams(search).get("returnTo");
  return adminMode ? safeAdminReturnTo(value) : safeReturnTo(value);
}

function passwordLengthMessage(password: string): AuthMessage | null {
  const bytes = new TextEncoder().encode(password).byteLength;
  if (bytes < 12) return authMessage("passwordTooShort");
  if (bytes > 128) return authMessage("passwordTooLong");
  return null;
}

function usernameError(username: string): AuthMessage | null {
  const normalized = username.trim().toLowerCase();
  if (normalized.length < 3 || normalized.length > 32) return authMessage("usernameInvalid");
  if (!/^[a-z0-9_]+$/u.test(normalized)) return authMessage("usernameInvalid");
  return null;
}

function authErrorMessage(cause: unknown, fallback: AuthMessageKey): AuthMessage {
  if (!(cause instanceof ApiError)) return authMessage(fallback);
  switch (cause.code) {
    case "USERNAME_EXISTS": return authMessage("usernameExists");
    case "EMAIL_EXISTS": return authMessage("registrationFailed");
    case "INVALID_CREDENTIALS": return authMessage("invalidCredentials");
    default: return authMessage(fallback);
  }
}

export function AuthPage({ mode, adminMode = false }: { mode: "login" | "register"; adminMode?: boolean }) {
  const { t } = useTranslation(authMessages);
  const { account, login, logout, register, status } = useAuth();
  const location = useLocation();
  const navigate = useNavigate();
  const [email, setEmail] = useState("");
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<AuthMessage | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [showPassword, setShowPassword] = useState(false);
  const [registrationSucceeded, setRegistrationSucceeded] = useState(false);
  const [switchingAccount, setSwitchingAccount] = useState(false);
  const registering = mode === "register";
  const returnTo = destination(location.search, adminMode);
  const googleError = googleErrorKey(new URLSearchParams(location.search).get("googleError"));

  const submit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (submitting) return;
    const normalizedEmail = email.trim();
    const normalizedUsername = username.trim().toLowerCase();
    if (!normalizedEmail || !password) return setError(authMessage(registering ? "requiredRegister" : "requiredLogin"));
    const passwordError = passwordLengthMessage(password);
    if (passwordError) return setError(passwordError);
    if (registering) {
      const usernameValidationError = usernameError(username);
      if (usernameValidationError) return setError(usernameValidationError);
    }
    setSubmitting(true);
    setError(null);
    setRegistrationSucceeded(false);
    try {
      if (registering) {
        try {
          await register({ email: normalizedEmail, username: normalizedUsername, password });
        } catch (cause) {
          const message = authErrorMessage(cause, "registrationFailed");
          setError(message);
          void notifyError(t("registrationFailureTitle"), translateAuthMessage(message));
          return;
        }

        setRegistrationSucceeded(true);
        void notifySuccess(t("registrationSuccessNotice"));
        try {
          await login({ email: normalizedEmail, password });
        } catch (cause) {
          const message = authErrorMessage(cause, "autoLoginFailed");
          const honestMessage = authMessage("registrationComplete", undefined, message);
          setError(honestMessage);
          void notifyError(t("loginFailureTitle"), translateAuthMessage(message));
          return;
        }
      } else {
        const loggedInAccount = await login({ email: normalizedEmail, password });
        if (adminMode && loggedInAccount.role !== "ADMIN") {
          setError(authMessage("adminLoginForbidden"));
          return;
        }
        void notifySuccess(t("loginSuccess"));
      }
      navigate(returnTo, { replace: true });
    } catch (cause) {
      const message = authErrorMessage(cause, "loginFailed");
      setError(message);
      void notifyError(t("loginFailureTitle"), translateAuthMessage(message));
    } finally {
      setSubmitting(false);
    }
  };

  const switchAccount = async () => {
    if (switchingAccount) return;
    setSwitchingAccount(true);
    try {
      await logout();
    } finally {
      setSwitchingAccount(false);
    }
  };

  if (adminMode && status === "authenticated" && account?.role !== "ADMIN") {
    return <main className="auth-page">
      <LanguageSwitcher />
      <div className="state-panel text-center flex flex-col items-center gap-4 max-w-md w-full">
        <h1>{t("adminLoginForbidden")}</h1>
        <p>{t("adminLoginForbiddenDescription")}</p>
        <button type="button" className="action-button w-full justify-center" disabled={switchingAccount} aria-busy={switchingAccount} onClick={() => void switchAccount()}>
          {switchingAccount ? t("loggingOut") : t("signOutToSwitchAccount")}
        </button>
      </div>
    </main>;
  }

  if (status === "authenticated") {
    return <main className="auth-page"><LanguageSwitcher /><div className="state-panel text-center flex flex-col items-center gap-4 max-w-md w-full"><h1>{t("alreadySignedIn")}</h1><Link className="action-button w-full justify-center bg-[var(--color-primary)] text-white hover:bg-[var(--color-primary-strong)] border-transparent" to={returnTo}>{adminMode ? t("adminLink") : t("goToReports")}</Link></div></main>;
  }

  return <main className="auth-page">
    <LanguageSwitcher />
    <Link to="/" className="auth-brand" aria-label={t("brandHomeAria")}>
      <img src="/flood-watch-mark.svg" alt="" />
      <div><strong>Flood Watch</strong><small>{t("brandTagline")}</small></div>
    </Link>

    <form className="auth-form data-section" onSubmit={submit} noValidate>
      <header className="auth-heading" style={{ textAlign: "left" }}>
        <h1>{adminMode ? t("adminLoginTitle") : t(registering ? "registerTitle" : "loginTitle")}</h1>
        <p>{adminMode ? t("adminLoginDescription") : t("authDescription")}</p>
      </header>

      {error && <p className="form-error" role="alert">{formatAuthMessage(error, t)}</p>}
      {registrationSucceeded && <p className="form-success" role="status" aria-label={t("registrationSuccessAria")} style={{ margin: 0, padding: "0.75rem", borderRadius: "0.75rem", color: "var(--color-source-community-ink)", background: "var(--color-source-community)" }}>{t("registrationSuccess")} <Link to={`/login?returnTo=${encodeURIComponent(returnTo)}`}>{t("signInWithCreatedAccount")}</Link></p>}
      {googleError && <p className="form-error" role="alert">{formatAuthMessage(googleError, t)}</p>}

      <label>
        {registering ? t("email") : t("emailOrUsername")}
        <input
          type={registering ? "email" : "text"}
          autoComplete={registering ? "email" : "username"}
          value={email}
          onChange={(event) => setEmail(event.target.value)}
        />
      </label>

      {registering && (
        <>
          <label>
            {t("username")}
            <input
              type="text"
              autoComplete="username"
              value={username}
              onChange={(event) => setUsername(event.target.value)}
            />
          </label>
          <small>{t("usernameRule")}</small>
        </>
      )}

      <label>
        {t("password")}
        <input
          type={showPassword ? "text" : "password"}
          autoComplete={registering ? "new-password" : "current-password"}
          value={password}
          onChange={(event) => setPassword(event.target.value)}
        />
      </label>
      <small>{t("authPasswordHint")}</small>
      <details>
        <summary>{t("passwordTechnicalDetailsSummary")}</summary>
        <small>{t("passwordTechnicalDetails")}</small>
      </details>

      <label className="auth-show-password">
        <input
          type="checkbox"
          checked={showPassword}
          onChange={(event) => setShowPassword(event.target.checked)}
          aria-label={showPassword ? t("hidePassword") : t("showPassword")}
        />
        {t("showPassword")}
      </label>

      <div className="auth-actions">
        <Link className="auth-text-action" to={`/forgot-password?returnTo=${encodeURIComponent(returnTo)}`}>
          {t("forgotPassword")}
        </Link>
        <button type="submit" className="action-button" disabled={submitting}>
          {submitting ? t("processing") : registering ? t("registerTitle") : adminMode ? t("adminLoginTitle") : t("loginTitle")} <span aria-hidden="true">→</span>
        </button>
      </div>

      {!adminMode && <>
        <div className="auth-divider">{t("or")}</div>

        <GoogleSignInButton returnTo={returnTo} />

        <p className="section-caption">
          {registering ? (
            <>{t("haveAccount")} <Link to={`/login?returnTo=${encodeURIComponent(returnTo)}`}>{t("signIn")}</Link></>
          ) : (
            <>{t("noAccount")} <Link to={`/register?returnTo=${encodeURIComponent(returnTo)}`}>{t("createAccount")}</Link></>
          )}
        </p>
      </>}
    </form>
  </main>;
}
