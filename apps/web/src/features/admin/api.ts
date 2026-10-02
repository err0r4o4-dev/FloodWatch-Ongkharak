import type { UserManagement, UserStats, SystemSetting, AuditLog, AdminDashboardStats } from "./types";

const BASE_URL = "/api/v1";

async function fetchWithAuth<T>(url: string, options: RequestInit = {}): Promise<T> {
  const token = sessionStorage.getItem("accessToken");
  const headers = {
    ...options.headers,
    "Content-Type": "application/json",
    ...(token && { Authorization: `Bearer ${token}` }),
  };

  const response = await fetch(`${BASE_URL}${url}`, { ...options, headers });

  if (!response.ok) {
    const error = await response.json().catch(() => ({ error: { message: "Request failed" } }));
    throw new Error(error.error?.message || "Request failed");
  }

  if (response.status === 204) {
    return undefined as T;
  }

  return response.json();
}

export const adminApi = {
  // User management
  async listUsers(params?: { role?: string; includeDisabled?: boolean; search?: string; limit?: number; offset?: number }) {
    const searchParams = new URLSearchParams();
    if (params?.role) searchParams.set("role", params.role);
    if (params?.includeDisabled) searchParams.set("includeDisabled", "true");
    if (params?.search) searchParams.set("search", params.search);
    if (params?.limit) searchParams.set("limit", String(params.limit));
    if (params?.offset) searchParams.set("offset", String(params.offset));
    
    return fetchWithAuth<{ users: UserManagement[]; pagination: { total: number; limit: number; offset: number } }>(
      `/admin/users?${searchParams.toString()}`
    );
  },

  async getUser(userId: string) {
    return fetchWithAuth<{ user: UserManagement; stats: UserStats }>(`/admin/users/${userId}`);
  },

  async updateUserRole(userId: string, role: string) {
    return fetchWithAuth<void>(`/admin/users/${userId}/role`, {
      method: "PATCH",
      body: JSON.stringify({ role }),
    });
  },

  async disableUser(userId: string) {
    return fetchWithAuth<void>(`/admin/users/${userId}/disable`, { method: "POST" });
  },

  async enableUser(userId: string) {
    return fetchWithAuth<void>(`/admin/users/${userId}/enable`, { method: "POST" });
  },

  // System settings
  async listSettings(params?: { category?: string; editable?: boolean }) {
    const searchParams = new URLSearchParams();
    if (params?.category) searchParams.set("category", params.category);
    if (params?.editable !== undefined) searchParams.set("editable", String(params.editable));
    
    return fetchWithAuth<{ settings: SystemSetting[] }>(`/admin/settings?${searchParams.toString()}`);
  },

  async getSetting(key: string) {
    return fetchWithAuth<{ setting: SystemSetting }>(`/admin/settings/${key}`);
  },

  async updateSetting(key: string, value: unknown) {
    return fetchWithAuth<void>(`/admin/settings/${key}`, {
      method: "PUT",
      body: JSON.stringify({ value }),
    });
  },

  // Audit logs
  async listAuditLogs(params?: {
    actorId?: string;
    targetType?: string;
    targetId?: string;
    action?: string;
    startTime?: string;
    endTime?: string;
    limit?: number;
    offset?: number;
  }) {
    const searchParams = new URLSearchParams();
    if (params?.actorId) searchParams.set("actorId", params.actorId);
    if (params?.targetType) searchParams.set("targetType", params.targetType);
    if (params?.targetId) searchParams.set("targetId", params.targetId);
    if (params?.action) searchParams.set("action", params.action);
    if (params?.startTime) searchParams.set("startTime", params.startTime);
    if (params?.endTime) searchParams.set("endTime", params.endTime);
    if (params?.limit) searchParams.set("limit", String(params.limit));
    if (params?.offset) searchParams.set("offset", String(params.offset));
    
    return fetchWithAuth<{ logs: AuditLog[]; pagination: { total: number; limit: number; offset: number } }>(
      `/admin/audit-logs?${searchParams.toString()}`
    );
  },

  async getAuditLog(logId: string) {
    return fetchWithAuth<{ log: AuditLog }>(`/admin/audit-logs/${logId}`);
  },

  // Dashboard stats (mock for now - can be implemented later)
  async getDashboardStats(): Promise<AdminDashboardStats> {
    // This would be a real endpoint in the future
    return {
      totalUsers: 0,
      activeUsers: 0,
      totalReports: 0,
      pendingReports: 0,
      recentActivity: 0,
    };
  },
};
