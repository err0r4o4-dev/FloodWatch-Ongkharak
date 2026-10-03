import { apiClient } from "../auth/api";

export const NOTIFICATION_PAGE_SIZE = 10;

export type NotificationSeverity = "INFO" | "WATCH" | "WARNING" | "CRITICAL";
export type NotificationSourceType = "OFFICIAL" | "COMMUNITY" | "AI";

export type NotificationItem = {
  id: string;
  alertId: string;
  title: string;
  message: string;
  severity: NotificationSeverity;
  sourceType: NotificationSourceType;
  createdAt: string;
  readAt: string | null;
};

export type NotificationPage = {
  items: NotificationItem[];
  pagination: { total: number; limit: number; offset: number };
};

export type NotificationListOptions = {
  limit: number;
  offset: number;
  signal?: AbortSignal;
};

const severities = new Set<NotificationSeverity>(["INFO", "WATCH", "WARNING", "CRITICAL"]);
const sourceTypes = new Set<NotificationSourceType>(["OFFICIAL", "COMMUNITY", "AI"]);

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

function isNonNegativeInteger(value: unknown): value is number {
  return typeof value === "number" && Number.isInteger(value) && value >= 0;
}

function parseNotificationPage(value: unknown): NotificationPage {
  if (!isRecord(value) || !Array.isArray(value.items) || !isRecord(value.pagination)) {
    throw new Error("Invalid notifications response");
  }

  const { total, limit, offset } = value.pagination;
  if (!isNonNegativeInteger(total) || !isNonNegativeInteger(limit) || limit === 0 || !isNonNegativeInteger(offset)) {
    throw new Error("Invalid notifications response");
  }

  const items = value.items.map((item) => {
    if (!isRecord(item) || typeof item.id !== "string" || typeof item.alertId !== "string" || typeof item.title !== "string" || typeof item.message !== "string" || typeof item.createdAt !== "string" || (item.readAt !== null && typeof item.readAt !== "string") || typeof item.severity !== "string" || !severities.has(item.severity as NotificationSeverity) || typeof item.sourceType !== "string" || !sourceTypes.has(item.sourceType as NotificationSourceType)) {
      throw new Error("Invalid notifications response");
    }
    return {
      id: item.id,
      alertId: item.alertId,
      title: item.title,
      message: item.message,
      severity: item.severity as NotificationSeverity,
      sourceType: item.sourceType as NotificationSourceType,
      createdAt: item.createdAt,
      readAt: item.readAt
    };
  });

  return { items, pagination: { total, limit, offset } };
}

function parseUnreadCount(value: unknown): { count: number } {
  if (!isRecord(value) || !isNonNegativeInteger(value.count)) {
    throw new Error("Invalid unread notifications response");
  }
  return { count: value.count };
}

export const notificationApi = {
  list: async ({ limit, offset, signal }: NotificationListOptions): Promise<NotificationPage> => {
    const searchParams = new URLSearchParams({ limit: String(limit), offset: String(offset) });
    const value = await apiClient.request<unknown>(`/notifications?${searchParams}`, { signal });
    return parseNotificationPage(value);
  },

  getUnreadCount: async (): Promise<{ count: number }> => {
    const value = await apiClient.request<unknown>("/notifications/unread-count");
    return parseUnreadCount(value);
  },

  markRead: async (id: string): Promise<void> => {
    await apiClient.request<void>(`/notifications/${encodeURIComponent(id)}/read`, { method: "PATCH" });
  },

  markAllRead: async (): Promise<void> => {
    await apiClient.request<void>("/notifications/read-all", { method: "POST" });
  },

  getPublicKey: async (): Promise<{ publicKey: string }> => {
    const value = await apiClient.request<unknown>("/notifications/public-key", {}, { authenticate: false, retryUnauthorized: false });
    if (!value || typeof value !== "object" || !("publicKey" in value) || typeof value.publicKey !== "string" || !value.publicKey.trim()) {
      throw new Error("Push notifications are not configured");
    }
    return { publicKey: value.publicKey };
  },

  subscribe: async (subscription: PushSubscription): Promise<void> => {
    const { endpoint, keys } = subscription.toJSON();
    await apiClient.request<void>("/notifications/subscribe", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ endpoint, keys })
    });
  },

  unsubscribe: async (endpoint: string): Promise<void> => {
    const searchParams = new URLSearchParams({ endpoint });
    await apiClient.request<void>(`/notifications/unsubscribe?${searchParams}`, { method: "DELETE" });
  }
};
