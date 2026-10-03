import { apiClient } from "../auth/api";

export const notificationApi = {
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
