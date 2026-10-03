import { useEffect, useRef, useState } from "react";
import { Link, useLocation, useNavigate } from "react-router";
import { ApiError, passwordRecoveryApi } from "./api";
import type { PasswordRecoveryApi } from "./api";
import { safeReturnTo } from "./google";

type RecoveryStep = 1 | 2 | 3 | "success";

const errorMessages: Record<string, string> = {
  INVALID_REQUEST: "ข้อมูลคำขอไม่ถูกต้อง กรุณาตรวจสอบข้อมูลแล้วลองใหม่",
  INVALID_OTP: "รหัส OTP ไม่ถูกต้อง หมดอายุ หรือถูกใช้ไปแล้ว กรุณาขอรหัสใหม่",
  INVALID_RESET_TOKEN: "สิทธิ์ตั้งรหัสผ่านหมดอายุหรือไม่ถูกต้อง กรุณาเริ่มกู้คืนรหัสผ่านใหม่",
  RECOVERY_RATE_LIMITED: "ดำเนินการกู้คืนรหัสผ่านบ่อยเกินไป กรุณารอสักครู่แล้วลองใหม่",
  RATE_LIMITED: "ดำเนินการกู้คืนรหัสผ่านบ่อยเกินไป กรุณารอสักครู่แล้วลองใหม่",
  RECOVERY_UNAVAILABLE: "บริการกู้คืนรหัสผ่านไม่พร้อมใช้งาน กรุณาลองใหม่ภายหลัง"
};

function recoveryError(cause: unknown): string {
  if (cause instanceof ApiError && cause.code && errorMessages[cause.code]) return errorMessages[cause.code];
  return "ไม่สามารถดำเนินการกู้คืนรหัสผ่านได้ กรุณาลองใหม่ภายหลัง";
}

function passwordError(password: string): string | null {
  const bytes = new TextEncoder().encode(password).byteLength;
  return bytes < 12 || bytes > 128 ? "รหัสผ่านต้องมีความยาว 12–128 ไบต์" : null;
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
  return (
    <main className="auth-page">
      <Link to="/" className="auth-brand" aria-label="Flood Watch Ongkharak หน้าหลัก">
        <img src="/flood-watch-mark.svg" alt="" />
        <div>
          <strong>Flood Watch</strong>
          <small>ติดตามสถานการณ์น้ำองครักษ์</small>
        </div>
      </Link>
      {children}
    </main>
  );
}

