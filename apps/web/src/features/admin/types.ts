import type { AccountRole } from "../auth/types";

export type UserManagement = {
  id: string;
  email: string;
  role: AccountRole;
  createdAt: string;
  updatedAt: string;
  disabledAt: string | null;
};

export type UserStats = {
  reportCount: number;
  lastActivity: string | null;
  verifiedReports: number;
  rejectedReports: number;
  auditLogCount: number;
  refreshSessionCount: number;
};

export type SettingCategory = "rate_limit" | "alert" | "ingestion" | "feature" | "security";
export type SettingValueType = "string" | "number" | "boolean" | "json";

export type SystemSetting = {
  key: string;
  value: unknown;
  valueType: SettingValueType;
  description: string;
  category: SettingCategory;
  editable: boolean;
  createdAt: string;
  updatedAt: string;
  updatedBy: string | null;
};

export type AuditAction =
  | "user.created"
  | "user.disabled"
  | "user.enabled"
  | "user.role_changed"
  | "user.password_changed"
  | "alert_rule.created"
  | "alert_rule.updated"
  | "alert_rule.deleted"
  | "setting.updated"
  | "report.reviewed"
  | "auth.login_success"
  | "auth.login_failed"
  | "auth.logout";

export type ActorRole = "USER" | "ADMIN" | "SYSTEM";
export type TargetType = "USER" | "ALERT_RULE" | "DATA_SOURCE" | "COMMUNITY_REPORT" | "SYSTEM_SETTING" | "AUTH_SESSION";

export type AuditLog = {
  id: string;
  action: AuditAction;
  actorId: string | null;
  actorEmail: string;
  actorRole: ActorRole;
  targetType: TargetType;
  targetId: string;
  metadata: Record<string, unknown>;
  ipAddress: string | null;
  userAgent: string | null;
  createdAt: string;
};

export type AdminDashboardStats = {
  totalUsers: number;
  activeUsers: number;
  totalReports: number;
  pendingReports: number;
  recentActivity: number;
};
