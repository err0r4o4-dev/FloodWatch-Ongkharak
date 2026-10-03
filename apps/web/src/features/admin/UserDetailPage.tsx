import { useEffect, useRef, useState } from "react";
import { NavLink, useParams } from "react-router";
import { adminApi } from "./api";
import type { UserManagement, UserStats } from "./types";
import { confirmAction } from "../../shared/alerts";

export function UserDetailPage() {
  const { id } = useParams<{ id: string }>();
  const [user, setUser] = useState<UserManagement | null>(null);
  const [stats, setStats] = useState<UserStats | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [actionLoading, setActionLoading] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);
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
      setError(err instanceof Error ? err.message : "Failed to load user");
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
        const message = cause instanceof Error ? cause.message : "ไม่สามารถดำเนินการได้ กรุณาลองใหม่อีกครั้ง";
        setActionError(message);
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
      title: "เปลี่ยนบทบาทผู้ใช้งานหรือไม่?",
      text: `เปลี่ยนบทบาทของ ${user.email} เป็น ${newRole === "ADMIN" ? "ผู้ดูแลระบบ" : "ผู้ใช้งาน"} การเปลี่ยนแปลงนี้มีผลต่อสิทธิ์เข้าถึงระบบ`,
      confirmText: "เปลี่ยนบทบาท",
      danger: true,
      action: async () => { await adminApi.updateUserRole(id, newRole); await loadUser(); },
      successTitle: "เปลี่ยนบทบาทแล้ว"
    });
  };

  const handleDisable = async () => {
    if (!id || !user) return;
    await manageUser({
      title: "ปิดการใช้งานบัญชีหรือไม่?",
      text: `บัญชี ${user.email} จะไม่สามารถใช้งานฟีเจอร์สมาชิกได้`,
      confirmText: "ปิดการใช้งาน",
      danger: true,
      action: async () => { await adminApi.disableUser(id); await loadUser(); },
      successTitle: "ปิดการใช้งานบัญชีแล้ว"
    });
  };

  const handleEnable = async () => {
    if (!id || !user) return;
    await manageUser({
      title: "เปิดการใช้งานบัญชีหรือไม่?",
      text: `เปิดให้บัญชี ${user.email} กลับมาใช้งานฟีเจอร์สมาชิกอีกครั้ง`,
      confirmText: "เปิดการใช้งาน",
      action: async () => { await adminApi.enableUser(id); await loadUser(); },
      successTitle: "เปิดการใช้งานบัญชีแล้ว"
    });
  };

  const formatDate = (dateString: string | null) => {
    if (!dateString) return "-";
    return new Date(dateString).toLocaleDateString("th-TH", {
      year: "numeric",
      month: "long",
      day: "numeric",
      hour: "2-digit",
      minute: "2-digit",
    });
  };

  if (loading) {
    return (
      <div className="page-container admin-page">
        <div className="panel loading-state">
          <p>กำลังโหลดข้อมูลผู้ใช้งาน...</p>
        </div>
      </div>
    );
  }

  if (error || !user || !stats) {
    return (
      <div className="page-container admin-page">
        <div className="panel error-state">
          <h1>เกิดข้อผิดพลาด</h1>
          <p>{error || "ไม่พบผู้ใช้งาน"}</p>
          <NavLink to="/admin/users" className="button">กลับไปรายชื่อผู้ใช้งาน</NavLink>
        </div>
      </div>
    );
  }

  return (
    <div className="page-container admin-page">
      <header className="page-header">
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
          <div>
            <h1>รายละเอียดผู้ใช้งาน</h1>
            <p className="muted">{user.email}</p>
          </div>
          <NavLink to="/admin/users" className="button">← กลับ</NavLink>
        </div>
      </header>

      {actionError && (
        <div className="panel error-state" style={{ marginBottom: "var(--space-6)" }}>
          <p>เกิดข้อผิดพลาด: {actionError}</p>
        </div>
      )}

      <div style={{ display: "grid", gap: "var(--space-6)", gridTemplateColumns: "repeat(auto-fit, minmax(20rem, 1fr))" }}>
        <section className="panel">
          <h2>ข้อมูลบัญชี</h2>
          <dl style={{ display: "grid", gap: "var(--space-3)", marginTop: "var(--space-4)" }}>
            <div>
              <dt className="muted small">ID</dt>
              <dd style={{ fontFamily: "monospace", fontSize: "0.875rem" }}>{user.id}</dd>
            </div>
            <div>
              <dt className="muted small">อีเมล</dt>
              <dd><strong>{user.email}</strong></dd>
            </div>
            <div>
              <dt className="muted small">บทบาท</dt>
              <dd>
                <span className={`admin-badge admin-badge--${user.role.toLowerCase()}`}>
                  {user.role === "ADMIN" ? "ผู้ดูแลระบบ" : "ผู้ใช้งาน"}
                </span>
              </dd>
            </div>
            <div>
              <dt className="muted small">สถานะ</dt>
              <dd>
                {user.disabledAt ? (
                  <span className="admin-badge admin-badge--disabled">ปิดใช้งาน</span>
                ) : (
                  <span className="admin-badge admin-badge--active">ใช้งานอยู่</span>
                )}
              </dd>
            </div>
            <div>
              <dt className="muted small">สร้างเมื่อ</dt>
              <dd>{formatDate(user.createdAt)}</dd>
            </div>
            <div>
              <dt className="muted small">อัปเดตล่าสุด</dt>
              <dd>{formatDate(user.updatedAt)}</dd>
            </div>
            {user.disabledAt && (
              <div>
                <dt className="muted small">ปิดการใช้งานเมื่อ</dt>
                <dd>{formatDate(user.disabledAt)}</dd>
              </div>
            )}
          </dl>
        </section>

        <section className="panel">
          <h2>สถิติกิจกรรม</h2>
          <dl style={{ display: "grid", gap: "var(--space-3)", marginTop: "var(--space-4)" }}>
            <div>
              <dt className="muted small">รายงานทั้งหมด</dt>
              <dd><strong>{stats.reportCount}</strong> รายงาน</dd>
            </div>
            <div>
              <dt className="muted small">รายงานที่ได้รับการอนุมัติ</dt>
              <dd><strong>{stats.verifiedReports}</strong> รายงาน</dd>
            </div>
            <div>
              <dt className="muted small">รายงานที่ถูกปฏิเสธ</dt>
              <dd><strong>{stats.rejectedReports}</strong> รายงาน</dd>
            </div>
            <div>
              <dt className="muted small">บันทึกการดำเนินการ</dt>
              <dd><strong>{stats.auditLogCount}</strong> รายการ</dd>
            </div>
            <div>
              <dt className="muted small">เซสชันที่ใช้งาน</dt>
              <dd><strong>{stats.refreshSessionCount}</strong> เซสชัน</dd>
            </div>
            <div>
              <dt className="muted small">กิจกรรมล่าสุด</dt>
              <dd>{formatDate(stats.lastActivity)}</dd>
            </div>
          </dl>
        </section>
      </div>

      <section className="panel" style={{ marginTop: "var(--space-6)" }}>
        <h2>การกระทำของผู้ดูแลระบบ</h2>
        <p className="muted">การกระทำเหล่านี้จะถูกบันทึกลงในบันทึกการดำเนินการ</p>

        <div style={{ display: "flex", gap: "var(--space-4)", marginTop: "var(--space-4)", flexWrap: "wrap" }}>
          <div>
            <h3 style={{ fontSize: "1rem", marginBottom: "var(--space-2)" }}>เปลี่ยนบทบาท</h3>
            <div className="admin-actions">
              <button
                onClick={() => void handleUpdateRole("USER")}
                disabled={actionLoading || user.role === "USER"}
                className="button"
              >
                ตั้งเป็นผู้ใช้งาน
              </button>
              <button
                onClick={() => void handleUpdateRole("ADMIN")}
                disabled={actionLoading || user.role === "ADMIN"}
                className="button"
              >
                ตั้งเป็นผู้ดูแลระบบ
              </button>
            </div>
          </div>

          <div>
            <h3 style={{ fontSize: "1rem", marginBottom: "var(--space-2)" }}>จัดการสถานะบัญชี</h3>
            <div className="admin-actions">
              {user.disabledAt ? (
                <button
                  onClick={() => void handleEnable()}
                  disabled={actionLoading}
                  className="button button--primary"
                >
                  เปิดการใช้งานบัญชี
                </button>
              ) : (
                <button
                  onClick={() => void handleDisable()}
                  disabled={actionLoading}
                  className="button"
                  style={{ background: "var(--color-danger-soft)", color: "var(--color-danger-ink)" }}
                >
                  ปิดการใช้งานบัญชี
                </button>
              )}
            </div>
          </div>
        </div>

        {actionLoading && (
          <p className="muted" style={{ marginTop: "var(--space-4)" }}>กำลังดำเนินการ...</p>
        )}
      </section>

      <section className="panel" style={{ marginTop: "var(--space-6)", background: "var(--color-canvas-soft)" }}>
        <h3>ดูเพิ่มเติม</h3>
        <div className="admin-actions" style={{ marginTop: "var(--space-3)" }}>
          <NavLink
            to={`/admin/audit-logs?actorId=${user.id}`}
            className="button"
          >
            ดูบันทึกการดำเนินการของผู้ใช้นี้
          </NavLink>
        </div>
      </section>
    </div>
  );
}
