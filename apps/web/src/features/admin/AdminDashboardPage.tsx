import { useEffect, useState } from "react";
import { NavLink } from "react-router";
import { useAuth } from "../auth/AuthProvider";
import { formatNumber, useTranslation } from "../../i18n";
import th from "../../i18n/locales/th/admin";
import en from "../../i18n/locales/en/admin";
import { adminApi } from "./api";
import type { AdminDashboardStats } from "./types";
import { adminErrorState, formatAdminNumber } from "./localization";
import type { AdminErrorState } from "./localization";

// Simple inline SVG icons to make the UI look more modern without external dependencies
const UsersIcon = () => (
  <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2"></path>
    <circle cx="9" cy="7" r="4"></circle>
    <path d="M22 21v-2a4 4 0 0 0-3-3.87"></path>
    <path d="M16 3.13a4 4 0 0 1 0 7.75"></path>
  </svg>
);

const SettingsIcon = () => (
  <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <path d="M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.39a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z"></path>
    <circle cx="12" cy="12" r="3"></circle>
  </svg>
);

const ShieldIcon = () => (
  <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"></path>
  </svg>
);

const FileIcon = () => (
  <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <path d="M14.5 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7.5L14.5 2z"></path>
    <polyline points="14 2 14 8 20 8"></polyline>
    <line x1="16" y1="13" x2="8" y2="13"></line>
    <line x1="16" y1="17" x2="8" y2="17"></line>
    <line x1="10" y1="9" x2="8" y2="9"></line>
  </svg>
);

const CpuIcon = () => (
  <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <rect x="4" y="4" width="16" height="16" rx="2" ry="2"></rect>
    <rect x="9" y="9" width="6" height="6"></rect>
    <line x1="9" y1="1" x2="9" y2="4"></line>
    <line x1="15" y1="1" x2="15" y2="4"></line>
    <line x1="9" y1="20" x2="9" y2="23"></line>
    <line x1="15" y1="20" x2="15" y2="23"></line>
    <line x1="20" y1="9" x2="23" y2="9"></line>
    <line x1="20" y1="14" x2="23" y2="14"></line>
    <line x1="1" y1="9" x2="4" y2="9"></line>
    <line x1="1" y1="14" x2="4" y2="14"></line>
  </svg>
);

const BellIcon = () => (
  <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <path d="M18 8A6 6 0 0 0 6 8c0 7-3 9-3 9h18s-3-2-3-9"></path>
    <path d="M13.73 21a2 2 0 0 1-3.46 0"></path>
  </svg>
);

const ActivityIcon = () => (
  <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <polyline points="22 12 18 12 15 21 9 3 6 12 2 12"></polyline>
  </svg>
);

const LogoutIcon = () => (
  <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4"></path>
    <polyline points="16 17 21 12 16 7"></polyline>
    <line x1="21" y1="12" x2="9" y2="12"></line>
  </svg>
);

