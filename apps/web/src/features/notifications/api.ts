import { apiConfig } from "../../shared/config/api";

export const notificationApi = {
  getPublicKey: async (): Promise<{ publicKey: string }> => {
    const response = await fetch(`${apiConfig.baseUrl}/notifications/public-key`, {
      method: 'GET',
      headers: {
        'Content-Type': 'application/json'
      }
    });
    if (!response.ok) throw new Error("Failed to fetch public key");
    return response.json();
  },

  subscribe: async (subscription: PushSubscription): Promise<void> => {
    const subJSON = subscription.toJSON();
    const response = await fetch(`${apiConfig.baseUrl}/notifications/subscribe`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json'
      },
      body: JSON.stringify({
        endpoint: subJSON.endpoint,
        keys: subJSON.keys,
      }),
    });
    if (!response.ok) throw new Error("Failed to subscribe");
  },

  unsubscribe: async (endpoint: string): Promise<void> => {
    const searchParams = new URLSearchParams({ endpoint });
    const response = await fetch(`${apiConfig.baseUrl}/notifications/unsubscribe?${searchParams}`, {
      method: 'DELETE',
    });
    if (!response.ok) throw new Error("Failed to unsubscribe");
  },
};
