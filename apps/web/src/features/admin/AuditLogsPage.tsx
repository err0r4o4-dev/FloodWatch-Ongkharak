import { useEffect, useState } from "react";
import { NavLink, useSearchParams } from "react-router";
import { adminApi } from "./api";
import type { AuditLog, AuditAction, TargetType } from "./types";

export function AuditLogsPage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const [logs, setLogs] = useState<AuditLog[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
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
      setError(err instanceof Error ? err.message : "Failed to load audit logs");
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

  const formatDate = (dateString: string) => {
    return new Date(dateString).toLocaleDateString("th-TH", {
      year: "numeric",
      month: "short",
      day: "numeric",
      hour: "2-digit",
      minute: "2-digit",
      second: "2-digit",
    });
  };

  const getActionLabel = (action: AuditAction): string => {
    const labels: Record<AuditAction, string> = {
      "user.created": "สร้างผู้ใช้งาน",
      "user.disabled": "ปิดการใช้งานผู้ใช้",
      "user.enabled": "เปิดการใช้งานผู้ใช้",
      "user.role_changed": "เปลี่ยนบทบาทผู้ใช้",
      "user.password_changed": "เปลี่ยนรหัสผ่าน",
      "alert_rule.created": "สร้างกฎการแจ้งเตือน",
      "alert_rule.updated": "อัปเดตกฎการแจ้งเตือน",
      "alert_rule.deleted": "ลบกฎการแจ้งเตือน",
      "setting.updated": "อัปเดตการตั้งค่า",
      "report.reviewed": "ตรวจสอบรายงาน",
      "auth.login_success": "เข้าสู่ระบบสำเร็จ",
      "auth.login_failed": "เข้าสู่ระบบล้มเหลว",
      "auth.logout": "ออกจากระบบ",
    };
    return labels[action] || action;
  };

  const getTargetTypeLabel = (targetType: TargetType): string => {
    const labels: Record<TargetType, string> = {
      USER: "ผู้ใช้งาน",
      ALERT_RULE: "กฎการแจ้งเตือน",
      DATA_SOURCE: "แหล่งข้อมูล",
      COMMUNITY_REPORT: "รายงานชุมชน",
      SYSTEM_SETTING: "การตั้งค่าระบบ",
      AUTH_SESSION: "เซสชัน",
    };
    return labels[targetType];
  };

  const getActorRoleBadge = (role: string) => {
    const badges = {
      ADMIN: { class: "admin-badge--admin", label: "ผู้ดูแลระบบ" },
      USER: { class: "admin-badge--user", label: "ผู้ใช้งาน" },
      SYSTEM: { class: "admin-badge--disabled", label: "ระบบ" },
    };
    const badge = badges[role as keyof typeof badges] || { class: "", label: role };
    return <span className={`admin-badge ${badge.class}`}>{badge.label}</span>;
  };

  return (
    <div className="page-container admin-page">
      <header className="page-header">
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
          <div>
            <h1>บันทึกการดำเนินการ</h1>
            <p>ตรวจสอบประวัติการกระทำในระบบ</p>
            {actorIdParam && (
              <p className="muted small">
                กรองตามผู้ใช้: {actorIdParam}{" "}
                <button
                  onClick={() => {
                    searchParams.delete("actorId");
                    setSearchParams(searchParams);
                  }}
                  className="button button--small"
                  style={{ marginLeft: "var(--space-2)" }}
                >
                  ล้างตัวกรอง
                </button>
              </p>
            )}
          </div>
          <NavLink to="/admin" className="button">← กลับ</NavLink>
        </div>
      </header>

      <div className="panel">
        <div className="admin-toolbar">
          <div className="admin-filter-group">
            <label htmlFor="action-filter">การกระทำ:</label>
            <select
              id="action-filter"
              value={actionFilter}
              onChange={(e) => setActionFilter(e.target.value as AuditAction | "")}
              className="admin-search"
              style={{ minWidth: "12rem" }}
            >
              <option value="">ทั้งหมด</option>
              <option value="user.created">สร้างผู้ใช้งาน</option>
              <option value="user.disabled">ปิดการใช้งานผู้ใช้</option>
              <option value="user.enabled">เปิดการใช้งานผู้ใช้</option>
              <option value="user.role_changed">เปลี่ยนบทบาท</option>
              <option value="setting.updated">อัปเดตการตั้งค่า</option>
              <option value="auth.login_success">เข้าสู่ระบบ</option>
              <option value="auth.logout">ออกจากระบบ</option>
            </select>
          </div>

          <div className="admin-filter-group">
            <label htmlFor="target-filter">ประเภทเป้าหมาย:</label>
            <select
              id="target-filter"
              value={targetTypeFilter}
              onChange={(e) => setTargetTypeFilter(e.target.value as TargetType | "")}
              className="admin-search"
              style={{ minWidth: "12rem" }}
            >
              <option value="">ทั้งหมด</option>
              <option value="USER">ผู้ใช้งาน</option>
              <option value="SYSTEM_SETTING">การตั้งค่า</option>
              <option value="ALERT_RULE">กฎการแจ้งเตือน</option>
              <option value="COMMUNITY_REPORT">รายงานชุมชน</option>
              <option value="AUTH_SESSION">เซสชัน</option>
            </select>
          </div>
        </div>

        {loading && (
          <div className="loading-state" style={{ padding: "var(--space-8)" }}>
            <p>กำลังโหลดบันทึก...</p>
          </div>
        )}

        {error && (
          <div className="error-state" style={{ padding: "var(--space-8)" }}>
            <p>เกิดข้อผิดพลาด: {error}</p>
            <button onClick={() => void loadLogs()} className="button">
              ลองอีกครั้ง
            </button>
          </div>
        )}

        {!loading && !error && logs.length === 0 && (
          <div className="empty-state" style={{ padding: "var(--space-8)", textAlign: "center" }}>
            <p>ไม่พบบันทึกการดำเนินการ</p>
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
                  onClick={() => setExpandedLog(expandedLog === log.id ? null : log.id)}
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
                        โดย: {log.actorEmail} • เวลา: {formatDate(log.createdAt)}
                      </p>
                    </div>
                    <span aria-hidden="true">{expandedLog === log.id ? "▼" : "▶"}</span>
                  </div>

                  {expandedLog === log.id && (
                    <div style={{ marginTop: "var(--space-4)", paddingTop: "var(--space-4)", borderTop: "1px solid var(--color-border)" }}>
                      <dl style={{ display: "grid", gap: "var(--space-3)", fontSize: "0.875rem" }}>
                        <div>
                          <dt className="muted">ID บันทึก</dt>
                          <dd style={{ fontFamily: "monospace" }}>{log.id}</dd>
                        </div>
                        {log.actorId && (
                          <div>
                            <dt className="muted">ID ผู้กระทำ</dt>
                            <dd style={{ fontFamily: "monospace" }}>{log.actorId}</dd>
                          </div>
                        )}
                        <div>
                          <dt className="muted">ID เป้าหมาย</dt>
                          <dd style={{ fontFamily: "monospace" }}>{log.targetId}</dd>
                        </div>
                        {log.ipAddress && (
                          <div>
                            <dt className="muted">IP Address</dt>
                            <dd style={{ fontFamily: "monospace" }}>{log.ipAddress}</dd>
                          </div>
                        )}
                        {log.userAgent && (
                          <div>
                            <dt className="muted">User Agent</dt>
                            <dd style={{ fontFamily: "monospace", fontSize: "0.75rem", wordBreak: "break-all" }}>
                              {log.userAgent}
                            </dd>
                          </div>
                        )}
                        {Object.keys(log.metadata).length > 0 && (
                          <div>
                            <dt className="muted">ข้อมูลเพิ่มเติม</dt>
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
                แสดง {pagination.offset + 1} ถึง {Math.min(pagination.offset + pagination.limit, pagination.total)} จากทั้งหมด {pagination.total} รายการ
              </div>
              <div className="admin-actions">
                <button
                  onClick={handlePrevPage}
                  disabled={pagination.offset === 0}
                  className="button"
                >
                  ← ก่อนหน้า
                </button>
                <button
                  onClick={handleNextPage}
                  disabled={pagination.offset + pagination.limit >= pagination.total}
                  className="button"
                >
                  ถัดไป →
                </button>
              </div>
            </div>
          </>
        )}
      </div>
    </div>
  );
}
