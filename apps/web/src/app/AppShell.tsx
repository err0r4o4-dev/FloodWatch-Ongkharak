import type { ReactNode } from "react";
import { NavLink } from "react-router";

type AppShellProps = {
  children: ReactNode;
};

export function AppShell({ children }: AppShellProps) {
  return (
    <div className="app-shell">
      <a className="skip-link" href="#main-content">
        ข้ามไปยังเนื้อหาหลัก
      </a>
      <header className="site-header">
        <div className="shell-container site-header__content">
          <NavLink className="brand" to="/" aria-label="Flood Watch Ongkharak หน้าหลัก">
            <span className="brand__mark" aria-hidden="true">
              FW
            </span>
            <span>
              <strong>Flood Watch</strong>
              <small>องครักษ์</small>
            </span>
          </NavLink>
          <nav aria-label="การนำทางหลัก">
            <ul className="primary-nav">
              <li>
                <NavLink className="nav-link" to="/" end>
                  ภาพรวม
                </NavLink>
              </li>
              <li>
                <NavLink className="nav-link" to="/system-status">
                  สถานะระบบ
                </NavLink>
              </li>
            </ul>
          </nav>
        </div>
      </header>
      <main id="main-content" className="shell-container page-content" tabIndex={-1}>
        {children}
      </main>
      <footer className="site-footer shell-container">
        <p>ระบบติดตามและแจ้งเตือนสถานการณ์น้ำ อำเภอองครักษ์ จังหวัดนครนายก</p>
        <p>ข้อมูลสาธารณะจะแสดงเมื่อแหล่งข้อมูลพร้อมใช้งาน</p>
      </footer>
    </div>
  );
}
