import { useState } from "react";
import { Link, useLocation, useNavigate } from "react-router";
import { ApiError } from "./api";
import { useAuth } from "./AuthProvider";

function destination(search: string): string {
  const value = new URLSearchParams(search).get("returnTo");
  return value && value.startsWith("/") && !value.startsWith("//") ? value : "/reports";
}

export function AuthPage({ mode }: { mode: "login" | "register" }) {
  const { login, register, status } = useAuth();
  const location = useLocation();
  const navigate = useNavigate();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [showPassword, setShowPassword] = useState(false);
  const registering = mode === "register";
  const returnTo = destination(location.search);

  const submit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!email.trim() || !password) return setError("กรอกอีเมลและรหัสผ่าน");
    setSubmitting(true);
    setError(null);
    try {
      if (registering) await register({ email: email.trim(), password });
      await login({ email: email.trim(), password });
      navigate(returnTo, { replace: true });
    } catch (cause) {
      setError(cause instanceof ApiError ? cause.message : registering ? "ไม่สามารถสมัครสมาชิกได้" : "ไม่สามารถเข้าสู่ระบบได้");
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

      <label>
        อีเมล หรือ รหัสประจำตัว
        <input
          type="email"
          autoComplete="email"
          value={email}
          onChange={(event) => setEmail(event.target.value)}
        />
      </label>

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

      <button type="button" className="auth-google" disabled title="รอเชื่อมต่อ Google API">
        <strong aria-hidden="true" style={{ fontSize: "1.1rem", marginRight: "0.4rem" }}>G</strong> Continue with Google <small>เร็ว ๆ นี้</small>
      </button>

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
