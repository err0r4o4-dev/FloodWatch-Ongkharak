import { useEffect, useState } from "react";
import { NavLink, useSearchParams } from "react-router";
import { formatDate, formatNumber, useTranslation } from "../../i18n";
import th from "../../i18n/locales/th/admin";
import en from "../../i18n/locales/en/admin";
import { adminApi } from "./api";
import type { AuditLog, AuditAction, TargetType } from "./types";
import { adminErrorState, formatAdminDate, formatAdminNumber } from "./localization";
import type { AdminErrorState } from "./localization";

export function AuditLogsPage() {
  const { locale, t } = useTranslation({ th, en });
  const [searchParams, setSearchParams] = useSearchParams();
  const [logs, setLogs] = useState<AuditLog[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<AdminErrorState | null>(null);
  const [pagination, setPagination] = useState({ total: 0, limit: 50, offset: 0 });
  const [expandedLog, setExpandedLog] = useState<string | null>(null);

  // Filters from URL
  const actorIdParam = searchParams.get("actorId") || undefined;
  const [actionFilter, setActionFilter] = useState<AuditAction | "">("");
  const [targetTypeFilter, setTargetTypeFilter] = useState<TargetType | "">("");

  const loadLogs = async () => {
    try {
      setLoading(true);
      setError(null);
      const response = await adminApi.listAuditLogs({
        actorId: actorIdParam,
        action: actionFilter || undefined,
        targetType: targetTypeFilter || undefined,
        limit: pagination.limit,
        offset: pagination.offset,
      });
      setLogs(response.logs);
      setPagination(response.pagination);
    } catch (err) {
      setError(adminErrorState(err, "errors.loadAudit"));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void loadLogs();
  }, [actorIdParam, actionFilter, targetTypeFilter, pagination.offset]);

  const handleNextPage = () => {
    setPagination(prev => ({ ...prev, offset: prev.offset + prev.limit }));
  };

  const handlePrevPage = () => {
    setPagination(prev => ({ ...prev, offset: Math.max(0, prev.offset - prev.limit) }));
  };

  const getActionLabel = (action: AuditAction): string => {
    const keys: Record<AuditAction, keyof typeof th> = {
      "user.created": "audit.action.userCreated",
      "user.disabled": "audit.action.userDisabled",
      "user.enabled": "audit.action.userEnabled",
      "user.role_changed": "audit.action.userRoleChanged",
      "user.password_changed": "audit.action.userPasswordChanged",
      "alert_rule.created": "audit.action.alertRuleCreated",
      "alert_rule.updated": "audit.action.alertRuleUpdated",
      "alert_rule.deleted": "audit.action.alertRuleDeleted",
      "setting.updated": "audit.action.settingUpdated",
      "report.reviewed": "audit.action.reportReviewed",
      "manual_announcement.created": "audit.action.announcementCreated",
      "manual_announcement.updated": "audit.action.announcementUpdated",
      "manual_announcement.published": "audit.action.announcementPublished",
      "manual_announcement.withdrawn": "audit.action.announcementWithdrawn",
      "auth.login_success": "audit.action.loginSuccess",
      "auth.login_failed": "audit.action.loginFailed",
      "auth.logout": "audit.action.logout",
    };
    return t(keys[action]) || action;
  };

  const getTargetTypeLabel = (targetType: TargetType): string => {
    const keys: Record<TargetType, keyof typeof th> = {
      USER: "audit.target.user",
      ALERT_RULE: "audit.target.alertRule",
      DATA_SOURCE: "audit.target.dataSource",
      COMMUNITY_REPORT: "audit.target.communityReport",
      MANUAL_ANNOUNCEMENT: "audit.target.manualAnnouncement",
      SYSTEM_SETTING: "audit.target.systemSetting",
      AUTH_SESSION: "audit.target.authSession",
    };
    return t(keys[targetType]);
  };

  const getActorRoleBadge = (role: string) => {
    const badges = {
      ADMIN: { class: "admin-badge--admin", label: t("common.admin") },
      USER: { class: "admin-badge--user", label: t("common.user") },
      SYSTEM: { class: "admin-badge--disabled", label: t("common.system") },
    };
    const badge = badges[role as keyof typeof badges] || { class: "", label: role };
    return <span className={`admin-badge ${badge.class}`}>{badge.label}</span>;
  };

  return (
    <div className="page-container admin-page">
      <header className="page-header">
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
          <div>
            <h1>{t("audit.title")}</h1>
            <p>{t("audit.description")}</p>
            {actorIdParam && (
              <p className="muted small">
                {t("audit.actorFilter", { id: actorIdParam })}{" "}
                <button
                  onClick={() => {
                    searchParams.delete("actorId");
                    setSearchParams(searchParams);
                  }}
                  className="button button--small"
                  style={{ marginLeft: "var(--space-2)" }}
                >
                  {t("audit.clearFilter")}
                </button>
              </p>
            )}
          </div>
          <NavLink to="/admin" className="button">{t("common.back")}</NavLink>
        </div>
      </header>

      <div className="panel">
        <div className="admin-toolbar">
          <div className="admin-filter-group">
            <label htmlFor="action-filter">{t("audit.actionFilter")}</label>
            <select
              id="action-filter"
              value={actionFilter}
              onChange={(e) => setActionFilter(e.target.value as AuditAction | "")}
              className="admin-search"
              style={{ minWidth: "12rem" }}
            >
              <option value="">{t("common.all")}</option>
              <option value="user.created">{getActionLabel("user.created")}</option>
              <option value="user.disabled">{getActionLabel("user.disabled")}</option>
              <option value="user.enabled">{getActionLabel("user.enabled")}</option>
              <option value="user.role_changed">{getActionLabel("user.role_changed")}</option>
              <option value="setting.updated">{getActionLabel("setting.updated")}</option>
              <option value="manual_announcement.created">{getActionLabel("manual_announcement.created")}</option>
              <option value="manual_announcement.updated">{getActionLabel("manual_announcement.updated")}</option>
              <option value="manual_announcement.published">{getActionLabel("manual_announcement.published")}</option>
              <option value="manual_announcement.withdrawn">{getActionLabel("manual_announcement.withdrawn")}</option>
              <option value="auth.login_success">{getActionLabel("auth.login_success")}</option>
              <option value="auth.logout">{getActionLabel("auth.logout")}</option>
            </select>
          </div>

          <div className="admin-filter-group">
            <label htmlFor="target-filter">{t("audit.targetFilter")}</label>
            <select
              id="target-filter"
              value={targetTypeFilter}
              onChange={(e) => setTargetTypeFilter(e.target.value as TargetType | "")}
              className="admin-search"
              style={{ minWidth: "12rem" }}
            >
              <option value="">{t("common.all")}</option>
              <option value="USER">{getTargetTypeLabel("USER")}</option>
              <option value="SYSTEM_SETTING">{getTargetTypeLabel("SYSTEM_SETTING")}</option>
              <option value="ALERT_RULE">{getTargetTypeLabel("ALERT_RULE")}</option>
              <option value="COMMUNITY_REPORT">{getTargetTypeLabel("COMMUNITY_REPORT")}</option>
              <option value="MANUAL_ANNOUNCEMENT">{getTargetTypeLabel("MANUAL_ANNOUNCEMENT")}</option>
              <option value="AUTH_SESSION">{getTargetTypeLabel("AUTH_SESSION")}</option>
            </select>
          </div>
        </div>

        {loading && (
          <div className="loading-state" style={{ padding: "var(--space-8)" }}>
            <p>{t("audit.loading")}</p>
          </div>
        )}

        {error && (
          <div className="error-state" style={{ padding: "var(--space-8)" }}>
            <p>{t("common.errorPrefix")} {t(error.key, error.params)}</p>
            <button onClick={() => void loadLogs()} className="button">
              {t("common.retry")}
            </button>
          </div>
        )}

        {!loading && !error && logs.length === 0 && (
          <div className="empty-state" style={{ padding: "var(--space-8)", textAlign: "center" }}>
            <p>{t("common.noAuditLogs")}</p>
          </div>
        )}

        {!loading && !error && logs.length > 0 && (
          <>
            <div style={{ marginTop: "var(--space-4)", display: "grid", gap: "var(--space-3)" }}>
              {logs.map((log) => (
                <div
                  key={log.id}
                  className="panel"
                  style={{
                    padding: "var(--space-4)",
                    background: "var(--color-canvas-soft)",
                    cursor: "pointer",
                  }}
                  role="button"
                  tabIndex={0}
                  aria-expanded={expandedLog === log.id}
                  aria-label={`${getActionLabel(log.action)}: ${log.id}`}
                  onClick={() => setExpandedLog(expandedLog === log.id ? null : log.id)}
                  onKeyDown={(event) => {
                    if (event.key === "Enter" || event.key === " ") {
                      event.preventDefault();
                      setExpandedLog(expandedLog === log.id ? null : log.id);
                    }
                  }}
                >
                  <div style={{ display: "flex", justifyContent: "space-between", alignItems: "start", gap: "var(--space-4)" }}>
                    <div style={{ flex: 1 }}>
                      <div style={{ display: "flex", gap: "var(--space-3)", alignItems: "center", flexWrap: "wrap" }}>
                        <strong>{getActionLabel(log.action)}</strong>
                        {getActorRoleBadge(log.actorRole)}
                        <span className="admin-badge" style={{ fontSize: "0.75rem" }}>
                          {getTargetTypeLabel(log.targetType)}
                        </span>
                      </div>
                      <p className="muted small" style={{ marginTop: "var(--space-2)" }}>
                        {t("audit.byTime", { email: log.actorEmail, time: formatAdminDate(log.createdAt, locale, formatDate, { year: "numeric", month: "short", day: "numeric", hour: "2-digit", minute: "2-digit", second: "2-digit" }) })}
                      </p>
                    </div>
                    <span aria-hidden="true">{expandedLog === log.id ? "▼" : "▶"}</span>
                  </div>

                  {expandedLog === log.id && (
                    <div style={{ marginTop: "var(--space-4)", paddingTop: "var(--space-4)", borderTop: "1px solid var(--color-border)" }}>
                      <dl style={{ display: "grid", gap: "var(--space-3)", fontSize: "0.875rem" }}>
                        <div>
                          <dt className="muted">{t("audit.id")}</dt>
                          <dd style={{ fontFamily: "monospace" }}>{log.id}</dd>
                        </div>
                        {log.actorId && (
                          <div>
                            <dt className="muted">{t("audit.actorId")}</dt>
                            <dd style={{ fontFamily: "monospace" }}>{log.actorId}</dd>
                          </div>
                        )}
                        <div>
                          <dt className="muted">{t("audit.targetId")}</dt>
                          <dd style={{ fontFamily: "monospace" }}>{log.targetId}</dd>
                        </div>
                        {log.ipAddress && (
                          <div>
                          <dt className="muted">{t("audit.ipAddress")}</dt>
                            <dd style={{ fontFamily: "monospace" }}>{log.ipAddress}</dd>
                          </div>
                        )}
                        {log.userAgent && (
                          <div>
                          <dt className="muted">{t("audit.userAgent")}</dt>
                            <dd style={{ fontFamily: "monospace", fontSize: "0.75rem", wordBreak: "break-all" }}>
                              {log.userAgent}
                            </dd>
                          </div>
                        )}
                        {Object.keys(log.metadata).length > 0 && (
                          <div>
                            <dt className="muted">{t("audit.metadata")}</dt>
                            <dd>
                              <pre style={{ 
                                background: "var(--color-surface)", 
                                padding: "var(--space-3)", 
                                borderRadius: "var(--radius-sm)", 
                                overflow: "auto",
                                fontSize: "0.75rem"
                              }}>
                                {JSON.stringify(log.metadata, null, 2)}
                              </pre>
                            </dd>
                          </div>
                        )}
                      </dl>
                    </div>
                  )}
                </div>
              ))}
            </div>

            <div className="admin-pagination">
              <div>
                {t("common.showing", {
                  from: formatAdminNumber(pagination.offset + 1, locale, formatNumber),
                  to: formatAdminNumber(Math.min(pagination.offset + pagination.limit, pagination.total), locale, formatNumber),
                  total: formatAdminNumber(pagination.total, locale, formatNumber),
                })}
              </div>
              <div className="admin-actions">
                <button
                  onClick={handlePrevPage}
                  disabled={pagination.offset === 0}
                  className="button"
                >
                  {t("common.previous")}
                </button>
                <button
                  onClick={handleNextPage}
                  disabled={pagination.offset + pagination.limit >= pagination.total}
                  className="button"
                >
                  {t("common.next")}
                </button>
              </div>
            </div>
          </>
        )}
      </div>
    </div>
  );
}