export function ForgotPasswordPage({ api = passwordRecoveryApi }: { api?: PasswordRecoveryApi }) {
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
  const [error, setError] = useState<string | null>(null);
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
    if (!normalizedEmail) return setError("กรุณากรอกอีเมล");
    if (!validEmail(normalizedEmail)) return setError("กรุณากรอกอีเมลให้ถูกต้อง");
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
    if (!/^\d{6}$/u.test(otp)) return setError("รหัส OTP ต้องเป็นตัวเลข 6 หลัก");
    if (!challengeId) return setError("คำขอกู้คืนหมดอายุ กรุณาเริ่มใหม่");
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
      setError("สิทธิ์ตั้งรหัสผ่านหมดอายุ กรุณาเริ่มกู้คืนรหัสผ่านใหม่");
      return;
    }
    const newPasswordError = passwordError(newPassword);
    if (newPasswordError) return setError(newPasswordError);
    if (newPassword !== confirmPassword) return setError("รหัสผ่านไม่ตรงกัน");
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
          <Header heading="เปลี่ยนรหัสผ่านสำเร็จ" headingId="recovery-success-heading" headingRef={headingRef}>คุณสามารถใช้รหัสผ่านใหม่เพื่อเข้าสู่ระบบได้แล้ว</Header>
          <button type="button" className="action-button" disabled={loading} onClick={() => navigate(loginPath, { replace: true })} style={{ minHeight: "3rem", justifyContent: "center" }}>
            กลับไปหน้าเข้าสู่ระบบ
          </button>
        </section>
      </Shell>
    );
  }

  if (step === 1) {
    return (
      <Shell>
        <form className="auth-form data-section" onSubmit={handleRequestOtp} noValidate>
          <Header heading="ลืมรหัสผ่าน?" headingRef={headingRef}>ระบุอีเมลที่ใช้สมัครบัญชีเพื่อเริ่มกู้คืนรหัสผ่าน</Header>
          {error && <p className="form-error" role="alert">{error}</p>}
          <label>
            อีเมล
            <input type="email" autoComplete="email" value={email} onChange={(event) => handleEmailChange(event.target.value)} disabled={loading} placeholder="อีเมลของคุณ" />
          </label>
          <div className="auth-actions" style={{ marginTop: "0.5rem" }}>
            <button type="submit" className="action-button" disabled={loading} style={{ width: "100%", justifyContent: "center" }}>
              {loading ? "กำลังตรวจสอบ..." : "ส่งรหัส OTP"}
            </button>
          </div>
          <p className="section-caption">
            จำรหัสผ่านได้แล้ว? <Link to={loginPath}>กลับไปหน้าเข้าสู่ระบบ</Link>
          </p>
        </form>
      </Shell>
    );
  }

  if (step === 2) {
    return (
      <Shell>
        <form className="auth-form data-section" onSubmit={handleVerifyOtp} noValidate>
          <Header heading="ยืนยันรหัส OTP" headingRef={headingRef}>หากอีเมลนี้มีบัญชี คุณอาจได้รับ <strong>รหัส OTP สำหรับ {email}</strong> เพื่อดำเนินการต่อ</Header>
          {requestMessage && <p role="status" aria-label="ข้อมูลการกู้คืน">{requestMessage}</p>}
          {error && <p className="form-error" role="alert">{error}</p>}
          <label>
            รหัส OTP
            <input type="text" inputMode="numeric" pattern="[0-9]{6}" maxLength={6} autoComplete="one-time-code" value={otp} onChange={(event) => setOtp(event.target.value)} disabled={loading} placeholder="ตัวเลข 6 หลัก" />
          </label>
          <div className="auth-actions" style={{ marginTop: "0.5rem" }}>
            <button type="submit" className="action-button" disabled={loading} style={{ width: "100%", justifyContent: "center" }}>
              {loading ? "กำลังตรวจสอบ..." : "ยืนยันรหัส OTP"}
            </button>
          </div>
          <p role="timer" aria-live="off" aria-label="เวลารอส่งรหัสอีกครั้ง">
            {resendRemaining > 0 ? `ส่งรหัสอีกครั้งได้ใน ${resendRemaining} วินาที` : "สามารถขอรหัสใหม่ได้แล้ว"}
          </p>
          <div className="auth-actions" style={{ marginTop: "0.5rem" }}>
            <button type="button" className="action-button" disabled={loading || resendRemaining > 0} onClick={() => void requestOtp()} style={{ width: "100%", justifyContent: "center", backgroundColor: "transparent", color: "var(--color-ink-muted)", border: "1px solid var(--color-border)" }}>
              ส่งรหัสอีกครั้ง
            </button>
          </div>
          <button type="button" className="auth-text-action" disabled={loading} onClick={editEmail}>แก้ไขอีเมล</button>
        </form>
      </Shell>
    );
  }

  return (
    <Shell>
      <form className="auth-form data-section" onSubmit={handleResetPassword} noValidate>
        <Header heading="ตั้งรหัสผ่านใหม่" headingRef={headingRef}>ตั้งรหัสผ่านใหม่เพื่อใช้เข้าสู่ระบบในครั้งถัดไป</Header>
        {error && <p className="form-error" role="alert">{error}</p>}
        <label>
          รหัสผ่านใหม่
          <input type="password" autoComplete="new-password" value={newPassword} onChange={(event) => setNewPassword(event.target.value)} disabled={loading} placeholder="ความยาว 12–128 ไบต์" />
        </label>
        <label>
          ยืนยันรหัสผ่านใหม่
          <input type="password" autoComplete="new-password" value={confirmPassword} onChange={(event) => setConfirmPassword(event.target.value)} disabled={loading} placeholder="พิมพ์รหัสผ่านใหม่อีกครั้ง" />
        </label>
        <p className="section-caption" style={{ textAlign: "left" }}>รหัสผ่านต้องมีความยาว 12–128 ไบต์</p>
        <div className="auth-actions" style={{ marginTop: "0.5rem" }}>
          <button type="submit" className="action-button" disabled={loading} style={{ width: "100%", justifyContent: "center" }}>
            {loading ? "กำลังบันทึก..." : "เปลี่ยนรหัสผ่าน"}
          </button>
        </div>
        <button type="button" className="auth-text-action" disabled={loading} onClick={editEmail}>เริ่มกู้คืนใหม่</button>
      </form>
    </Shell>
  );
}