export function AdminDashboardPage() {
  const { account, logout } = useAuth();
  const { locale, t } = useTranslation({ th, en });
  const [stats, setStats] = useState<AdminDashboardStats | null>(null);
  const [statsLoading, setStatsLoading] = useState(false);
  const [statsError, setStatsError] = useState<AdminErrorState | null>(null);
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
          setStatsError(adminErrorState(error, "errors.loadDashboard"));
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
          <h1>{t("common.accessDenied")}</h1>
          <p>{t("common.adminOnly")}</p>
          <NavLink to="/" className="button">{t("common.home")}</NavLink>
        </div>
      </div>
    );
  }

  return (
    <div className="page-container admin-page">
      <header className="page-hero" style={{ marginBottom: '2.5rem' }}>
        <div className="page-hero__copy">
          <p className="eyebrow">{t("dashboard.title")}</p>
          <h1 tabIndex={-1}>{t("dashboard.title")}</h1>
          <p>{t("dashboard.description")}</p>
        </div>
        <div className="page-hero__art" aria-hidden="true">
          <div className="page-hero__tile page-hero__tile--large" style={{ color: "var(--color-primary-strong)", background: "var(--color-canvas-soft)" }}>
             <ShieldIcon />
          </div>
        </div>
      </header>

      <section aria-labelledby="admin-stats-heading" aria-live="polite" style={{ marginBottom: '3rem' }}>
        <h2 id="admin-stats-heading" className="sr-only">{t("dashboard.stats.title")}</h2>

        {statsLoading && <div className="panel"><p role="status">{t("dashboard.stats.loading")}</p></div>}

        {statsError && (
          <div className="panel error-state" role="alert">
            <p>{t("dashboard.stats.error")}</p>
            <p className="muted">{t(statsError.key, statsError.params)}</p>
            <button type="button" className="action-button" onClick={() => setRetry((current) => current + 1)}>{t("dashboard.stats.retry")}</button>
          </div>
        )}

        {stats && (
          <div className="summary-metrics" style={{ marginTop: 0, paddingInline: 0 }}>
            <div className="summary-metric">
              <div className="summary-metric__icon summary-metric__icon--road">
                <UsersIcon />
              </div>
              <span>
                <strong>{formatAdminNumber(stats.totalUsers, locale, formatNumber)}</strong>
                <small>{t("dashboard.stats.totalUsers")}</small>
              </span>
            </div>

            <div className="summary-metric">
              <div className="summary-metric__icon summary-metric__icon--success">
                <ActivityIcon />
              </div>
              <span>
                <strong>{formatAdminNumber(stats.activeUsers, locale, formatNumber)}</strong>
                <small>{t("dashboard.stats.activeUsers")}</small>
              </span>
            </div>

            <div className="summary-metric">
              <div className="summary-metric__icon summary-metric__icon--sky">
                <FileIcon />
              </div>
              <span>
                <strong>{formatAdminNumber(stats.totalReports, locale, formatNumber)}</strong>
                <small>{t("dashboard.stats.totalReports")}</small>
              </span>
            </div>

            <div className="summary-metric">
              <div className="summary-metric__icon summary-metric__icon--warning">
                <BellIcon />
              </div>
              <span>
                <strong>{formatAdminNumber(stats.pendingReports, locale, formatNumber)}</strong>
                <small>{t("dashboard.stats.pendingReports")}</small>
              </span>
            </div>
          </div>
        )}
      </section>

      <h2 style={{ fontSize: '1.25rem', marginBottom: '1.5rem', fontWeight: 700 }}>{t("dashboard.modules.title")}</h2>
      <div className="admin-dashboard-grid">
        <NavLink to="/admin/announcements" className="admin-module-card">
          <div className="admin-module-card__icon"><BellIcon /></div>
          <div className="admin-module-card__content">
            <h2>{t("dashboard.cards.announcements.title")}</h2>
            <p>{t("dashboard.cards.announcements.description")}</p>
            <span className="admin-module-card__action">{t("dashboard.cards.announcements.action")} &rarr;</span>
          </div>
        </NavLink>
        <NavLink to="/admin/users" className="admin-module-card">
          <div className="admin-module-card__icon"><UsersIcon /></div>
          <div className="admin-module-card__content">
            <h2>{t("dashboard.cards.users.title")}</h2>
            <p>{t("dashboard.cards.users.description")}</p>
            <span className="admin-module-card__action">{t("dashboard.cards.users.action")} &rarr;</span>
          </div>
        </NavLink>

        <NavLink to="/admin/settings" className="admin-module-card">
          <div className="admin-module-card__icon"><SettingsIcon /></div>
          <div className="admin-module-card__content">
            <h2>{t("dashboard.cards.settings.title")}</h2>
            <p>{t("dashboard.cards.settings.description")}</p>
            <span className="admin-module-card__action">{t("dashboard.cards.settings.action")} &rarr;</span>
          </div>
        </NavLink>

        <NavLink to="/admin/audit-logs" className="admin-module-card">
          <div className="admin-module-card__icon"><ShieldIcon /></div>
          <div className="admin-module-card__content">
            <h2>{t("dashboard.cards.audit.title")}</h2>
            <p>{t("dashboard.cards.audit.description")}</p>
            <span className="admin-module-card__action">{t("dashboard.cards.audit.action")} &rarr;</span>
          </div>
        </NavLink>

        <NavLink to="/admin/reports" className="admin-module-card">
          <div className="admin-module-card__icon"><FileIcon /></div>
          <div className="admin-module-card__content">
            <h2>{t("dashboard.cards.reports.title")}</h2>
            <p>{t("dashboard.cards.reports.description")}</p>
            <span className="admin-module-card__action">{t("dashboard.cards.reports.action")} &rarr;</span>
          </div>
        </NavLink>

        <NavLink to="/admin/ai-evidence" className="admin-module-card">
          <div className="admin-module-card__icon"><CpuIcon /></div>
          <div className="admin-module-card__content">
            <h2>{t("dashboard.cards.ai.title")}</h2>
            <p>{t("dashboard.cards.ai.description")}</p>
            <span className="admin-module-card__action">{t("dashboard.cards.ai.action")} &rarr;</span>
          </div>
        </NavLink>

        <NavLink to="/admin/alert-rules" className="admin-module-card">
          <div className="admin-module-card__icon"><BellIcon /></div>
          <div className="admin-module-card__content">
            <h2>{t("dashboard.cards.alertRules.title")}</h2>
            <p>{t("dashboard.cards.alertRules.description")}</p>
            <span className="admin-module-card__action">{t("dashboard.cards.alertRules.action")} &rarr;</span>
          </div>
        </NavLink>

        <NavLink to="/admin/road-segments" className="admin-module-card">
          <div className="admin-module-card__icon"><ActivityIcon /></div>
          <div className="admin-module-card__content">
            <h2>{t("dashboard.cards.roadSegments.title")}</h2>
            <p>{t("dashboard.cards.roadSegments.description")}</p>
            <span className="admin-module-card__action">{t("dashboard.cards.roadSegments.action")} &rarr;</span>
          </div>
        </NavLink>

        <NavLink to="/system-status" className="admin-module-card">
          <div className="admin-module-card__icon"><ActivityIcon /></div>
          <div className="admin-module-card__content">
            <h2>{t("dashboard.cards.system.title")}</h2>
            <p>{t("dashboard.cards.system.description")}</p>
            <span className="admin-module-card__action">{t("dashboard.cards.system.action")} &rarr;</span>
          </div>
        </NavLink>

        <button
          onClick={() => void logout()}
          className="admin-module-card"
          style={{ textAlign: 'left', cursor: 'pointer', outline: 'none' }}
        >
          <div className="admin-module-card__icon" style={{ color: 'var(--color-alert-danger)', background: 'var(--color-alert-danger-soft, #ffeaf0)' }}><LogoutIcon /></div>
          <div className="admin-module-card__content">
            <h2>{t("dashboard.cards.logout.title")}</h2>
            <p>{t("dashboard.cards.logout.description")}</p>
            <span className="admin-module-card__action" style={{ color: 'var(--color-alert-danger)' }}>{t("dashboard.cards.logout.action")} &rarr;</span>
          </div>
        </button>
      </div>
    </div>
  );
}
