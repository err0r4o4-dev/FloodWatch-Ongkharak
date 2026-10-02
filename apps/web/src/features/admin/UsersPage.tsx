import { useEffect, useState } from "react";
import { NavLink } from "react-router";
import { adminApi } from "./api";
import type { UserManagement } from "./types";

export function UsersPage() {
  const [users, setUsers] = useState<UserManagement[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
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
      setError(err instanceof Error ? err.message : "Failed to load users");
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

  const formatDate = (dateString: string) => {
    return new Date(dateString).toLocaleDateString("th-TH", {
      year: "numeric",
      month: "short",
      day: "numeric",
      hour: "2-digit",
      minute: "2-digit",
    });
  };

  return (
    <div className="page-container admin-page">
      <header className="page-header">
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
          <div>
            <h1>จัดการผู้ใช้งาน</h1>
            <p>ดูรายชื่อและจัดการบัญชีผู้ใช้งาน</p>
          </div>
          <NavLink to="/admin" className="button">← กลับ</NavLink>
        </div>
      </header>

      <div className="panel">
        <form onSubmit={handleSearch} className="admin-toolbar">
          <input
            type="search"
            placeholder="ค้นหาอีเมล..."
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            className="admin-search"
          />
          
          <div className="admin-filter-group">
            <label htmlFor="role-filter">บทบาท:</label>
            <select
              id="role-filter"
              value={roleFilter}
              onChange={(e) => setRoleFilter(e.target.value)}
              className="admin-search"
              style={{ minWidth: "8rem" }}
            >
              <option value="">ทั้งหมด</option>
              <option value="USER">ผู้ใช้งาน</option>
              <option value="ADMIN">ผู้ดูแลระบบ</option>
            </select>
          </div>

          <label style={{ display: "flex", alignItems: "center", gap: "0.5rem" }}>
            <input
              type="checkbox"
              checked={includeDisabled}
              onChange={(e) => setIncludeDisabled(e.target.checked)}
            />
            <span>แสดงบัญชีที่ปิดการใช้งาน</span>
          </label>

          <button type="submit" className="button button--primary">
            ค้นหา
          </button>
        </form>

        {loading && (
          <div className="loading-state" style={{ padding: "var(--space-8)" }}>
            <p>กำลังโหลดรายชื่อผู้ใช้งาน...</p>
          </div>
        )}

        {error && (
          <div className="error-state" style={{ padding: "var(--space-8)" }}>
            <p>เกิดข้อผิดพลาด: {error}</p>
            <button onClick={() => void loadUsers()} className="button">
              ลองอีกครั้ง
            </button>
          </div>
        )}

        {!loading && !error && users.length === 0 && (
          <div className="empty-state" style={{ padding: "var(--space-8)", textAlign: "center" }}>
            <p>ไม่พบผู้ใช้งาน</p>
          </div>
        )}

        {!loading && !error && users.length > 0 && (
          <>
            <div style={{ overflowX: "auto" }}>
              <table className="admin-table">
                <thead>
                  <tr>
                    <th>อีเมล</th>
                    <th>บทบาท</th>
                    <th>สถานะ</th>
                    <th>สร้างเมื่อ</th>
                    <th>การกระทำ</th>
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
                          {user.role === "ADMIN" ? "ผู้ดูแลระบบ" : "ผู้ใช้งาน"}
                        </span>
                      </td>
                      <td>
                        {user.disabledAt ? (
                          <span className="admin-badge admin-badge--disabled">ปิดใช้งาน</span>
                        ) : (
                          <span className="admin-badge admin-badge--active">ใช้งานอยู่</span>
                        )}
                      </td>
                      <td className="muted small">{formatDate(user.createdAt)}</td>
                      <td>
                        <div className="admin-actions">
                          <NavLink
                            to={`/admin/users/${user.id}`}
                            className="button button--small"
                          >
                            ดูรายละเอียด
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
