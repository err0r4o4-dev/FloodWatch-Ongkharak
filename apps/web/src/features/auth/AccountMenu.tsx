import { useEffect, useId, useRef, useState } from "react";
import type { FormEvent } from "react";
import { NavLink, useLocation } from "react-router";
import { useTranslation } from "../../i18n";
import { useGuestIdentity } from "../guest/identity";
import { useOptionalAuth } from "./AuthProvider";
import { ApiError } from "./api";
import { authMessage, authMessages, formatAuthMessage } from "./messages";
import type { AuthMessage } from "./messages";

type AccountMenuProps = {
  signingOut: boolean;
  onLogout(): void;
};

export function AccountMenu({ signingOut, onLogout }: AccountMenuProps) {
  const { t } = useTranslation(authMessages);
  const { account, status, updateUsername } = useOptionalAuth();
  const { displayName, setDisplayName } = useGuestIdentity();
  const location = useLocation();
  const detailsRef = useRef<HTMLDetailsElement>(null);
  const triggerRef = useRef<HTMLElement>(null);
  const editRef = useRef<HTMLButtonElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const savingRef = useRef(false);
  const returnFocus = useRef(false);
  const inputId = useId();
  const guestInputId = useId();
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState("");
  const [guestDraft, setGuestDraft] = useState(displayName);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<AuthMessage | null>(null);
  const [message, setMessage] = useState("");
  const isAdminPath = location.pathname === "/admin" || location.pathname.startsWith("/admin/");

  useEffect(() => {
    setGuestDraft(displayName);
  }, [displayName, isAdminPath]);

  useEffect(() => {
    if (editing) inputRef.current?.focus();
    else if (returnFocus.current && detailsRef.current?.open) {
      editRef.current?.focus();
      returnFocus.current = false;
    }
  }, [editing]);

  useEffect(() => {
    if (detailsRef.current) detailsRef.current.open = false;
    setEditing(false);
    setError(null);
    setMessage("");
  }, [account?.id, location.pathname]);

  useEffect(() => {
    const dismiss = (event: PointerEvent) => {
      if (event.target instanceof Node && !detailsRef.current?.contains(event.target) && detailsRef.current) {
        detailsRef.current.open = false;
      }
    };
    document.addEventListener("pointerdown", dismiss);
    return () => document.removeEventListener("pointerdown", dismiss);
  }, []);

  const beginEditing = () => {
    setDraft(account?.username ?? "");
    setError(null);
    setMessage("");
    setEditing(true);
  };

  const cancelEditing = () => {
    returnFocus.current = true;
    setEditing(false);
    setError(null);
  };

  const saveUsername = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (savingRef.current || signingOut) return;
    const username = draft.trim().toLowerCase();
    if (!/^[a-z0-9_]{3,32}$/.test(username)) {
      setError(authMessage("usernameInvalid"));
      inputRef.current?.focus();
      return;
    }
    savingRef.current = true;
    setSaving(true);
    setError(null);
    try {
      await updateUsername(username);
      returnFocus.current = true;
      setEditing(false);
      setMessage(t("usernameSaved"));
    } catch (failure) {
      setError(failure instanceof ApiError && failure.code === "USERNAME_EXISTS"
        ? authMessage("usernameExists")
        : failure instanceof ApiError && failure.status === 401
          ? authMessage("sessionExpired")
          : failure instanceof ApiError && failure.status === 400
            ? authMessage("usernameInvalid")
            : authMessage("saveFailed"));
    } finally {
      savingRef.current = false;
      setSaving(false);
    }
  };

  return <details className="account-menu" ref={detailsRef} onKeyDown={(event) => {
    if (event.key === "Escape") {
      event.preventDefault();
      event.currentTarget.open = false;
      triggerRef.current?.focus();
    }
  }}>
    <summary ref={triggerRef} aria-label={t("accountMenuAria")} aria-disabled={!isAdminPath || status !== "loading" ? undefined : true} className={account && isAdminPath ? "is-logged-in" : ""} onClick={(event) => {
      if (isAdminPath && status === "loading") event.preventDefault();
    }}>
      <span className="account-user-icon" aria-hidden="true">
        <svg viewBox="0 0 24 24" width="22" height="22" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round"><circle cx="12" cy="8" r="4" /><path d="M4 21a8 8 0 0 1 16 0" /></svg>
        {account && isAdminPath && <svg className="account-user-icon__check" viewBox="0 0 16 16" width="14" height="14"><circle cx="8" cy="8" r="7" fill="currentColor" /><path d="m4.5 8 2.2 2.2 4.8-4.8" fill="none" stroke="white" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" /></svg>}
      </span>
      <span className="account-chevron" aria-hidden="true"><svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="m6 9 6 6 6-6" /></svg></span>
      <span className="sr-only">{!isAdminPath ? displayName : status === "loading" ? t("accountChecking") : account ? t("accountSignedIn") : t("signIn")}</span>
    </summary>
    <div className="account-menu__content">
      {!isAdminPath ? <div className="account-menu__field">
        <label className="account-menu__label" htmlFor={guestInputId}>{t("guestDisplayName")}</label>
        <input id={guestInputId} value={guestDraft} maxLength={80} onChange={(event) => setGuestDraft(event.target.value)} autoComplete="nickname" />
        <p className="account-menu__hint">{t("guestDisplayNameHint")}</p>
        <button type="button" className="account-menu__save" onClick={() => setDisplayName(guestDraft)}>{t("save")}</button>
      </div> : account ? <>
        <div className="account-menu__field"><span className="account-menu__label">{t("email")}</span><span className="account-menu__value">{account.email}</span></div>
        <div className="account-menu__field">
          {editing ? <form onSubmit={(event) => void saveUsername(event)} noValidate aria-busy={saving}>
            <label className="account-menu__label" htmlFor={inputId}>{t("username")}</label>
            <input id={inputId} ref={inputRef} value={draft} onChange={(event) => { setDraft(event.target.value); setError(null); }} disabled={saving || signingOut} autoComplete="username" autoCapitalize="none" spellCheck={false} aria-invalid={Boolean(error)} aria-describedby={`${inputId}-hint${error ? ` ${inputId}-error` : ""}`} />
            <p id={`${inputId}-hint`} className="account-menu__hint">{t("usernameHint")}</p>
            {error && <p id={`${inputId}-error`} className="account-menu__error" role="alert">{formatAuthMessage(error, t)}</p>}
            <div className="account-menu__edit-actions"><button className="account-menu__save" type="submit" disabled={saving || signingOut}>{saving ? t("saving") : t("save")}</button><button type="button" disabled={saving || signingOut} onClick={cancelEditing}>{t("cancel")}</button></div>
          </form> : <><span className="account-menu__label">{t("username")}</span><div className="account-menu__username"><span className="account-menu__value">{account.username || t("usernameNotSet")}</span><button ref={editRef} type="button" aria-label={account.username ? t("editUsername") : t("setUsername")} disabled={signingOut} onClick={beginEditing}>{account.username ? t("edit") : t("setUsername")}</button></div></>}
        </div>
        <p className="account-menu__message" role="status">{message}</p>
        {account.role === "ADMIN" && <NavLink to="/admin">{t("adminLink")}</NavLink>}
        <button className="account-menu__logout" type="button" disabled={signingOut || saving} onClick={onLogout}><svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true"><path d="M9 5H4v14h5M9 12h12m-4-4 4 4-4 4" /></svg>{signingOut ? t("loggingOut") : t("logout")}</button>
      </> : status === "loading" ? <p role="status">{t("accountLoading")}</p> : <NavLink to="/admin/login">{t("signIn")}</NavLink>}
    </div>
  </details>;
}
