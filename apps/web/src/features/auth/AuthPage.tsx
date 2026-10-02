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
    return <section className="feature-page auth-page"><div className="state-panel"><h1>คุณเข้าสู่ระบบแล้ว</h1><Link className="action-button" to={returnTo}>ไปยังรายงานชุมชน</Link></div></section>;
  }

  return <section className="feature-page auth-page"><div className="page-heading"><p className="eyebrow">COMMUNITY ACCOUNT</p><h1>{registering ? "สมัครสมาชิก" : "เข้าสู่ระบบ"}</h1><p>ใช้บัญชีของคุณเพื่อส่งและติดตามรายงานชุมชน</p></div>
    <form className="data-section auth-form" onSubmit={submit} noValidate>
      {error && <p className="form-error" role="alert">{error}</p>}
      <label>อีเมล<input type="email" autoComplete="email" value={email} onChange={(event) => setEmail(event.target.value)} /></label>
      <label>รหัสผ่าน<input type="password" autoComplete={registering ? "new-password" : "current-password"} value={password} onChange={(event) => setPassword(event.target.value)} /></label>
      <button type="submit" className="action-button" disabled={submitting}>{submitting ? "กำลังดำเนินการ..." : registering ? "สมัครสมาชิกและเข้าสู่ระบบ" : "เข้าสู่ระบบ"}</button>
      <p className="section-caption">{registering ? <>มีบัญชีอยู่แล้ว? <Link to={`/login?returnTo=${encodeURIComponent(returnTo)}`}>เข้าสู่ระบบ</Link></> : <>ยังไม่มีบัญชี? <Link to={`/register?returnTo=${encodeURIComponent(returnTo)}`}>สมัครสมาชิก</Link></>}</p>
    </form>
  </section>;
}
