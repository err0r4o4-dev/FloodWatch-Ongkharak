import { Link } from "react-router";

export function ForgotPasswordPage() {
  return <section className="min-h-screen flex flex-col items-center justify-center p-4 bg-[var(--color-canvas)]">
    <Link to="/" className="brand mb-8" aria-label="Flood Watch Ongkharak หน้าหลัก">
      <img className="w-10 h-10" src="/flood-watch-mark.svg" alt="" />
      <span><strong>Flood Watch</strong><small>ติดตามสถานการณ์น้ำองครักษ์</small></span>
    </Link>

    <div className="data-section w-full max-w-md flex flex-col gap-5 text-center">
      <div className="mb-2">
        <h1 className="text-2xl font-bold mb-1">ลืมรหัสผ่าน?</h1>
        <p className="text-[var(--color-ink-muted)] text-sm">การกู้คืนรหัสผ่านยังรอเชื่อมต่อ API กรุณาอย่ากรอกข้อมูลส่วนตัวในหน้านี้</p>
      </div>

      <label className="flex flex-col gap-1 text-sm font-bold text-[var(--color-ink-muted)] text-left">
        อีเมล
        <input
          type="email"
          autoComplete="email"
          disabled
          placeholder="ยังไม่เปิดรับคำขอ"
          className="w-full min-h-[var(--touch-target)] px-3 py-2 border border-[var(--color-border)] rounded-[var(--radius-sm)] bg-[var(--color-canvas-soft)] text-[var(--color-ink-muted)] cursor-not-allowed font-normal"
        />
      </label>

      <div className="flex flex-col gap-4 mt-2">
        <button type="button" className="action-button w-full flex items-center justify-center text-[1rem] bg-[var(--color-surface)] text-[var(--color-ink-muted)] opacity-70 cursor-not-allowed" disabled>
          ส่งลิงก์ตั้งรหัสผ่านใหม่ · รอ API
        </button>
      </div>

      <p className="text-center mt-2 text-sm text-[var(--color-ink-muted)]">
        <Link className="text-[var(--color-primary-strong)] font-bold hover:underline" to="/login">กลับไปหน้าเข้าสู่ระบบ</Link>
      </p>
    </div>
  </section>;
}
