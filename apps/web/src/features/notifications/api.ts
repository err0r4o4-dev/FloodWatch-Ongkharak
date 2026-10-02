import { fetchClient } from '../../shared/api/client';

export const notificationApi = {
  getPublicKey: async (): Promise<{ publicKey: string }> => {
    return fetchClient('/api/v1/notifications/public-key', {
      method: 'GET',
    });
  },

  subscribe: async (subscription: PushSubscription): Promise<void> => {
    const subJSON = subscription.toJSON();
    await fetchClient('/api/v1/notifications/subscribe', {
      method: 'POST',
      body: JSON.stringify({
        endpoint: subJSON.endpoint,
        keys: subJSON.keys,
      }),
    });
  },

  unsubscribe: async (endpoint: string): Promise<void> => {
    const searchParams = new URLSearchParams({ endpoint });
    await fetchClient(`/api/v1/notifications/unsubscribe?${searchParams}`, {
      method: 'DELETE',
    });
  },
};
