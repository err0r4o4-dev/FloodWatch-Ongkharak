import { Link } from "react-router";

export function ForgotPasswordPage() {
  return <section className="feature-page auth-page"><Link to="/" className="auth-brand"><img src="/flood-watch-mark.svg" alt="" /><span><strong>Flood Watch</strong><small>ติดตามสถานการณ์น้ำองครักษ์</small></span></Link>
    <div className="data-section auth-form auth-pending"><div className="auth-heading"><h1>ลืมรหัสผ่าน?</h1><p>การกู้คืนรหัสผ่านยังรอเชื่อมต่อ API กรุณาอย่ากรอกข้อมูลส่วนตัวในหน้านี้</p></div>
      <label>อีเมล<input type="email" autoComplete="email" disabled placeholder="ยังไม่เปิดรับคำขอ" /></label>
      <button type="button" className="action-button" disabled>ส่งลิงก์ตั้งรหัสผ่านใหม่ · รอ API</button>
      <p className="section-caption"><Link to="/login">กลับไปหน้าเข้าสู่ระบบ</Link></p>
    </div>
  </section>;
}
