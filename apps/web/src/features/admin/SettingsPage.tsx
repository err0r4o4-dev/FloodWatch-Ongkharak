import { useEffect, useState } from "react";
import { NavLink } from "react-router";
import { adminApi } from "./api";
import type { SystemSetting, SettingCategory } from "./types";

export function SettingsPage() {
  const [settings, setSettings] = useState<SystemSetting[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [categoryFilter, setCategoryFilter] = useState<SettingCategory | "">("");
  const [editingKey, setEditingKey] = useState<string | null>(null);
  const [editValue, setEditValue] = useState<string>("");
  const [saveLoading, setSaveLoading] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);

  const loadSettings = async () => {
    try {
      setLoading(true);
      setError(null);
      const response = await adminApi.listSettings({
        category: categoryFilter || undefined,
      });
      setSettings(response.settings);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load settings");
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
    try {
      setSaveLoading(true);
      setSaveError(null);
      const parsedValue = JSON.parse(editValue);
      await adminApi.updateSetting(key, parsedValue);
      await loadSettings();
      setEditingKey(null);
      setEditValue("");
    } catch (err) {
      setSaveError(err instanceof Error ? err.message : "Failed to save setting");
    } finally {
      setSaveLoading(false);
    }
  };

  const getCategoryLabel = (category: SettingCategory): string => {
    const labels: Record<SettingCategory, string> = {
      rate_limit: "การจำกัดอัตรา",
      alert: "การแจ้งเตือน",
      ingestion: "การนำเข้าข้อมูล",
      feature: "ฟีเจอร์",
      security: "ความปลอดภัย",
    };
    return labels[category];
  };

  const formatDate = (dateString: string) => {
    return new Date(dateString).toLocaleDateString("th-TH", {
      year: "numeric",
      month: "short",
      day: "numeric",
      hour: "2-digit",
      minute: "2-digit",
    });
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
            <h1>ตั้งค่าระบบ</h1>
            <p>กำหนดค่าการทำงานของระบบและฟีเจอร์ต่างๆ</p>
          </div>
          <NavLink to="/admin" className="button">← กลับ</NavLink>
        </div>
      </header>

      <div className="panel">
        <div className="admin-toolbar">
          <div className="admin-filter-group">
            <label htmlFor="category-filter">หมวดหมู่:</label>
            <select
              id="category-filter"
              value={categoryFilter}
              onChange={(e) => setCategoryFilter(e.target.value as SettingCategory | "")}
              className="admin-search"
              style={{ minWidth: "12rem" }}
            >
              <option value="">ทั้งหมด</option>
              <option value="rate_limit">การจำกัดอัตรา</option>
              <option value="alert">การแจ้งเตือน</option>
              <option value="ingestion">การนำเข้าข้อมูล</option>
              <option value="feature">ฟีเจอร์</option>
              <option value="security">ความปลอดภัย</option>
            </select>
          </div>
        </div>

        {loading && (
          <div className="loading-state" style={{ padding: "var(--space-8)" }}>
            <p>กำลังโหลดการตั้งค่า...</p>
          </div>
        )}

        {error && (
          <div className="error-state" style={{ padding: "var(--space-8)" }}>
            <p>เกิดข้อผิดพลาด: {error}</p>
            <button onClick={() => void loadSettings()} className="button">
              ลองอีกครั้ง
            </button>
          </div>
        )}

        {!loading && !error && settings.length === 0 && (
          <div className="empty-state" style={{ padding: "var(--space-8)", textAlign: "center" }}>
            <p>ไม่พบการตั้งค่า</p>
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
                                placeholder="ใส่ค่าในรูปแบบ JSON"
                              />
                              {saveError && (
                                <p className="muted small" style={{ color: "var(--color-danger-ink)", marginTop: "var(--space-2)" }}>
                                  {saveError}
                                </p>
                              )}
                              <div className="admin-actions" style={{ marginTop: "var(--space-3)" }}>
                                <button
                                  onClick={() => void handleSave(setting.key)}
                                  disabled={saveLoading}
                                  className="button button--primary"
                                >
                                  บันทึก
                                </button>
                                <button
                                  onClick={handleCancel}
                                  disabled={saveLoading}
                                  className="button"
                                >
                                  ยกเลิก
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
                                อัปเดตล่าสุด: {formatDate(setting.updatedAt)}
                                {setting.updatedBy && ` โดย ${setting.updatedBy}`}
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
                                แก้ไข
                              </button>
                            ) : (
                              <span className="admin-badge admin-badge--disabled">
                                อ่านอย่างเดียว
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
        <h3>⚠️ คำเตือน</h3>
        <ul>
          <li>การเปลี่ยนแปลงการตั้งค่าจะมีผลทันทีและอาจส่งผลกระทบต่อการทำงานของระบบ</li>
          <li>การกระทำทั้งหมดจะถูกบันทึกในบันทึกการดำเนินการ</li>
          <li>ค่าต้องอยู่ในรูปแบบ JSON ที่ถูกต้อง (เช่น "string", 123, true, ["array"])</li>
          <li>การตั้งค่าที่ทำเครื่องหมาย "อ่านอย่างเดียว" ไม่สามารถแก้ไขได้</li>
        </ul>
      </aside>
    </div>
  );
}
