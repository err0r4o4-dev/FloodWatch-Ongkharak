import type { UserManagement, UserStats, SystemSetting, AuditLog, AdminDashboardStats } from "./types";
import { apiClient } from "../auth/api";

async function requestAdmin<T>(url: string, options: RequestInit = {}): Promise<T> {
  const headers = new Headers(options.headers);
  headers.set("Content-Type", "application/json");
  return apiClient.request<T>(url, { ...options, headers });
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
    
    return requestAdmin<{ users: UserManagement[]; pagination: { total: number; limit: number; offset: number } }>(
      `/admin/users?${searchParams.toString()}`
    );
  },

  async getUser(userId: string) {
    return requestAdmin<{ user: UserManagement; stats: UserStats }>(`/admin/users/${userId}`);
  },

  async updateUserRole(userId: string, role: string) {
    return requestAdmin<void>(`/admin/users/${userId}/role`, {
      method: "PATCH",
      body: JSON.stringify({ role }),
    });
  },

  async disableUser(userId: string) {
    return requestAdmin<void>(`/admin/users/${userId}/disable`, { method: "POST" });
  },

  async enableUser(userId: string) {
    return requestAdmin<void>(`/admin/users/${userId}/enable`, { method: "POST" });
  },

  // System settings
  async listSettings(params?: { category?: string; editable?: boolean }) {
    const searchParams = new URLSearchParams();
    if (params?.category) searchParams.set("category", params.category);
    if (params?.editable !== undefined) searchParams.set("editable", String(params.editable));
    
    return requestAdmin<{ settings: SystemSetting[] }>(`/admin/settings?${searchParams.toString()}`);
  },

  async getSetting(key: string) {
    return requestAdmin<{ setting: SystemSetting }>(`/admin/settings/${key}`);
  },

  async updateSetting(key: string, value: unknown) {
    return requestAdmin<void>(`/admin/settings/${key}`, {
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
    
    return requestAdmin<{ logs: AuditLog[]; pagination: { total: number; limit: number; offset: number } }>(
      `/admin/audit-logs?${searchParams.toString()}`
    );
  },

  async getAuditLog(logId: string) {
    return requestAdmin<{ log: AuditLog }>(`/admin/audit-logs/${logId}`);
  },

  async getDashboardStats(): Promise<AdminDashboardStats> {
    return requestAdmin<AdminDashboardStats>("/admin/dashboard/stats");
  },
};
