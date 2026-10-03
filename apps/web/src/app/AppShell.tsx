import type { ReactNode } from "react";
import { NavLink, useLocation } from "react-router";
import { useOptionalAuth } from "../features/auth/AuthProvider";

const links = [
  { to: "/", label: "ภาพรวม", symbol: "home" },
  { to: "/announcements", label: "ประกาศ", symbol: "bell" },
  { to: "/risk", label: "พื้นที่เสี่ยง", symbol: "pin" },
  { to: "/map", label: "แผนที่", symbol: "map" },
  { to: "/reports", label: "รายงาน", symbol: "report" }
];

function Icon({ name }: { name: string }) {
  const paths: Record<string, ReactNode> = {
    home: <><path d="m3 10 9-7 9 7v10a1 1 0 0 1-1 1H4a1 1 0 0 1-1-1z" /><path d="M9 21v-8h6v8" /></>,
    bell: <><path d="M18 8a6 6 0 0 0-12 0c0 7-3 7-3 9h18c0-2-3-2-3-9" /><path d="M10 21h4" /></>,
    pin: <><path d="M20 10c0 5-8 11-8 11S4 15 4 10a8 8 0 1 1 16 0Z" /><circle cx="12" cy="10" r="2.5" /></>,
    map: <><path d="m3 5 6-2 6 2 6-2v16l-6 2-6-2-6 2z" /><path d="M9 3v16M15 5v16" /></>,
    report: <><rect x="5" y="3" width="14" height="18" rx="2" /><path d="M9 8h6M9 12h6M9 16h4" /></>,
    settings: <><path d="M3 7h4m4 0h10M3 17h10m4 0h4" /><circle cx="9" cy="7" r="2" /><circle cx="15" cy="17" r="2" /></>,
    user: <><circle cx="12" cy="8" r="4" /><path d="M4 21a8 8 0 0 1 16 0" /></>
  };
  return <svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">{paths[name]}</svg>;
}

function Navigation({ mobile = false }: { mobile?: boolean }) {
  const { account } = useOptionalAuth();
  const isAdmin = account?.role === "ADMIN";

  return <nav aria-label={mobile ? "การนำทางมือถือ" : "การนำทางหลัก"} className={mobile ? "bottom-navigation" : "sidebar-navigation"}>
    <ul>{links.map(({ to, label, symbol }) => <li key={to}><NavLink to={to} end={to === "/"} className={({ isActive }) => `shell-nav-link${isActive ? " is-active" : ""}`}>
      <span className="shell-nav-symbol" aria-hidden="true"><Icon name={symbol} /></span><span>{label}</span>
    </NavLink></li>)}
      {isAdmin && !mobile && <li className="admin-nav-divider"><NavLink to="/admin" className={({ isActive }) => `shell-nav-link admin-nav-link${isActive ? " is-active" : ""}`}>
      <span className="shell-nav-symbol" aria-hidden="true"><Icon name="settings" /></span><span>จัดการระบบ</span>
    </NavLink></li>}
    </ul>
  </nav>;
}

export function AppShell({ children }: { children: ReactNode }) {
  const { account, logout } = useOptionalAuth();
  const location = useLocation();
  if (["/login", "/register", "/forgot-password"].includes(location.pathname)) return <main id="main-content">{children}</main>;
  return <div className="app-shell public-shell">
    <a className="skip-link" href="#main-content">ข้ามไปยังเนื้อหาหลัก</a>
    <aside className="desktop-sidebar">
      <NavLink to="/" className="brand" aria-label="Flood Watch Ongkharak หน้าหลัก"><img className="brand__image" src="/flood-watch-mark.svg" alt="" /><span><strong>Flood Watch</strong><small>Ongkharak</small></span></NavLink>
      <Navigation />
      <aside className="sidebar-status" aria-labelledby="sidebar-status-title"><span className="sidebar-status__dot" aria-hidden="true" /><div><strong id="sidebar-status-title">ข้อควรทราบ</strong><p>ข้อมูลไม่พร้อมใช้งานไม่ได้หมายถึงพื้นที่ปลอดภัย</p></div></aside>
    </aside>
    <div className="shell-main"><div className="shell-top">
      <header className="institution-header"><div className="w-full max-w-[1600px] mx-auto px-4 sm:px-5 md:px-5 lg:px-5 flex items-center justify-between gap-4"><span className="institution-header__name">ศูนย์ติดตามสถานการณ์น้ำ อำเภอองครักษ์ จังหวัดนครนายก</span><div className="institution-header__actions"><div className="locale-toggle" aria-label="ภาษา"><span aria-current="true">TH</span><button type="button" disabled title="ภาษาอังกฤษยังไม่พร้อมใช้งาน">EN</button></div><NavLink className="header-icon" to="/#notification-settings" aria-label="ตั้งค่าการแจ้งเตือน"><Icon name="bell" /></NavLink><details className="account-menu" onKeyDown={(event) => { if (event.key === "Escape") event.currentTarget.open = false; }}><summary aria-label="เมนูบัญชี"><Icon name="user" /> <span aria-hidden="true">⌄</span></summary><div className="account-menu__content">{account ? <><span>{account.email}</span>{account.role === "ADMIN" && <NavLink to="/admin">จัดการระบบ</NavLink>}<button type="button" onClick={() => void logout()}>ออกจากระบบ</button></> : <><NavLink to="/login">เข้าสู่ระบบ</NavLink><NavLink to="/register">สมัครสมาชิก</NavLink></>}</div></details></div></div></header>
      <header className="mobile-header"><div className="w-full max-w-[1600px] mx-auto px-4 sm:px-5 md:px-5 lg:px-5 flex items-center h-full"><NavLink to="/" className="brand" aria-label="Flood Watch Ongkharak หน้าหลัก"><img className="brand__image" src="/flood-watch-mark.svg" alt="" /><span><strong>Flood Watch</strong><small>Ongkharak</small></span></NavLink></div></header>
    </div>
      <main id="main-content" className="page-content w-full max-w-[1600px] mx-auto px-4 sm:px-5 md:px-5 lg:px-5" tabIndex={-1}>{children}</main>
    </div>
    <Navigation mobile />
  </div>;
}
