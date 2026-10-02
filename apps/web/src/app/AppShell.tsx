import type { ReactNode } from "react";
import { NavLink } from "react-router";
import { useOptionalAuth } from "../features/auth/AuthProvider";
import { useRealtimeStatus } from "../features/realtime/RealtimeProvider";

const links = [
  { to: "/", label: "ภาพรวม", symbol: "◉" },
  { to: "/announcements", label: "ประกาศ", symbol: "◇" },
  { to: "/map", label: "แผนที่", symbol: "▤" },
  { to: "/cctv", label: "กล้อง CCTV", symbol: "▣" },
  { to: "/reports", label: "รายงาน", symbol: "▦" }
];

function Navigation({ mobile = false }: { mobile?: boolean }) {
  const { account } = useOptionalAuth();
  const isAdmin = account?.role === "ADMIN";

  return <nav aria-label={mobile ? "การนำทางมือถือ" : "การนำทางหลัก"} className={mobile ? "bottom-navigation" : "sidebar-navigation"}>
    <ul>{links.map(({ to, label, symbol }) => <li key={to}><NavLink to={to} end={to === "/"} className={({ isActive }) => `shell-nav-link${isActive ? " is-active" : ""}`}>
      <span className="shell-nav-symbol" aria-hidden="true">{symbol}</span><span>{label}</span>
    </NavLink></li>)}
    {isAdmin && <li className="admin-nav-divider"><NavLink to="/admin" className={({ isActive }) => `shell-nav-link admin-nav-link${isActive ? " is-active" : ""}`}>
      <span className="shell-nav-symbol" aria-hidden="true">⚙</span><span>จัดการระบบ</span>
    </NavLink></li>}
    </ul>
  </nav>;
}

export function AppShell({ children }: { children: ReactNode }) {
  const realtimeStatus = useRealtimeStatus();
  return <div className="app-shell public-shell">
    <a className="skip-link" href="#main-content">ข้ามไปยังเนื้อหาหลัก</a>
    <aside className="desktop-sidebar">
      <NavLink to="/" className="brand" aria-label="Flood Watch Ongkharak หน้าหลัก"><span className="brand__mark" aria-hidden="true">FW</span><span><strong>Flood Watch</strong><small>องครักษ์</small></span></NavLink>
      <Navigation />
      <p className="sidebar-note">ข้อมูลที่ไม่พร้อมใช้งานไม่ได้หมายถึงปลอดภัย</p>
    </aside>
    <div className="shell-main"><header className="mobile-header"><NavLink to="/" className="brand" aria-label="Flood Watch Ongkharak หน้าหลัก"><span className="brand__mark" aria-hidden="true">FW</span><span><strong>Flood Watch</strong><small>องครักษ์</small></span></NavLink></header>
      <main id="main-content" className="page-content" tabIndex={-1}>{children}</main>
      <footer className="site-footer"><p>Flood Watch Ongkharak · ข้อมูลสาธารณะ</p><span role="status" className="realtime-indicator">{realtimeStatus === "CONNECTED" ? "อัปเดตอัตโนมัติพร้อมใช้งาน" : realtimeStatus === "RECONNECTING" ? "กำลังเชื่อมต่อการอัปเดต · ข้อมูลยังโหลดผ่าน API ได้" : "การอัปเดตอัตโนมัติไม่พร้อม · ข้อมูลยังโหลดผ่าน API ได้"}</span><NavLink to="/system-status">สถานะระบบ</NavLink></footer>
    </div>
    <Navigation mobile />
  </div>;
}
