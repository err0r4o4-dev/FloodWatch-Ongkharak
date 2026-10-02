import { NavLink } from "react-router";
import { useAuth } from "../auth/AuthProvider";

export function AdminDashboardPage() {
  const { account } = useAuth();

  if (account?.role !== "ADMIN") {
    return (
      <div className="page-container">
        <div className="panel error-state">
          <h1>ไม่มีสิทธิ์เข้าถึง</h1>
          <p>คุณไม่มีสิทธิ์เข้าถึงส่วนจัดการระบบ</p>
          <NavLink to="/" className="button">กลับหน้าหลัก</NavLink>
        </div>
      </div>
    );
  }

  return (
    <div className="page-container admin-page">
      <header className="page-header">
        <h1>ส่วนจัดการระบบ</h1>
        <p>จัดการผู้ใช้งาน ตั้งค่าระบบ และตรวจสอบการดำเนินการ</p>
      </header>

      <div className="admin-dashboard-grid">
        <section className="panel admin-card">
          <h2>จัดการผู้ใช้งาน</h2>
          <p className="muted">ดูรายชื่อผู้ใช้งาน เปลี่ยนบทบาท และจัดการสถานะบัญชี</p>
          <NavLink to="/admin/users" className="button button--primary">
            เปิดการจัดการผู้ใช้งาน
          </NavLink>
        </section>

        <section className="panel admin-card">
          <h2>ตั้งค่าระบบ</h2>
          <p className="muted">กำหนดค่าการทำงานของระบบ อัตราการจำกัด และฟีเจอร์ต่างๆ</p>
          <NavLink to="/admin/settings" className="button button--primary">
            เปิดการตั้งค่า
          </NavLink>
        </section>

        <section className="panel admin-card">
          <h2>บันทึกการดำเนินการ</h2>
          <p className="muted">ตรวจสอบประวัติการกระทำของผู้ดูแลระบบและผู้ใช้งาน</p>
          <NavLink to="/admin/audit-logs" className="button button--primary">
            เปิดบันทึก
          </NavLink>
        </section>

        <section className="panel admin-card">
          <h2>รายงานจากชุมชน</h2>
          <p className="muted">ตรวจสอบและอนุมัติรายงานสถานการณ์จากชุมชน</p>
          <NavLink to="/admin/reports" className="button button--primary">
            ตรวจสอบรายงาน
          </NavLink>
        </section>

        <section className="panel admin-card">
          <h2>หลักฐานจาก AI</h2>
          <p className="muted">ตรวจสอบและยืนยันข้อมูลที่ประมวลผลโดย AI</p>
          <NavLink to="/admin/ai-evidence" className="button button--primary">
            ตรวจสอบหลักฐาน AI
          </NavLink>
        </section>
        <section className="panel admin-card">
          <h2>กฎการแจ้งเตือน</h2>
          <p className="muted">สร้างและจัดการกฎการแจ้งเตือนอัตโนมัติ</p>
          <NavLink to="/admin/alert-rules" className="button button--primary">
            จัดการการแจ้งเตือน
          </NavLink>
        </section>

        <section className="panel admin-card">
          <h2>สถานะระบบ</h2>
          <p className="muted">ตรวจสอบสุขภาพและประสิทธิภาพของระบบ</p>
          <NavLink to="/system-status" className="button button--primary">
            เปิดสถานะระบบ
          </NavLink>
        </section>
      </div>

      <aside className="admin-info-panel panel">
        <h3>ข้อมูลสำคัญ</h3>
        <ul>
          <li><strong>บัญชีของคุณ:</strong> {account.email}</li>
          <li><strong>บทบาท:</strong> ผู้ดูแลระบบ (ADMIN)</li>
        </ul>
        <p className="muted small">
          การกระทำทั้งหมดในส่วนนี้จะถูกบันทึกเพื่อความปลอดภัยและการตรวจสอบ
        </p>
      </aside>
    </div>
  );
}
