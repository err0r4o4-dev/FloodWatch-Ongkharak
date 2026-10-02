import React, { useState, useEffect } from 'react';
import { useAuth } from '../auth/AuthProvider';
import { notificationApi } from './api';

export function NotificationToggle() {
  const { account } = useAuth();
  const [supported, setSupported] = useState(false);
  const [subscribed, setSubscribed] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if ('serviceWorker' in navigator && 'PushManager' in window) {
      setSupported(true);
      checkSubscription();
    }
  }, []);

  const checkSubscription = async () => {
    try {
      const registration = await navigator.serviceWorker.ready;
      const sub = await registration.pushManager.getSubscription();
      setSubscribed(!!sub);
    } catch (err) {
      console.error('Error checking subscription', err);
    }
  };

  const urlBase64ToUint8Array = (base64String: string) => {
    const padding = '='.repeat((4 - (base64String.length % 4)) % 4);
    const base64 = (base64String + padding).replace(/-/g, '+').replace(/_/g, '/');
    const rawData = window.atob(base64);
    const outputArray = new Uint8Array(rawData.length);
    for (let i = 0; i < rawData.length; ++i) {
      outputArray[i] = rawData.charCodeAt(i);
    }
    return outputArray;
  };

  const handleToggle = async () => {
    if (!account) {
      setError('You must be logged in to manage notifications.');
      return;
    }

    setLoading(true);
    setError(null);

    try {
      const registration = await navigator.serviceWorker.ready;
      let subscription = await registration.pushManager.getSubscription();

      if (subscribed && subscription) {
        // Unsubscribe
        await notificationApi.unsubscribe(subscription.endpoint);
        await subscription.unsubscribe();
        setSubscribed(false);
      } else {
        // Subscribe
        const { publicKey } = await notificationApi.getPublicKey();
        const applicationServerKey = urlBase64ToUint8Array(publicKey);

        subscription = await registration.pushManager.subscribe({
          userVisibleOnly: true,
          applicationServerKey,
        });

        await notificationApi.subscribe(subscription);
        setSubscribed(true);
      }
    } catch (err: any) {
      console.error('Subscription error', err);
      setError(err.message || 'An error occurred while toggling notifications.');
    } finally {
      setLoading(false);
    }
  };

  if (!supported) {
    return <div className="text-sm text-gray-500">Push notifications are not supported by your browser.</div>;
  }

  return (
    <div className="flex flex-col gap-2">
      <div className="flex items-center gap-3">
        <label className="relative inline-flex items-center cursor-pointer">
          <input
            type="checkbox"
            className="sr-only peer"
            checked={subscribed}
            onChange={handleToggle}
            disabled={loading || !account}
          />
          <div className="w-11 h-6 bg-gray-200 peer-focus:outline-none peer-focus:ring-4 peer-focus:ring-blue-300 dark:peer-focus:ring-blue-800 rounded-full peer dark:bg-gray-700 peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all dark:border-gray-600 peer-checked:bg-blue-600"></div>
          <span className="ml-3 text-sm font-medium text-gray-900 dark:text-gray-300">
            {subscribed ? 'Notifications Enabled' : 'Enable Notifications'}
          </span>
        </label>
        {loading && <span className="text-sm text-gray-500">Processing...</span>}
      </div>
      {error && <p className="text-sm text-red-600">{error}</p>}
      {!account && <p className="text-sm text-gray-500">Log in to receive push notifications.</p>}
    </div>
  );
}
