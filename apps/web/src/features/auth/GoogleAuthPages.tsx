import { useState } from "react";
import { Link, Navigate, useLocation } from "react-router";
import { notifyError, notifySuccess } from "../../shared/alerts";
import { ApiError } from "./api";
import { useAuth } from "./AuthProvider";
import { googleAuthApi, googleErrorMessage, safeReturnTo } from "./google";
import type { GoogleAuthApi } from "./google";

const panelClass = "auth-form data-section";

export function GoogleCompletePage() {
  const { status } = useAuth();
  const location = useLocation();
  const returnTo = safeReturnTo(new URLSearchParams(location.search).get("returnTo"));
  if (status === "authenticated") return <Navigate to={returnTo} replace />;
  return <section className="auth-page"><div className={panelClass}>
    <h1 className="text-2xl font-bold">เข้าสู่ระบบด้วย Google</h1>
    {status === "loading" ? <p role="status">กำลังยืนยันการเข้าสู่ระบบ...</p> : <>
      <p className="form-error" role="alert">ยืนยันการเข้าสู่ระบบไม่สำเร็จ กรุณาลองเข้าสู่ระบบใหม่</p>
      <Link className="action-button w-full justify-center" to={`/login?returnTo=${encodeURIComponent(returnTo)}`}>กลับไปเข้าสู่ระบบ</Link>
    </>}
  </div></section>;
}

export function GoogleLinkPage({ api = googleAuthApi, navigate = (url: string) => window.location.assign(url) }: { api?: GoogleAuthApi; navigate?: (url: string) => void }) {
  const [password, setPassword] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [blocked, setBlocked] = useState(false);
  const submit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!password || submitting || blocked) return;
    setSubmitting(true);
    setError(null);
    try {
      const result = await api.link(password);
      setPassword("");
      await notifySuccess("เชื่อมบัญชี Google สำเร็จ");
      navigate(`/auth/complete?returnTo=${encodeURIComponent(safeReturnTo(result.returnTo))}`);
    } catch (failure) {
      const code = failure instanceof ApiError ? failure.code ?? "GOOGLE_LOGIN_FAILED" : "GOOGLE_LOGIN_FAILED";
      const message = googleErrorMessage(code) ?? "เชื่อมบัญชี Google ไม่สำเร็จ กรุณาลองใหม่";
      setError(message);
      void notifyError("เชื่อมบัญชี Google ไม่สำเร็จ", message);
      setBlocked(failure instanceof ApiError && [429, 409, 503].includes(failure.status));
      setSubmitting(false);
    }
  };
  return <section className="auth-page"><form className={panelClass} onSubmit={submit}>
    <h1 className="text-2xl font-bold">เชื่อมบัญชี Google</h1>
    <p>อีเมลนี้มีบัญชี Flood Watch อยู่แล้ว ยืนยันรหัสผ่านบัญชีเดิมเพื่อเชื่อม Google และเก็บรายงานของคุณไว้ในบัญชีเดิม</p>
    {error && <p className="form-error" role="alert">{error}</p>}
    <label className="flex flex-col gap-1 text-sm font-bold text-[var(--color-ink-muted)]">รหัสผ่านบัญชีเดิม
      <input type="password" autoComplete="current-password" required value={password} disabled={submitting || blocked} maxLength={128} onChange={(event) => setPassword(event.target.value)} className="w-full min-h-[var(--touch-target)] px-3 py-2 border border-[var(--color-border)] rounded-[var(--radius-sm)] bg-[var(--color-surface)] text-[var(--color-ink)] focus:outline-[3px] focus:outline-[var(--color-primary)] font-normal" />
    </label>
    <button type="submit" className="action-button w-full justify-center" disabled={!password || submitting || blocked}>{submitting ? "กำลังยืนยัน..." : "ยืนยันและเชื่อม Google"}</button>
    <Link className="text-[var(--color-primary-strong)] hover:underline" to="/login">กลับไปเข้าสู่ระบบ</Link>
  </form></section>;
}
