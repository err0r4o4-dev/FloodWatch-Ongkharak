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

export function AdminDashboardPage() {
  const { account } = useAuth();
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
      <header className="page-header">
        <h1>{t("dashboard.title")}</h1>
        <p>{t("dashboard.description")}</p>
      </header>

      <section className="panel admin-stats-panel" aria-labelledby="admin-stats-heading" aria-live="polite">
        <h2 id="admin-stats-heading">{t("dashboard.stats.title")}</h2>
        {statsLoading && <p role="status">{t("dashboard.stats.loading")}</p>}
        {statsError && (
          <div className="error-state" role="alert">
            <p>{t("dashboard.stats.error")}</p>
            <p className="muted">{t(statsError.key, statsError.params)}</p>
            <button type="button" className="action-button" onClick={() => setRetry((current) => current + 1)}>{t("dashboard.stats.retry")}</button>
          </div>
        )}
        {stats && (
          <dl className="admin-dashboard-grid">
            <div className="admin-card">
              <dt>{t("dashboard.stats.totalUsers")}</dt>
              <dd>{formatAdminNumber(stats.totalUsers, locale, formatNumber)}</dd>
            </div>
            <div className="admin-card">
              <dt>{t("dashboard.stats.activeUsers")}</dt>
              <dd>{formatAdminNumber(stats.activeUsers, locale, formatNumber)}</dd>
            </div>
            <div className="admin-card">
              <dt>{t("dashboard.stats.totalReports")}</dt>
              <dd>{formatAdminNumber(stats.totalReports, locale, formatNumber)}</dd>
            </div>
            <div className="admin-card">
              <dt>{t("dashboard.stats.pendingReports")}</dt>
              <dd>{formatAdminNumber(stats.pendingReports, locale, formatNumber)}</dd>
            </div>
            <div className="admin-card">
              <dt>{t("dashboard.stats.recentActivity")}</dt>
              <dd>{formatAdminNumber(stats.recentActivity, locale, formatNumber)}</dd>
            </div>
          </dl>
        )}
      </section>

      <div className="admin-dashboard-grid">
        <section className="panel admin-card">
          <h2>{t("dashboard.cards.users.title")}</h2>
          <p className="muted">{t("dashboard.cards.users.description")}</p>
          <NavLink to="/admin/users" className="button button--primary">
            {t("dashboard.cards.users.action")}
          </NavLink>
        </section>

        <section className="panel admin-card">
          <h2>{t("dashboard.cards.settings.title")}</h2>
          <p className="muted">{t("dashboard.cards.settings.description")}</p>
          <NavLink to="/admin/settings" className="button button--primary">
            {t("dashboard.cards.settings.action")}
          </NavLink>
        </section>

        <section className="panel admin-card">
          <h2>{t("dashboard.cards.audit.title")}</h2>
          <p className="muted">{t("dashboard.cards.audit.description")}</p>
          <NavLink to="/admin/audit-logs" className="button button--primary">
            {t("dashboard.cards.audit.action")}
          </NavLink>
        </section>

        <section className="panel admin-card">
          <h2>{t("dashboard.cards.reports.title")}</h2>
          <p className="muted">{t("dashboard.cards.reports.description")}</p>
          <NavLink to="/admin/reports" className="button button--primary">
            {t("dashboard.cards.reports.action")}
          </NavLink>
        </section>

        <section className="panel admin-card">
          <h2>{t("dashboard.cards.ai.title")}</h2>
          <p className="muted">{t("dashboard.cards.ai.description")}</p>
          <NavLink to="/admin/ai-evidence" className="button button--primary">
            {t("dashboard.cards.ai.action")}
          </NavLink>
        </section>
        <section className="panel admin-card">
          <h2>{t("dashboard.cards.alertRules.title")}</h2>
          <p className="muted">{t("dashboard.cards.alertRules.description")}</p>
          <NavLink to="/admin/alert-rules" className="button button--primary">
            {t("dashboard.cards.alertRules.action")}
          </NavLink>
        </section>

        <section className="panel admin-card">
          <h2>{t("dashboard.cards.system.title")}</h2>
          <p className="muted">{t("dashboard.cards.system.description")}</p>
          <NavLink to="/system-status" className="button button--primary">
            {t("dashboard.cards.system.action")}
          </NavLink>
        </section>
      </div>

      <aside className="admin-info-panel panel">
        <h3>{t("dashboard.important.title")}</h3>
        <ul>
          <li><strong>{t("common.account")}</strong> {account.email}</li>
          <li><strong>{t("common.role")}</strong> {t("dashboard.important.role")}</li>
        </ul>
        <p className="muted small">
          {t("dashboard.important.note")}
        </p>
      </aside>
    </div>
  );
}
