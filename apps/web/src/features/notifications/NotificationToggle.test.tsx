import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { NotificationToggle } from './NotificationToggle';
import { useAuth } from '../auth/AuthProvider';
import { notificationApi } from './api';

// Mock dependencies
vi.mock('../auth/AuthProvider');
vi.mock('./api');

describe('NotificationToggle', () => {
  beforeEach(() => {
    vi.resetAllMocks();
    vi.mocked(useAuth).mockReturnValue({ account: null } as ReturnType<typeof useAuth>);

    // Mock navigator.serviceWorker and window.PushManager
    Object.defineProperty(navigator, 'serviceWorker', {
      value: {
        ready: Promise.resolve({
          pushManager: {
            getSubscription: vi.fn().mockResolvedValue(null),
            subscribe: vi.fn().mockResolvedValue({
              endpoint: 'test-endpoint',
              toJSON: () => ({ endpoint: 'test-endpoint', keys: { p256dh: 'p', auth: 'a' } })
            }),
          }
        })
      },
      configurable: true
    });

    Object.defineProperty(window, 'PushManager', {
      value: {},
      configurable: true
    });

  });

  it('shows unsupported message if push manager is missing', () => {
    Object.defineProperty(window, 'PushManager', { value: undefined, configurable: true });

    render(<NotificationToggle />);
    expect(screen.getByText('Push notifications are not supported by your browser.')).toBeDefined();
  });

  it('renders enable notifications when supported and logged in', async () => {
    vi.mocked(useAuth).mockReturnValue({ account: { id: '1', role: 'USER' } } as any);

    render(<NotificationToggle />);

    // Wait for the async check to complete
    await screen.findByText('Enable Notifications');

    const checkbox = screen.getByRole('checkbox');
    expect(checkbox).not.toBeDisabled();
    expect((checkbox as HTMLInputElement).checked).toBe(false);
  });

  it('toggles subscription on click', async () => {
    vi.mocked(useAuth).mockReturnValue({ account: { id: '1', role: 'USER' } } as any);
    vi.mocked(notificationApi.getPublicKey).mockResolvedValue({ publicKey: 'dGVzdA==' }); // "test" in base64
    vi.mocked(notificationApi.subscribe).mockResolvedValue(undefined);

    render(<NotificationToggle />);

    await screen.findByText('Enable Notifications');
    const checkbox = screen.getByRole('checkbox');

    fireEvent.click(checkbox);

    // Wait for it to become 'Notifications Enabled'
    await screen.findByText('Notifications Enabled');
    expect((checkbox as HTMLInputElement).checked).toBe(true);
    expect(notificationApi.subscribe).toHaveBeenCalled();
  });
});
