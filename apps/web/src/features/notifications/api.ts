import { apiClient } from "../auth/api";
import { getLocale } from "../../i18n";
import type { Locale } from "../../i18n";

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

export type PublicNotification = {
  id: string;
  severity: NotificationSeverity;
  sourceType: NotificationSourceType;
  createdAt: string;
  title: string;
  message: string;
  url: string;
  location: { type: "Point"; coordinates: [number, number] } | null;
};

export type PublicNotificationPage = {
  items: PublicNotification[];
  pagination: { total: number; limit: number; offset: number };
};

export type NotificationListOptions = {
  limit: number;
  offset: number;
  signal?: AbortSignal;
};

export type PublicNotificationListOptions = NotificationListOptions & { bbox?: string };

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

function validPublicLocation(value: unknown): value is PublicNotification["location"] {
  if (value === null) return true;
  if (!isRecord(value) || value.type !== "Point" || !Array.isArray(value.coordinates) || value.coordinates.length !== 2) return false;
  return value.coordinates.every((coordinate) => typeof coordinate === "number" && Number.isFinite(coordinate)) &&
    Math.abs(value.coordinates[0] as number) <= 180 && Math.abs(value.coordinates[1] as number) <= 90;
}

function validPublicURL(value: string): boolean {
  if (!value.startsWith("/") || value.startsWith("//") || value.includes("\\") || /[\u0000-\u001f\u007f-\u009f]/u.test(value)) return false;
  try {
    const pathname = new URL(value, "http://floodwatch.invalid").pathname;
    return pathname === "/announcements" || pathname === "/reports";
  } catch {
    return false;
  }
}

function parsePublicNotificationPage(value: unknown): PublicNotificationPage {
  if (!isRecord(value) || !Array.isArray(value.items) || !isRecord(value.pagination)) {
    throw new Error("Invalid public notifications response");
  }
  const { total, limit, offset } = value.pagination;
  if (!isNonNegativeInteger(total) || !isNonNegativeInteger(limit) || limit === 0 || !isNonNegativeInteger(offset)) {
    throw new Error("Invalid public notifications response");
  }
  const items = value.items.map((item) => {
    if (!isRecord(item) || typeof item.id !== "string" || typeof item.createdAt !== "string" || Number.isNaN(new Date(item.createdAt).getTime()) || typeof item.title !== "string" || typeof item.message !== "string" || typeof item.url !== "string" || !validPublicURL(item.url) || typeof item.severity !== "string" || !severities.has(item.severity as NotificationSeverity) || typeof item.sourceType !== "string" || !sourceTypes.has(item.sourceType as NotificationSourceType) || !validPublicLocation(item.location)) {
      throw new Error("Invalid public notifications response");
    }
    return item as unknown as PublicNotification;
  });
  return { items, pagination: { total, limit, offset } };
}

export const notificationApi = {
  list: async ({ limit, offset, signal }: NotificationListOptions): Promise<NotificationPage> => {
    const searchParams = new URLSearchParams({ limit: String(limit), offset: String(offset) });
    const value = await apiClient.request<unknown>(`/notifications?${searchParams}`, { signal });
    return parseNotificationPage(value);
  },

  listPublic: async ({ limit, offset, bbox, signal }: PublicNotificationListOptions): Promise<PublicNotificationPage> => {
    const searchParams = new URLSearchParams({ limit: String(limit), offset: String(offset) });
    if (bbox) searchParams.set("bbox", bbox);
    const value = await apiClient.request<unknown>(`/notifications/public?${searchParams}`, { signal }, { authenticate: false, retryUnauthorized: false });
    return parsePublicNotificationPage(value);
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

  subscribe: async (subscription: PushSubscription, locale: Locale = getLocale()): Promise<void> => {
    const { endpoint, keys } = subscription.toJSON();
    await apiClient.request<void>("/notifications/subscribe", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ endpoint, keys, locale })
    }, { retryUnauthorized: false });
  },

  unsubscribe: async (endpoint: string): Promise<void> => {
    const searchParams = new URLSearchParams({ endpoint });
    await apiClient.request<void>(`/notifications/unsubscribe?${searchParams}`, { method: "DELETE" }, { retryUnauthorized: false });
  },

  updateLanguage: async (endpoint: string, locale: Locale): Promise<void> => {
    await apiClient.request<void>("/notifications/language", {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ endpoint, locale })
    }, { retryUnauthorized: false });
  }
};
