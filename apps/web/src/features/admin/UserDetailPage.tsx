import { useEffect, useRef, useState } from "react";
import { NavLink, useParams } from "react-router";
import { formatDate, formatNumber, useTranslation } from "../../i18n";
import th from "../../i18n/locales/th/admin";
import en from "../../i18n/locales/en/admin";
import { adminApi } from "./api";
import type { UserManagement, UserStats } from "./types";
import { confirmAction } from "../../shared/alerts";
import { adminErrorMessage, adminErrorState, formatAdminDate, formatAdminNumber } from "./localization";
import type { AdminErrorState } from "./localization";

export function UserDetailPage() {
  const { locale, t } = useTranslation({ th, en });
  const { id } = useParams<{ id: string }>();
  const [user, setUser] = useState<UserManagement | null>(null);
  const [stats, setStats] = useState<UserStats | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<AdminErrorState | null>(null);
  const [actionLoading, setActionLoading] = useState(false);
  const [actionError, setActionError] = useState<AdminErrorState | null>(null);
  const actionPending = useRef(false);

  const loadUser = async () => {
    if (!id) return;
    
    try {
      setLoading(true);
      setError(null);
      const response = await adminApi.getUser(id);
      setUser(response.user);
      setStats(response.stats);
    } catch (err) {
      setError(adminErrorState(err, "errors.loadUser"));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void loadUser();
  }, [id]);

  const manageUser = async (options: Parameters<typeof confirmAction>[0]) => {
    if (actionPending.current) return;
    actionPending.current = true;
    setActionLoading(true);
    setActionError(null);
    try {
      await confirmAction({ ...options, errorText: (cause) => {
        const message = adminErrorMessage(cause, t, "errors.actionFailed");
        setActionError(adminErrorState(cause, "errors.actionFailed"));
        return message;
      } });
    } finally {
      actionPending.current = false;
      setActionLoading(false);
    }
  };

  const handleUpdateRole = async (newRole: string) => {
    if (!id || !user) return;
    await manageUser({
      title: t("userDetail.confirm.role.title"),
      text: t("userDetail.confirm.role.text", { email: user.email, role: newRole === "ADMIN" ? t("common.admin") : t("common.user") }),
      confirmText: t("userDetail.confirm.role.confirm"),
      danger: true,
      action: async () => { await adminApi.updateUserRole(id, newRole); await loadUser(); },
      successTitle: t("userDetail.confirm.role.success")
    });
  };

  const handleDisable = async () => {
    if (!id || !user) return;
    await manageUser({
      title: t("userDetail.confirm.disable.title"),
      text: t("userDetail.confirm.disable.text", { email: user.email }),
      confirmText: t("userDetail.confirm.disable.confirm"),
      danger: true,
      action: async () => { await adminApi.disableUser(id); await loadUser(); },
      successTitle: t("userDetail.confirm.disable.success")
    });
  };

  const handleEnable = async () => {
    if (!id || !user) return;
    await manageUser({
      title: t("userDetail.confirm.enable.title"),
      text: t("userDetail.confirm.enable.text", { email: user.email }),
      confirmText: t("userDetail.confirm.enable.confirm"),
      action: async () => { await adminApi.enableUser(id); await loadUser(); },
      successTitle: t("userDetail.confirm.enable.success")
    });
  };

  if (loading) {
    return (
      <div className="page-container admin-page">
        <div className="panel loading-state">
          <p>{t("userDetail.loading")}</p>
        </div>
      </div>
    );
  }

  if (error || !user || !stats) {
    return (
      <div className="page-container admin-page">
        <div className="panel error-state">
          <h1>{t("common.error")}</h1>
          <p>{error ? t(error.key, error.params) : t("userDetail.notFound")}</p>
          <NavLink to="/admin/users" className="button">{t("common.back")}</NavLink>
        </div>
      </div>
    );
  }

  return (
    <div className="page-container admin-page admin-user-detail">
      <header className="page-header">
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", flexWrap: "wrap", gap: "var(--space-3)" }}>
          <div style={{ minWidth: 0, flex: "1 1 15rem" }}>
            <h1>{t("userDetail.title")}</h1>
            <p className="muted">{user.email}</p>
          </div>
          <NavLink to="/admin/users" className="button">{t("common.back")}</NavLink>
        </div>
      </header>

      {actionError && (
        <div className="panel error-state" style={{ marginBottom: "var(--space-6)" }}>
          <p>{t("common.errorPrefix")} {t(actionError.key, actionError.params)}</p>
        </div>
      )}

      <div style={{ display: "grid", gap: "var(--space-6)", gridTemplateColumns: "repeat(auto-fit, minmax(min(20rem, 100%), 1fr))" }}>
        <section className="panel">
          <h2>{t("userDetail.account.title")}</h2>
          <dl style={{ display: "grid", gap: "var(--space-3)", marginTop: "var(--space-4)" }}>
            <div>
              <dt className="muted small">ID</dt>
              <dd style={{ fontFamily: "monospace", fontSize: "0.875rem" }}>{user.id}</dd>
            </div>
            <div>
              <dt className="muted small">{t("common.email")}</dt>
              <dd><strong>{user.email}</strong></dd>
            </div>
            <div>
              <dt className="muted small">{t("common.role")}</dt>
              <dd>
                <span className={`admin-badge admin-badge--${user.role.toLowerCase()}`}>
                  {user.role === "ADMIN" ? t("common.admin") : t("common.user")}
                </span>
              </dd>
            </div>
            <div>
              <dt className="muted small">{t("common.status")}</dt>
              <dd>
                {user.disabledAt ? (
                  <span className="admin-badge admin-badge--disabled">{t("common.disabled")}</span>
                ) : (
                  <span className="admin-badge admin-badge--active">{t("common.active")}</span>
                )}
              </dd>
            </div>
            <div>
              <dt className="muted small">{t("common.createdAt")}</dt>
              <dd>{formatAdminDate(user.createdAt, locale, formatDate, { year: "numeric", month: "long", day: "numeric", hour: "2-digit", minute: "2-digit" })}</dd>
            </div>
            <div>
              <dt className="muted small">{t("common.updatedAt")}</dt>
              <dd>{formatAdminDate(user.updatedAt, locale, formatDate, { year: "numeric", month: "long", day: "numeric", hour: "2-digit", minute: "2-digit" })}</dd>
            </div>
            {user.disabledAt && (
              <div>
                <dt className="muted small">{t("userDetail.disabledAt")}</dt>
                <dd>{formatAdminDate(user.disabledAt, locale, formatDate, { year: "numeric", month: "long", day: "numeric", hour: "2-digit", minute: "2-digit" })}</dd>
              </div>
            )}
          </dl>
        </section>

        <section className="panel">
          <h2>{t("userDetail.activity.title")}</h2>
          <dl style={{ display: "grid", gap: "var(--space-3)", marginTop: "var(--space-4)" }}>
            <div>
              <dt className="muted small">{t("userDetail.activity.totalReports")}</dt>
              <dd><strong>{formatAdminNumber(stats.reportCount, locale, formatNumber)}</strong> {t("common.reports")}</dd>
            </div>
            <div>
              <dt className="muted small">{t("userDetail.activity.verifiedReports")}</dt>
              <dd><strong>{formatAdminNumber(stats.verifiedReports, locale, formatNumber)}</strong> {t("common.reports")}</dd>
            </div>
            <div>
              <dt className="muted small">{t("userDetail.activity.rejectedReports")}</dt>
              <dd><strong>{formatAdminNumber(stats.rejectedReports, locale, formatNumber)}</strong> {t("common.reports")}</dd>
            </div>
            <div>
              <dt className="muted small">{t("userDetail.activity.auditLogs")}</dt>
              <dd><strong>{formatAdminNumber(stats.auditLogCount, locale, formatNumber)}</strong> {t("common.records")}</dd>
            </div>
            <div>
              <dt className="muted small">{t("userDetail.activity.sessions")}</dt>
              <dd><strong>{formatAdminNumber(stats.refreshSessionCount, locale, formatNumber)}</strong> {t("common.sessions")}</dd>
            </div>
            <div>
              <dt className="muted small">{t("common.latestActivity")}</dt>
              <dd>{formatAdminDate(stats.lastActivity, locale, formatDate, { year: "numeric", month: "long", day: "numeric", hour: "2-digit", minute: "2-digit" })}</dd>
            </div>
          </dl>
        </section>
      </div>

      <section className="panel" style={{ marginTop: "var(--space-6)" }}>
        <h2>{t("userDetail.actions.title")}</h2>
        <p className="muted">{t("userDetail.actions.note")}</p>

        <div style={{ display: "flex", gap: "var(--space-4)", marginTop: "var(--space-4)", flexWrap: "wrap" }}>
          <div>
            <h3 style={{ fontSize: "1rem", marginBottom: "var(--space-2)" }}>{t("userDetail.actions.role.title")}</h3>
            <div className="admin-actions">
              <button
                onClick={() => void handleUpdateRole("USER")}
                disabled={actionLoading || user.role === "USER"}
                className="button"
              >
                {t("userDetail.actions.role.user")}
              </button>
              <button
                onClick={() => void handleUpdateRole("ADMIN")}
                disabled={actionLoading || user.role === "ADMIN"}
                className="button"
              >
                {t("userDetail.actions.role.admin")}
              </button>
            </div>
          </div>

          <div>
            <h3 style={{ fontSize: "1rem", marginBottom: "var(--space-2)" }}>{t("userDetail.actions.account.title")}</h3>
            <div className="admin-actions">
              {user.disabledAt ? (
                <button
                  onClick={() => void handleEnable()}
                  disabled={actionLoading}
                  className="button button--primary"
                >
                  {t("userDetail.actions.account.enable")}
                </button>
              ) : (
                <button
                  onClick={() => void handleDisable()}
                  disabled={actionLoading}
                  className="button"
                  style={{ background: "var(--color-danger-soft)", color: "var(--color-danger-ink)" }}
                >
                  {t("userDetail.actions.account.disable")}
                </button>
              )}
            </div>
          </div>
        </div>

        {actionLoading && (
          <p className="muted" style={{ marginTop: "var(--space-4)" }}>{t("userDetail.actions.inProgress")}</p>
        )}
      </section>

      <section className="panel" style={{ marginTop: "var(--space-6)", background: "var(--color-canvas-soft)" }}>
        <h3>{t("userDetail.more.title")}</h3>
        <div className="admin-actions" style={{ marginTop: "var(--space-3)" }}>
          <NavLink
            to={`/admin/audit-logs?actorId=${user.id}`}
            className="button"
          >
            {t("userDetail.more.audit")}
          </NavLink>
        </div>
      </section>
    </div>
  );
}
