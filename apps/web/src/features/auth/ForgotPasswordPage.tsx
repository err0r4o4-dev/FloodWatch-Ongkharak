import { useState } from "react";
import { Link, useNavigate } from "react-router";

export function ForgotPasswordPage() {
  const navigate = useNavigate();
  const [step, setStep] = useState<1 | 2 | 3>(1);
  const [email, setEmail] = useState("");
  const [otp, setOtp] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleRequestOtp = (e: React.FormEvent) => {
    e.preventDefault();
    if (!email.trim()) return setError("กรุณากรอกอีเมล");
    setError(null);
    setLoading(true);
    // Simulate API call
    setTimeout(() => {
      setLoading(false);
      setStep(2);
    }, 1000);
  };

  const handleVerifyOtp = (e: React.FormEvent) => {
    e.preventDefault();
    if (!otp.trim()) return setError("กรุณากรอกรหัส OTP");
    setError(null);
    setLoading(true);
    // Simulate API call
    setTimeout(() => {
      setLoading(false);
      setStep(3);
    }, 1000);
  };

  const handleResetPassword = (e: React.FormEvent) => {
    e.preventDefault();
    if (!newPassword || !confirmPassword) return setError("กรุณากรอกรหัสผ่านให้ครบถ้วน");
    if (newPassword !== confirmPassword) return setError("รหัสผ่านไม่ตรงกัน");
    setError(null);
    setLoading(true);
    // Simulate API call
    setTimeout(() => {
      setLoading(false);
      // Ideally redirect or show success. We will show success inside step 3 or redirect.
      alert("เปลี่ยนรหัสผ่านสำเร็จ (จำลอง)");
      navigate("/login", { replace: true });
    }, 1000);
  };

  return (
    <main className="auth-page">
      <Link to="/" className="auth-brand" aria-label="Flood Watch Ongkharak หน้าหลัก">
        <img src="/flood-watch-mark.svg" alt="" />
        <div>
          <strong>Flood Watch</strong>
          <small>ติดตามสถานการณ์น้ำองครักษ์</small>
        </div>
      </Link>

      {step === 1 && (
        <form className="auth-form data-section" onSubmit={handleRequestOtp} noValidate>
          <header className="auth-heading">
            <h1>ลืมรหัสผ่าน?</h1>
            <p>ระบุอีเมลที่ใช้สมัครบัญชีเพื่อรับรหัส OTP สำหรับตั้งรหัสผ่านใหม่</p>
          </header>

          {error && <p className="form-error" role="alert">{error}</p>}

          <label>
            อีเมล
            <input
              type="email"
              autoComplete="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              disabled={loading}
              placeholder="อีเมลของคุณ"
            />
          </label>

          <div className="auth-actions" style={{ marginTop: "0.5rem" }}>
            <button type="submit" className="action-button" disabled={loading} style={{ width: "100%", justifyContent: "center" }}>
              {loading ? "กำลังส่ง..." : "ส่งรหัส OTP"}
            </button>
          </div>

          <p className="section-caption">
            จำรหัสผ่านได้แล้ว? <Link to="/login">กลับไปหน้าเข้าสู่ระบบ</Link>
          </p>
        </form>
      )}

      {step === 2 && (
        <form className="auth-form data-section" onSubmit={handleVerifyOtp} noValidate>
          <header className="auth-heading">
            <h1>ยืนยันรหัส OTP</h1>
            <p>รหัส OTP ได้ถูกส่งไปยัง <strong>{email}</strong> แล้ว</p>
          </header>

          {error && <p className="form-error" role="alert">{error}</p>}

          <label>
            รหัส OTP
            <input
              type="text"
              autoComplete="one-time-code"
              value={otp}
              onChange={(e) => setOtp(e.target.value)}
              disabled={loading}
              placeholder="ตัวเลข 6 หลัก"
            />
          </label>

          <div className="auth-actions" style={{ marginTop: "0.5rem" }}>
            <button type="submit" className="action-button" disabled={loading} style={{ width: "100%", justifyContent: "center" }}>
              {loading ? "กำลังตรวจสอบ..." : "ยืนยันรหัส OTP"}
            </button>
          </div>

          <div className="auth-actions" style={{ marginTop: "0.5rem" }}>
            <button
              type="button"
              className="action-button"
              disabled={loading}
              onClick={() => {
                setOtp("");
                setStep(1);
              }}
              style={{ width: "100%", justifyContent: "center", backgroundColor: "transparent", color: "var(--color-ink-muted)", border: "1px solid var(--color-border)" }}
            >
              แก้ไขอีเมล หรือส่งรหัสอีกครั้ง
            </button>
          </div>
        </form>
      )}

      {step === 3 && (
        <form className="auth-form data-section" onSubmit={handleResetPassword} noValidate>
          <header className="auth-heading">
            <h1>ตั้งรหัสผ่านใหม่</h1>
            <p>กรุณาตั้งรหัสผ่านใหม่เพื่อใช้เข้าสู่ระบบในครั้งถัดไป</p>
          </header>

          {error && <p className="form-error" role="alert">{error}</p>}

          <label>
            รหัสผ่านใหม่
            <input
              type="password"
              autoComplete="new-password"
              value={newPassword}
              onChange={(e) => setNewPassword(e.target.value)}
              disabled={loading}
              placeholder="อย่างน้อย 8 ตัวอักษร"
            />
          </label>

          <label>
            ยืนยันรหัสผ่านใหม่
            <input
              type="password"
              autoComplete="new-password"
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
              disabled={loading}
              placeholder="พิมพ์รหัสผ่านใหม่อีกครั้ง"
            />
          </label>

          <div className="auth-actions" style={{ marginTop: "0.5rem" }}>
            <button type="submit" className="action-button" disabled={loading} style={{ width: "100%", justifyContent: "center" }}>
              {loading ? "กำลังบันทึก..." : "เปลี่ยนรหัสผ่าน"}
            </button>
          </div>
        </form>
      )}
    </main>
  );
}
