import { useState } from "react";
import { Link, useLocation, useNavigate } from "react-router";
import { notifyError, notifySuccess } from "../../shared/alerts";
import { ApiError } from "./api";
import { useAuth } from "./AuthProvider";
import { GoogleSignInButton } from "./GoogleSignInButton";
import { googleErrorMessage, safeReturnTo } from "./google";

function destination(search: string): string {
  const value = new URLSearchParams(search).get("returnTo");
  return safeReturnTo(value);
}

export function AuthPage({ mode }: { mode: "login" | "register" }) {
  const { login, register, status } = useAuth();
  const location = useLocation();
  const navigate = useNavigate();
  const [email, setEmail] = useState("");
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [showPassword, setShowPassword] = useState(false);
  const registering = mode === "register";
  const returnTo = destination(location.search);
  const googleError = googleErrorMessage(new URLSearchParams(location.search).get("googleError"));

  const submit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (submitting) return;
    if (!email.trim() || !password) return setError(registering ? "กรุณากรอกอีเมล ชื่อผู้ใช้งาน และรหัสผ่าน" : "กรอกอีเมลและรหัสผ่าน");
    if (registering && !username.trim()) return setError("กรุณากรอกชื่อผู้ใช้งาน");
    setSubmitting(true);
    setError(null);
    try {
      if (registering) await register({ email: email.trim(), username: username.trim(), password });
      await login({ email: email.trim(), password });
      void notifySuccess(registering ? "สมัครสมาชิกสำเร็จ" : "เข้าสู่ระบบสำเร็จ");
      navigate(returnTo, { replace: true });
    } catch (cause) {
      const message = cause instanceof ApiError ? cause.message : registering ? "ไม่สามารถสมัครสมาชิกได้" : "ไม่สามารถเข้าสู่ระบบได้";
      setError(message);
      void notifyError(registering ? "สมัครสมาชิกไม่สำเร็จ" : "เข้าสู่ระบบไม่สำเร็จ", message);
    } finally {
      setSubmitting(false);
    }
  };

  if (status === "authenticated") {
    return <main className="auth-page"><div className="state-panel text-center flex flex-col items-center gap-4 max-w-md w-full"><h1>คุณเข้าสู่ระบบแล้ว</h1><Link className="action-button w-full justify-center bg-[var(--color-primary)] text-white hover:bg-[var(--color-primary-strong)] border-transparent" to={returnTo}>ไปยังรายงานชุมชน</Link></div></main>;
  }

  return <main className="auth-page">
    <Link to="/" className="auth-brand" aria-label="Flood Watch Ongkharak หน้าหลัก">
      <img src="/flood-watch-mark.svg" alt="" />
      <div><strong>Flood Watch</strong><small>ติดตามสถานการณ์น้ำองครักษ์</small></div>
    </Link>

    <form className="auth-form data-section" onSubmit={submit} noValidate>
      <header className="auth-heading">
        <h1>{registering ? "สมัครสมาชิก" : "เข้าสู่ระบบ"}</h1>
        <p>เข้าสู่ระบบเพื่อส่งและติดตามรายงานสถานการณ์น้ำในพื้นที่</p>
      </header>

      {error && <p className="form-error" role="alert">{error}</p>}
      {googleError && <p className="form-error" role="alert">{googleError}</p>}

      <label>
        {registering ? "อีเมล" : "อีเมล หรือ ชื่อผู้ใช้งาน"}
        <input
          type={registering ? "email" : "text"}
          autoComplete={registering ? "email" : "username"}
          value={email}
          onChange={(event) => setEmail(event.target.value)}
        />
      </label>

      {registering && (
        <label>
          ชื่อผู้ใช้งาน
          <input
            type="text"
            autoComplete="username"
            value={username}
            onChange={(event) => setUsername(event.target.value)}
          />
        </label>
      )}

      <label>
        รหัสผ่าน
        <input
          type={showPassword ? "text" : "password"}
          autoComplete={registering ? "new-password" : "current-password"}
          value={password}
          onChange={(event) => setPassword(event.target.value)}
        />
      </label>

      <label className="auth-show-password">
        <input
          type="checkbox"
          checked={showPassword}
          onChange={(event) => setShowPassword(event.target.checked)}
          aria-label={showPassword ? "ซ่อนรหัสผ่าน" : "แสดงรหัสผ่าน"}
        />
        แสดงรหัสผ่าน
      </label>

      <div className="auth-actions">
        <Link className="auth-text-action" to={`/forgot-password?returnTo=${encodeURIComponent(returnTo)}`}>
          ลืมรหัสผ่าน?
        </Link>
        <button type="submit" className="action-button" disabled={submitting}>
          {submitting ? "กำลังดำเนินการ..." : registering ? "สมัครสมาชิก" : "เข้าสู่ระบบ"} <span aria-hidden="true">→</span>
        </button>
      </div>

      <div className="auth-divider">หรือ</div>

      <GoogleSignInButton returnTo={returnTo} />

      <p className="section-caption">
        {registering ? (
          <>มีบัญชีอยู่แล้ว? <Link to={`/login?returnTo=${encodeURIComponent(returnTo)}`}>เข้าสู่ระบบ</Link></>
        ) : (
          <>ยังไม่มีบัญชี? <Link to={`/register?returnTo=${encodeURIComponent(returnTo)}`}>สร้างบัญชีผู้ใช้</Link></>
        )}
      </p>
    </form>
  </main>;
}
