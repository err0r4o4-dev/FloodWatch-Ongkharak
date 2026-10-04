import { useTranslation, LanguageSwitcher } from "../i18n";
import th from "../i18n/locales/th/common";
import en from "../i18n/locales/en/common";
const messages = { th, en };
import { useRef, useState } from "react";
import type { ReactNode } from "react";
import { NavLink, useLocation, useNavigate } from "react-router";
import { useOptionalAuth } from "../features/auth/AuthProvider";
import { confirmAction } from "../shared/alerts";

const links = (t: (key: keyof typeof th) => string) => [
  { to: "/overview", label: t("overview"), symbol: "home" },
  { to: "/announcements", label: t("announcements"), symbol: "bell" },
  { to: "/map", label: t("mapAndRiskAreas"), symbol: "map" },
  { to: "/reports", label: t("reports"), symbol: "report" }
];

function Icon({ name }: { name: string }) {
  const paths: Record<string, ReactNode> = {
    home: <><path d="m3 10 9-7 9 7v10a1 1 0 0 1-1 1H4a1 1 0 0 1-1-1z" /><path d="M9 21v-8h6v8" /></>,
    bell: <><path d="M18 8a6 6 0 0 0-12 0c0 7-3 7-3 9h18c0-2-3-2-3-9" /><path d="M10 21h4" /></>,
    pin: <><path d="M20 10c0 5-8 11-8 11S4 15 4 10a8 8 0 1 1 16 0Z" /><circle cx="12" cy="10" r="2.5" /></>,
    map: <><path d="m3 5 6-2 6 2 6-2v16l-6 2-6-2-6 2z" /><path d="M9 3v16M15 5v16" /></>,
    report: <><rect x="5" y="3" width="14" height="18" rx="2" /><path d="M9 8h6M9 12h6M9 16h4" /></>,
    settings: <><path d="M3 7h4m4 0h10M3 17h10m4 0h4" /><circle cx="9" cy="7" r="2" /><circle cx="15" cy="17" r="2" /></>,
    user: <><circle cx="12" cy="8" r="4" /><path d="M4 21a8 8 0 0 1 16 0" /></>,
    chevron: <><path d="m6 9 6 6 6-6" /></>
  };
  return <svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">{paths[name]}</svg>;
}

function Navigation({ mobile = false }: { mobile?: boolean }) {
  const { t } = useTranslation(messages);
  const { account } = useOptionalAuth();
  const isAdmin = account?.role === "ADMIN";

  return <nav aria-label={mobile ? t("mobileNavigation") : t("mainNavigation")} className={mobile ? "bottom-navigation" : "sidebar-navigation"}>
    <ul>{links(t).map(({ to, label, symbol }) => <li key={to}><NavLink to={to} end={to === "/overview"} className={({ isActive }) => `shell-nav-link${isActive ? " is-active" : ""}`}>
      <span className="shell-nav-symbol" aria-hidden="true"><Icon name={symbol} /></span><span>{label}</span>
    </NavLink></li>)}
      {isAdmin && !mobile && <li className="admin-nav-divider"><NavLink to="/admin" className={({ isActive }) => `shell-nav-link admin-nav-link${isActive ? " is-active" : ""}`}>
      <span className="shell-nav-symbol" aria-hidden="true"><Icon name="settings" /></span><span>{t("administration")}</span>
    </NavLink></li>}
    </ul>
  </nav>;
}

export function AppShell({ children }: { children: ReactNode }) {
  const { t } = useTranslation(messages);
  const { logout } = useOptionalAuth();
  const location = useLocation();
  const navigate = useNavigate();
  const [signingOut, setSigningOut] = useState(false);
  const logoutPending = useRef(false);
  const handleLogout = async () => {
    if (logoutPending.current) return;
    logoutPending.current = true;
    setSigningOut(true);
    try {
      const completed = await confirmAction({
        title: t("signOut"),
        text: t("youWillNeedToSignInAgain"),
        confirmText: t("signOut9"),
        cancelText: t("staySignedIn"),
        danger: true,
        action: logout,
        successTitle: t("youAreSignedOut"),
        errorTitle: t("couldNotConfirmSignOut"),
        errorText: t("yourAccountHasBeenClearedFromThis")
      });
      if (completed) navigate("/", { replace: true });
    } finally {
      logoutPending.current = false;
      setSigningOut(false);
    }
  };
  if (location.pathname === "/") return <>{children}</>;
  if (["/admin/login", "/login", "/register", "/forgot-password", "/auth/complete", "/auth/google/link"].includes(location.pathname)) return <>{children}</>;
  return <div className="app-shell public-shell">
    <a className="skip-link" href="#main-content">{t("skipToMainContent")}</a>
    <aside className="desktop-sidebar">
      <NavLink to="/" className="brand" aria-label={t("floodWatchOngkharakHome")}><img className="brand__image" src="/flood-watch-mark.svg" alt="" /><span><strong>Flood Watch</strong><small>Ongkharak</small></span></NavLink>
      <Navigation />
      <aside className="sidebar-status" aria-labelledby="sidebar-status-title"><span className="sidebar-status__dot" aria-hidden="true" /><div><strong id="sidebar-status-title">{t("pleaseNote")}</strong><p>{t("unavailableDataDoesNotMeanAnArea")}</p></div></aside>
    </aside>
    <div className="shell-main"><div className="shell-top">
      <header className="institution-header"><div className="w-full max-w-[1600px] mx-auto px-4 sm:px-5 md:px-5 lg:px-5 flex items-center justify-between gap-4"><span className="institution-header__name">{t("waterSituationMonitoringCentreOngkharakNakhonNayok")}</span><div className="institution-header__actions"><LanguageSwitcher /></div></div></header>
      <header className="mobile-header"><div className="w-full max-w-[1600px] mx-auto px-4 sm:px-5 md:px-5 lg:px-5 flex items-center justify-between h-full w-full"><NavLink to="/" className="brand" aria-label={t("floodWatchOngkharakHome")}><img className="brand__image" src="/flood-watch-mark.svg" alt="" /><span><strong>Flood Watch</strong><small>Ongkharak</small></span></NavLink><div className="institution-header__actions"><LanguageSwitcher /></div></div></header>
    </div>
      <main id="main-content" className="page-content w-full" tabIndex={-1}>
        <div className="page-content__inner w-full max-w-[1600px] mx-auto px-4 sm:px-5 md:px-5 lg:px-5">{children}</div>
      </main>
    </div>
    <Navigation mobile />
  </div>;
}
