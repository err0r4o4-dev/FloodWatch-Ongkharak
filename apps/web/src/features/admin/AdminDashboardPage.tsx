import { useEffect, useState } from "react";
import { NavLink } from "react-router";
import { useAuth } from "../auth/AuthProvider";
import { adminApi } from "./api";
import type { AdminDashboardStats } from "./types";

export function AdminDashboardPage() {
  const { account } = useAuth();
  const [stats, setStats] = useState<AdminDashboardStats | null>(null);
  const [statsLoading, setStatsLoading] = useState(false);
  const [statsError, setStatsError] = useState<string | null>(null);
  const [retry, setRetry] = useState(0);

  useEffect(() => {
    if (account?.role !== "ADMIN") {
      return;
    }

    let active = true;
    setStatsLoading(true);
    setStatsError(null);
    setStats(null);

    void adminApi.getDashboardStats()
      .then((dashboardStats) => {
        if (active) {
          setStats(dashboardStats);
        }
      })
      .catch((error: unknown) => {
        if (active) {
          setStatsError(error instanceof Error ? error.message : "ไม่สามารถโหลดสถิติแดชบอร์ดได้");
        }
      })
      .finally(() => {
        if (active) {
          setStatsLoading(false);
        }
      });

    return () => {
      active = false;
    };
  }, [account?.id, account?.role, retry]);

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

      <section className="panel admin-stats-panel" aria-labelledby="admin-stats-heading" aria-live="polite">
        <h2 id="admin-stats-heading">ภาพรวมระบบ</h2>
        {statsLoading && <p role="status">กำลังโหลดสถิติแดชบอร์ด...</p>}
        {statsError && (
          <div className="error-state" role="alert">
            <p>ไม่สามารถโหลดสถิติแดชบอร์ดได้</p>
            <p className="muted">{statsError}</p>
            <button type="button" className="action-button" onClick={() => setRetry((current) => current + 1)}>ลองโหลดสถิติอีกครั้ง</button>
          </div>
        )}
        {stats && (
          <dl className="admin-dashboard-grid">
            <div className="admin-card">
              <dt>ผู้ใช้งานทั้งหมด</dt>
              <dd>{stats.totalUsers}</dd>
            </div>
            <div className="admin-card">
              <dt>ผู้ใช้งานที่ใช้งานอยู่</dt>
              <dd>{stats.activeUsers}</dd>
            </div>
            <div className="admin-card">
              <dt>รายงานทั้งหมด</dt>
              <dd>{stats.totalReports}</dd>
            </div>
            <div className="admin-card">
              <dt>รายงานรอตรวจสอบ</dt>
              <dd>{stats.pendingReports}</dd>
            </div>
            <div className="admin-card">
              <dt>กิจกรรมใน 24 ชั่วโมงล่าสุด</dt>
              <dd>{stats.recentActivity}</dd>
            </div>
          </dl>
        )}
      </section>

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
