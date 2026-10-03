import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { NotificationToggle } from "./NotificationToggle";
import { notificationApi } from "./api";
import { notifyError, notifySuccess } from "../../shared/alerts";

vi.mock("../auth/AuthProvider", () => {
  const account = { id: "member", email: "member@example.test", role: "USER", createdAt: "2026-10-02T00:00:00Z" };
  return { useAuth: () => ({ account, status: "authenticated" }), useOptionalAuth: () => ({ account, status: "authenticated" }) };
});
vi.mock("./api", () => ({ notificationApi: { getPublicKey: vi.fn(), subscribe: vi.fn(), unsubscribe: vi.fn() } }));
vi.mock("../../shared/alerts", () => ({ notifyError: vi.fn(), notifySuccess: vi.fn(), notifyInfo: vi.fn() }));
const subscription = { endpoint: "https://push.example.test/alert-fixture", unsubscribe: vi.fn() };
const getSubscription = vi.fn();
const subscribe = vi.fn();

describe("push settings action notifications", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    getSubscription.mockResolvedValue(null);
    subscribe.mockResolvedValue(subscription);
    subscription.unsubscribe.mockResolvedValue(true);
    const registration = { active: {}, pushManager: { getSubscription, subscribe } };
    Object.defineProperty(navigator, "serviceWorker", { value: { ready: Promise.resolve(registration), getRegistration: vi.fn().mockResolvedValue(registration) }, configurable: true });
    vi.stubGlobal("PushManager", class {});
    vi.stubGlobal("Notification", { permission: "granted" });
    vi.mocked(notificationApi.getPublicKey).mockResolvedValue({ publicKey: "dGVzdA==" });
    vi.mocked(notificationApi.subscribe).mockResolvedValue(undefined);
    vi.mocked(notificationApi.unsubscribe).mockResolvedValue(undefined);
  });
  afterEach(() => { vi.unstubAllGlobals(); });
  async function toggle() {
    render(<MemoryRouter><NotificationToggle /></MemoryRouter>);
    await waitFor(() => expect(screen.getByRole("checkbox")).toBeEnabled());
    fireEvent.click(screen.getByRole("checkbox"));
  }

  it("notifies after the subscription has been accepted", async () => {
    await toggle();
    await waitFor(() => expect(notifySuccess).toHaveBeenCalledTimes(1));
    expect(notificationApi.subscribe).toHaveBeenCalledTimes(1);
    expect(screen.getByRole("checkbox")).toBeChecked();
  });

  it("does not claim success when subscription fails", async () => {
    vi.mocked(notificationApi.subscribe).mockRejectedValue(new Error("offline"));
    await toggle();
    await waitFor(() => expect(notifyError).toHaveBeenCalledTimes(1));
    expect(notifySuccess).not.toHaveBeenCalled();
    expect(screen.getByRole("checkbox")).not.toBeChecked();
  });

  it("does not claim push is disabled if the browser refuses to unsubscribe", async () => {
    getSubscription.mockResolvedValue(subscription);
    subscription.unsubscribe.mockResolvedValue(false);
    render(<MemoryRouter><NotificationToggle /></MemoryRouter>);
    await waitFor(() => expect(screen.getByRole("checkbox")).toBeChecked());
    fireEvent.click(screen.getByRole("checkbox"));
    await waitFor(() => expect(notifyError).toHaveBeenCalledTimes(1));
    expect(notifySuccess).not.toHaveBeenCalled();
  });
});
