import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useOptionalAuth } from "../auth/AuthProvider";
import { notificationApi } from "./api";
import { NotificationToggle } from "./NotificationToggle";

vi.mock("../auth/AuthProvider");
vi.mock("./api");
vi.mock("../../shared/alerts", () => ({ notifyError: vi.fn(), notifySuccess: vi.fn() }));

const account = { id: "test-user", email: "user@example.test", role: "USER" as const, createdAt: "2026-01-01T00:00:00Z" };
const subscription = {
  endpoint: "https://push.example.test/subscription",
  toJSON: () => ({ endpoint: "https://push.example.test/subscription", keys: { p256dh: "test-key", auth: "test-auth" } }),
  unsubscribe: vi.fn()
};
const getSubscription = vi.fn();
const subscribe = vi.fn();
const getRegistration = vi.fn();

function renderToggle() {
  return render(<MemoryRouter><NotificationToggle /></MemoryRouter>);
}

describe("NotificationToggle", () => {
  beforeEach(() => {
    vi.resetAllMocks();
    vi.mocked(useOptionalAuth).mockReturnValue({ account, status: "authenticated", login: vi.fn(), logout: vi.fn(), register: vi.fn() });
    subscription.unsubscribe.mockResolvedValue(true);
    getSubscription.mockResolvedValue(null);
    subscribe.mockResolvedValue(subscription);
    getRegistration.mockResolvedValue({ active: {}, pushManager: { getSubscription, subscribe } });
    vi.stubGlobal("navigator", { serviceWorker: { getRegistration } });
    vi.stubGlobal("PushManager", class {});
    vi.stubGlobal("Notification", { permission: "default", requestPermission: vi.fn() });
    vi.mocked(notificationApi.getPublicKey).mockResolvedValue({ publicKey: "dGVzdA==" });
    vi.mocked(notificationApi.subscribe).mockResolvedValue(undefined);
    vi.mocked(notificationApi.unsubscribe).mockResolvedValue(undefined);
  });

  afterEach(() => { vi.unstubAllGlobals(); });

  it("explains browser support instead of exposing a broken toggle", () => {
    vi.stubGlobal("PushManager", undefined);
    renderToggle();
    expect(screen.getByText("เบราว์เซอร์นี้ไม่รองรับการแจ้งเตือนแบบ Push")).toBeInTheDocument();
    expect(screen.queryByRole("checkbox")).not.toBeInTheDocument();
  });

  it("asks guests to sign in without requesting browser permission", () => {
    vi.mocked(useOptionalAuth).mockReturnValue({ account: null, status: "anonymous", login: vi.fn(), logout: vi.fn(), register: vi.fn() });
    renderToggle();
    expect(screen.getByRole("link", { name: "เข้าสู่ระบบเพื่อเปิดแจ้งเตือน" })).toHaveAttribute("href", "/login");
    expect(screen.queryByRole("checkbox")).not.toBeInTheDocument();
    expect(Notification.requestPermission).not.toHaveBeenCalled();
    expect(subscribe).not.toHaveBeenCalled();
  });

  it("waits for authentication before exposing account settings", () => {
    vi.mocked(useOptionalAuth).mockReturnValue({ account: null, status: "loading", login: vi.fn(), logout: vi.fn(), register: vi.fn() });
    renderToggle();
    expect(screen.getByRole("status")).toHaveTextContent("กำลังตรวจสอบบัญชี");
    expect(screen.queryByRole("link", { name: "เข้าสู่ระบบเพื่อเปิดแจ้งเตือน" })).not.toBeInTheDocument();
  });

  it("keeps the toggle disabled while checking the browser subscription", async () => {
    let resolveRegistration: (value: unknown) => void = () => undefined;
    getRegistration.mockReturnValue(new Promise((resolve) => { resolveRegistration = resolve; }));
    renderToggle();
    expect(screen.getByRole("checkbox", { name: "รับการแจ้งเตือนผ่านเบราว์เซอร์" })).toBeDisabled();
    resolveRegistration({ active: {}, pushManager: { getSubscription, subscribe } });
    await waitFor(() => expect(screen.getByRole("checkbox")).not.toBeDisabled());
    expect(Notification.requestPermission).not.toHaveBeenCalled();
    expect(subscribe).not.toHaveBeenCalled();
  });

  it("reports a missing service worker without waiting indefinitely", async () => {
    getRegistration.mockResolvedValue(undefined);
    renderToggle();
    expect(await screen.findByText("ระบบแจ้งเตือนบนเบราว์เซอร์ยังไม่พร้อมใช้งาน")).toBeInTheDocument();
    expect(screen.getByRole("checkbox")).toBeDisabled();
    expect(screen.getByRole("button", { name: "ตรวจสอบอีกครั้ง" })).toBeEnabled();
  });

  it("reports browser permission denial without silently retrying the prompt", async () => {
    vi.stubGlobal("Notification", { permission: "denied", requestPermission: vi.fn() });
    renderToggle();
    expect(await screen.findByText("เบราว์เซอร์ปิดกั้นการแจ้งเตือนอยู่")).toBeInTheDocument();
    expect(screen.getByRole("checkbox")).toBeDisabled();
    expect(Notification.requestPermission).not.toHaveBeenCalled();
  });

  it("enables push only after the server accepts the subscription", async () => {
    let resolveSubscription: () => void = () => undefined;
    vi.mocked(notificationApi.subscribe).mockReturnValue(new Promise((resolve) => { resolveSubscription = resolve; }));
    renderToggle();
    await waitFor(() => expect(screen.getByRole("checkbox")).toBeEnabled());
    fireEvent.click(screen.getByRole("checkbox"));
    await screen.findByText("กำลังบันทึกการตั้งค่า…");
    expect(screen.getByRole("checkbox")).not.toBeChecked();
    expect(screen.getByRole("checkbox")).toBeDisabled();
    resolveSubscription();
    await screen.findByText("เปิดการแจ้งเตือนในเบราว์เซอร์นี้แล้ว");
    expect(screen.getByRole("checkbox")).toBeChecked();
  });

  it("keeps a failed subscription visibly off and allows another attempt", async () => {
    vi.mocked(notificationApi.subscribe).mockRejectedValueOnce(new Error("server unavailable"));
    renderToggle();
    await waitFor(() => expect(screen.getByRole("checkbox")).toBeEnabled());
    fireEvent.click(screen.getByRole("checkbox"));
    expect(await screen.findByRole("alert")).toHaveTextContent("เปลี่ยนการตั้งค่าไม่สำเร็จ");
    expect(screen.getByRole("checkbox")).not.toBeChecked();
    await waitFor(() => expect(screen.getByRole("checkbox")).toBeEnabled());
    fireEvent.click(screen.getByRole("checkbox"));
    await screen.findByText("เปิดการแจ้งเตือนในเบราว์เซอร์นี้แล้ว");
    expect(screen.getByRole("checkbox")).toBeChecked();
  });

  it("keeps push enabled when the server cannot unsubscribe", async () => {
    getSubscription.mockResolvedValue(subscription);
    vi.mocked(notificationApi.unsubscribe).mockRejectedValue(new Error("server unavailable"));
    renderToggle();
    await waitFor(() => expect(screen.getByRole("checkbox")).toBeEnabled());
    expect(screen.getByRole("checkbox")).toBeChecked();
    fireEvent.click(screen.getByRole("checkbox"));
    expect(await screen.findByRole("alert")).toHaveTextContent("เปลี่ยนการตั้งค่าไม่สำเร็จ");
    expect(screen.getByRole("checkbox")).toBeChecked();
    expect(subscription.unsubscribe).not.toHaveBeenCalled();
  });

  it("turns push off after both the server and browser unsubscribe", async () => {
    getSubscription.mockResolvedValue(subscription);
    renderToggle();
    await waitFor(() => expect(screen.getByRole("checkbox")).toBeEnabled());
    fireEvent.click(screen.getByRole("checkbox"));
    await screen.findByText("ยังไม่ได้เปิดการแจ้งเตือนในเบราว์เซอร์นี้");
    expect(screen.getByRole("checkbox")).not.toBeChecked();
  });

  it("shows a subscription-check error and can recover through the retry action", async () => {
    getRegistration.mockRejectedValueOnce(new Error("browser unavailable"));
    renderToggle();
    expect(await screen.findByRole("alert")).toHaveTextContent("ตรวจสอบการแจ้งเตือนไม่สำเร็จ");
    expect(screen.getByRole("checkbox")).toBeDisabled();
    fireEvent.click(screen.getByRole("button", { name: "ตรวจสอบอีกครั้ง" }));
    await waitFor(() => expect(screen.getByRole("checkbox")).toBeEnabled());
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("does not create a new subscription when turning off one that already expired", async () => {
    getSubscription.mockResolvedValueOnce(subscription).mockResolvedValue(null);
    renderToggle();
    await waitFor(() => expect(screen.getByRole("checkbox")).toBeEnabled());
    expect(screen.getByRole("checkbox")).toBeChecked();
    fireEvent.click(screen.getByRole("checkbox"));
    await screen.findByText("ยังไม่ได้เปิดการแจ้งเตือนในเบราว์เซอร์นี้");
    expect(screen.getByRole("checkbox")).not.toBeChecked();
    expect(subscribe).not.toHaveBeenCalled();
    expect(notificationApi.subscribe).not.toHaveBeenCalled();
  });
});
