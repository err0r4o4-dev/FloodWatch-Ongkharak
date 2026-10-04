import { useEffect, useState } from "react";
import { NavLink } from "react-router";
import { formatDate, useTranslation } from "../../i18n";
import th from "../../i18n/locales/th/admin";
import en from "../../i18n/locales/en/admin";
import { adminApi } from "./api";
import type { SystemSetting, SettingCategory } from "./types";
import { notifyError, notifySuccess } from "../../shared/alerts";
import { adminErrorState, formatAdminDate } from "./localization";
import type { AdminErrorState } from "./localization";

export function SettingsPage() {
  const { locale, t } = useTranslation({ th, en });
  const [settings, setSettings] = useState<SystemSetting[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<AdminErrorState | null>(null);
  const [categoryFilter, setCategoryFilter] = useState<SettingCategory | "">("");
  const [editingKey, setEditingKey] = useState<string | null>(null);
  const [editValue, setEditValue] = useState<string>("");
  const [saveLoading, setSaveLoading] = useState(false);
  const [saveError, setSaveError] = useState<AdminErrorState | null>(null);

  const loadSettings = async () => {
    try {
      setLoading(true);
      setError(null);
      const response = await adminApi.listSettings({
        category: categoryFilter || undefined,
      });
      setSettings(response.settings);
    } catch (err) {
      setError(adminErrorState(err, "errors.loadSettings"));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void loadSettings();
  }, [categoryFilter]);

  const handleEdit = (setting: SystemSetting) => {
    setEditingKey(setting.key);
    setEditValue(JSON.stringify(setting.value));
    setSaveError(null);
  };

  const handleCancel = () => {
    setEditingKey(null);
    setEditValue("");
    setSaveError(null);
  };

  const handleSave = async (key: string) => {
    if (saveLoading) return;
    try {
      setSaveLoading(true);
      setSaveError(null);
      const parsedValue = JSON.parse(editValue);
      await adminApi.updateSetting(key, parsedValue);
      await loadSettings();
      setEditingKey(null);
      setEditValue("");
      void notifySuccess(t("settings.save.success"), t("settings.save.successText", { key }));
    } catch (err) {
      const message = err instanceof SyntaxError ? { key: "settings.save.invalidJson" as const } : adminErrorState(err, "errors.saveSetting");
      setSaveError(message);
      if (!(err instanceof SyntaxError)) void notifyError(t("settings.save.error"), t(message.key, message.params));
    } finally {
      setSaveLoading(false);
    }
  };

  const getCategoryLabel = (category: SettingCategory): string => {
    const keys = {
      rate_limit: "settings.category.rateLimit",
      alert: "settings.category.alert",
      ingestion: "settings.category.ingestion",
      feature: "settings.category.feature",
      security: "settings.category.security",
    } as const;
    return t(keys[category]);
  };

  const groupedSettings = settings.reduce((acc, setting) => {
    if (!acc[setting.category]) {
      acc[setting.category] = [];
    }
    acc[setting.category].push(setting);
    return acc;
  }, {} as Record<SettingCategory, SystemSetting[]>);

  return (
    <div className="page-container admin-page">
      <header className="page-header">
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
          <div>
            <h1>{t("settings.title")}</h1>
            <p>{t("settings.description")}</p>
          </div>
          <NavLink to="/admin" className="button">{t("common.back")}</NavLink>
        </div>
      </header>

      <div className="panel">
        <div className="admin-toolbar">
          <div className="admin-filter-group">
            <label htmlFor="category-filter">{t("settings.filter.category")}</label>
            <select
              id="category-filter"
              value={categoryFilter}
              onChange={(e) => setCategoryFilter(e.target.value as SettingCategory | "")}
              className="admin-search"
              style={{ minWidth: "12rem" }}
            >
              <option value="">{t("common.all")}</option>
              <option value="rate_limit">{t("settings.category.rateLimit")}</option>
              <option value="alert">{t("settings.category.alert")}</option>
              <option value="ingestion">{t("settings.category.ingestion")}</option>
              <option value="feature">{t("settings.category.feature")}</option>
              <option value="security">{t("settings.category.security")}</option>
            </select>
          </div>
        </div>

        {loading && (
          <div className="loading-state" style={{ padding: "var(--space-8)" }}>
            <p>{t("settings.loading")}</p>
          </div>
        )}

        {error && (
          <div className="error-state" role="alert" style={{ padding: "var(--space-8)" }}>
            <p>{t("common.errorPrefix")} {t(error.key, error.params)}</p>
            <button onClick={() => void loadSettings()} className="button">
              {t("common.retry")}
            </button>
          </div>
        )}

        {!loading && !error && settings.length === 0 && (
          <div className="empty-state" style={{ padding: "var(--space-8)", textAlign: "center" }}>
            <p>{t("common.noSettings")}</p>
          </div>
        )}

        {!loading && !error && Object.keys(groupedSettings).length > 0 && (
          <div style={{ display: "grid", gap: "var(--space-6)", marginTop: "var(--space-6)" }}>
            {(Object.keys(groupedSettings) as SettingCategory[]).map((category) => (
              <section key={category} className="panel" style={{ background: "var(--color-canvas-soft)" }}>
                <h2>{getCategoryLabel(category)}</h2>
                <div style={{ marginTop: "var(--space-4)", display: "grid", gap: "var(--space-4)" }}>
                  {groupedSettings[category].map((setting) => (
                    <div
                      key={setting.key}
                      className="panel"
                      style={{
                        padding: "var(--space-4)",
                        background: "var(--color-surface)",
                      }}
                    >
                      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "start", gap: "var(--space-4)" }}>
                        <div style={{ flex: 1 }}>
                          <h3 style={{ fontSize: "1rem", margin: 0, fontFamily: "monospace" }}>
                            {setting.key}
                          </h3>
                          <p className="muted small" style={{ marginTop: "var(--space-2)" }}>
                            {setting.description}
                          </p>

                          {editingKey === setting.key ? (
                            <div style={{ marginTop: "var(--space-3)" }}>
                              <input
                                type="text"
                                value={editValue}
                                onChange={(e) => setEditValue(e.target.value)}
                                className="admin-search"
                                style={{ width: "100%", fontFamily: "monospace" }}
                                placeholder={t("settings.json.placeholder")}
                              />
                              {saveError && (
                                <p className="muted small" style={{ color: "var(--color-danger-ink)", marginTop: "var(--space-2)" }}>
                                  {t(saveError.key, saveError.params)}
                                </p>
                              )}
                              <div className="admin-actions" style={{ marginTop: "var(--space-3)" }}>
                                <button
                                  onClick={() => void handleSave(setting.key)}
                                  disabled={saveLoading}
                                  className="button button--primary"
                                >
                                  {t("common.save")}
                                </button>
                                <button
                                  onClick={handleCancel}
                                  disabled={saveLoading}
                                  className="button"
                                >
                                  {t("common.cancel")}
                                </button>
                              </div>
                            </div>
                          ) : (
                            <div style={{ marginTop: "var(--space-3)" }}>
                              <div style={{ display: "flex", gap: "var(--space-3)", alignItems: "center" }}>
                                <span style={{ fontFamily: "monospace", fontWeight: 600 }}>
                                  {JSON.stringify(setting.value)}
                                </span>
                                <span className="admin-badge" style={{ fontSize: "0.75rem" }}>
                                  {setting.valueType}
                                </span>
                              </div>
                              <p className="muted small" style={{ marginTop: "var(--space-2)" }}>
                                {t("settings.updatedAt")} {formatAdminDate(setting.updatedAt, locale, formatDate, { year: "numeric", month: "short", day: "numeric", hour: "2-digit", minute: "2-digit" })}
                                {setting.updatedBy && ` ${t("settings.updatedBy", { name: setting.updatedBy })}`}
                              </p>
                            </div>
                          )}
                        </div>

                        {editingKey !== setting.key && (
                          <div>
                            {setting.editable ? (
                              <button
                                onClick={() => handleEdit(setting)}
                                className="button button--small"
                              >
                                {t("common.edit")}
                              </button>
                            ) : (
                              <span className="admin-badge admin-badge--disabled">
                                {t("common.readOnly")}
                              </span>
                            )}
                          </div>
                        )}
                      </div>
                    </div>
                  ))}
                </div>
              </section>
            ))}
          </div>
        )}
      </div>

      <aside className="admin-info-panel panel" style={{ marginTop: "var(--space-6)" }}>
        <h3>{t("settings.warning.title")}</h3>
        <ul>
          <li>{t("settings.warning.immediate")}</li>
          <li>{t("settings.warning.audit")}</li>
          <li>{t("settings.warning.json")}</li>
          <li>{t("settings.warning.readOnly")}</li>
        </ul>
      </aside>
    </div>
  );
}
