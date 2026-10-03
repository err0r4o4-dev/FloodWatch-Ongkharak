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

function passwordLengthMessage(password: string): string | null {
  const bytes = new TextEncoder().encode(password).byteLength;
  return bytes < 12 || bytes > 128 ? "รหัสผ่านต้องมีความยาว 12–128 ไบต์" : null;
}

function usernameError(username: string): string | null {
  const normalized = username.trim().toLowerCase();
  if (normalized.length < 3 || normalized.length > 32) return "ชื่อผู้ใช้งานต้องมีความยาว 3–32 ตัวอักษร";
  if (!/^[a-z0-9_]+$/u.test(normalized)) return "ชื่อผู้ใช้งานใช้ได้เฉพาะ a–z, 0–9 และ _";
  return null;
}

function authErrorMessage(cause: unknown, fallback: string): string {
  if (!(cause instanceof ApiError)) return fallback;
  switch (cause.code) {
    case "USERNAME_EXISTS": return "ชื่อผู้ใช้งานนี้ถูกใช้แล้ว กรุณาเลือกชื่ออื่น";
    case "EMAIL_EXISTS": return "อีเมลนี้มีบัญชีอยู่แล้ว กรุณาเข้าสู่ระบบหรือใช้ลืมรหัสผ่าน";
    case "INVALID_CREDENTIALS": return "อีเมล/ชื่อผู้ใช้งานหรือรหัสผ่านไม่ถูกต้อง";
    default: return cause.message || fallback;
  }
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
  const [registrationSucceeded, setRegistrationSucceeded] = useState(false);
  const registering = mode === "register";
  const returnTo = destination(location.search);
  const googleError = googleErrorMessage(new URLSearchParams(location.search).get("googleError"));

  const submit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (submitting) return;
    const normalizedEmail = email.trim();
    const normalizedUsername = username.trim().toLowerCase();
    if (!normalizedEmail || !password) return setError(registering ? "กรุณากรอกอีเมล ชื่อผู้ใช้งาน และรหัสผ่าน" : "กรอกอีเมลหรือชื่อผู้ใช้งาน และรหัสผ่าน");
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
          const message = authErrorMessage(cause, "ไม่สามารถสมัครสมาชิกได้");
          setError(message);
          void notifyError("สมัครสมาชิกไม่สำเร็จ", message);
          return;
        }

        setRegistrationSucceeded(true);
        void notifySuccess("สมัครสมาชิกสำเร็จ");
        try {
          await login({ email: normalizedEmail, password });
        } catch (cause) {
          const message = authErrorMessage(cause, "เข้าสู่ระบบอัตโนมัติไม่สำเร็จ");
          const honestMessage = `สมัครสมาชิกสำเร็จแล้ว แต่เข้าสู่ระบบอัตโนมัติไม่สำเร็จ: ${message}`;
          setError(honestMessage);
          void notifyError("เข้าสู่ระบบไม่สำเร็จ", message);
          return;
        }
      } else {
        await login({ email: normalizedEmail, password });
        void notifySuccess("เข้าสู่ระบบสำเร็จ");
      }
      navigate(returnTo, { replace: true });
    } catch (cause) {
      const message = authErrorMessage(cause, "ไม่สามารถเข้าสู่ระบบได้");
      setError(message);
      void notifyError("เข้าสู่ระบบไม่สำเร็จ", message);
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
      <header className="auth-heading" style={{ textAlign: "left" }}>
        <h1>{registering ? "สมัครสมาชิก" : "เข้าสู่ระบบ"}</h1>
        <p>เข้าสู่ระบบเพื่อส่งและติดตามรายงานสถานการณ์น้ำในพื้นที่</p>
      </header>

      {error && <p className="form-error" role="alert">{error}</p>}
      {registrationSucceeded && <p className="form-success" role="status" aria-label="สมัครสมาชิกสำเร็จ" style={{ margin: 0, padding: "0.75rem", borderRadius: "0.75rem", color: "var(--color-source-community-ink)", background: "var(--color-source-community)" }}>สมัครสมาชิกสำเร็จ บัญชีของคุณถูกสร้างแล้ว <Link to={`/login?returnTo=${encodeURIComponent(returnTo)}`}>เข้าสู่ระบบด้วยบัญชีที่สร้างไว้</Link></p>}
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
        <>
          <label>
            ชื่อผู้ใช้งาน
            <input
              type="text"
              autoComplete="username"
              value={username}
              onChange={(event) => setUsername(event.target.value)}
            />
          </label>
          <small>ใช้ a–z, 0–9 หรือ _ ความยาว 3–32 ตัวอักษร</small>
        </>
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
      <small>รหัสผ่านต้องมีความยาว 12–128 ไบต์</small>

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
