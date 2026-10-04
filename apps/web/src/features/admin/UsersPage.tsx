import { useEffect, useState } from "react";
import { NavLink } from "react-router";
import { formatDate, formatNumber, useTranslation } from "../../i18n";
import th from "../../i18n/locales/th/admin";
import en from "../../i18n/locales/en/admin";
import { adminApi } from "./api";
import type { UserManagement } from "./types";
import { adminErrorState, formatAdminDate, formatAdminNumber } from "./localization";
import type { AdminErrorState } from "./localization";

export function UsersPage() {
  const { locale, t } = useTranslation({ th, en });
  const [users, setUsers] = useState<UserManagement[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<AdminErrorState | null>(null);
  const [searchTerm, setSearchTerm] = useState("");
  const [roleFilter, setRoleFilter] = useState<string>("");
  const [includeDisabled, setIncludeDisabled] = useState(false);
  const [pagination, setPagination] = useState({ total: 0, limit: 50, offset: 0 });

  const loadUsers = async () => {
    try {
      setLoading(true);
      setError(null);
      const response = await adminApi.listUsers({
        role: roleFilter || undefined,
        includeDisabled,
        search: searchTerm || undefined,
        limit: pagination.limit,
        offset: pagination.offset,
      });
      setUsers(response.users);
      setPagination(response.pagination);
    } catch (err) {
      setError(adminErrorState(err, "errors.loadUsers"));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void loadUsers();
  }, [roleFilter, includeDisabled, pagination.offset]);

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault();
    setPagination(prev => ({ ...prev, offset: 0 }));
    void loadUsers();
  };

  const handleNextPage = () => {
    setPagination(prev => ({ ...prev, offset: prev.offset + prev.limit }));
  };

  const handlePrevPage = () => {
    setPagination(prev => ({ ...prev, offset: Math.max(0, prev.offset - prev.limit) }));
  };

  return (
    <div className="page-container admin-page">
      <header className="page-header">
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
          <div>
            <h1>{t("users.title")}</h1>
            <p>{t("users.description")}</p>
          </div>
          <NavLink to="/admin" className="button">{t("common.back")}</NavLink>
        </div>
      </header>

      <div className="panel">
        <form onSubmit={handleSearch} className="admin-toolbar">
          <input
            type="search"
            placeholder={t("users.search.placeholder")}
            aria-label={t("users.search.placeholder")}
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            className="admin-search"
          />
          
          <div className="admin-filter-group">
            <label htmlFor="role-filter">{t("users.filter.role")}</label>
            <select
              id="role-filter"
              value={roleFilter}
              onChange={(e) => setRoleFilter(e.target.value)}
              className="admin-search"
              style={{ minWidth: "8rem" }}
            >
              <option value="">{t("common.all")}</option>
              <option value="USER">{t("users.filter.user")}</option>
              <option value="ADMIN">{t("users.filter.admin")}</option>
            </select>
          </div>

          <label style={{ display: "flex", alignItems: "center", gap: "0.5rem" }}>
            <input
              type="checkbox"
              checked={includeDisabled}
              onChange={(e) => setIncludeDisabled(e.target.checked)}
            />
            <span>{t("users.filter.includeDisabled")}</span>
          </label>

          <button type="submit" className="button button--primary">
            {t("common.search")}
          </button>
        </form>

        {loading && (
          <div className="loading-state" style={{ padding: "var(--space-8)" }}>
            <p>{t("users.loading")}</p>
          </div>
        )}

        {error && (
          <div className="error-state" style={{ padding: "var(--space-8)" }}>
            <p>{t("common.errorPrefix")} {t(error.key, error.params)}</p>
            <button onClick={() => void loadUsers()} className="button">
              {t("common.retry")}
            </button>
          </div>
        )}

        {!loading && !error && users.length === 0 && (
          <div className="empty-state" style={{ padding: "var(--space-8)", textAlign: "center" }}>
            <p>{t("common.noUsers")}</p>
          </div>
        )}

        {!loading && !error && users.length > 0 && (
          <>
            <div style={{ overflowX: "auto" }}>
              <table className="admin-table">
                <thead>
                  <tr>
                    <th>{t("common.email")}</th>
                    <th>{t("users.table.role")}</th>
                    <th>{t("users.table.status")}</th>
                    <th>{t("users.table.createdAt")}</th>
                    <th>{t("users.table.actions")}</th>
                  </tr>
                </thead>
                <tbody>
                  {users.map((user) => (
                    <tr key={user.id}>
                      <td>
                        <strong>{user.email}</strong>
                      </td>
                      <td>
                        <span className={`admin-badge admin-badge--${user.role.toLowerCase()}`}>
                          {user.role === "ADMIN" ? t("common.admin") : t("common.user")}
                        </span>
                      </td>
                      <td>
                        {user.disabledAt ? (
                          <span className="admin-badge admin-badge--disabled">{t("common.disabled")}</span>
                        ) : (
                          <span className="admin-badge admin-badge--active">{t("common.active")}</span>
                        )}
                      </td>
                      <td className="muted small">{formatAdminDate(user.createdAt, locale, formatDate, { year: "numeric", month: "short", day: "numeric", hour: "2-digit", minute: "2-digit" })}</td>
                      <td>
                        <div className="admin-actions">
                          <NavLink
                            to={`/admin/users/${user.id}`}
                            className="button button--small"
                          >
                            {t("common.details")}
                          </NavLink>
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
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
