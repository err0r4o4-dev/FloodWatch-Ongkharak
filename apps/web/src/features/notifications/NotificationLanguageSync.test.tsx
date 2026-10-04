import { fireEvent, render, screen, waitFor, act } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { LanguageProvider, LanguageSwitcher } from "../../i18n";
import { NotificationLanguageSync } from "./NotificationLanguageSync";
import { notificationApi } from "./api";
import { ApiError } from "../auth/api";

const auth = vi.hoisted(() => ({ account: { id: "synthetic-user" } as { id: string } | null, status: "authenticated" }));
vi.mock("../auth/AuthProvider", () => ({ useOptionalAuth: () => auth }));
vi.mock("./api", () => ({ notificationApi: { updateLanguage: vi.fn().mockResolvedValue(undefined) } }));
const subscription = { endpoint: "https://push.example.test/device" } as PushSubscription;
beforeEach(() => { localStorage.clear(); vi.clearAllMocks(); auth.account = { id: "synthetic-user" }; auth.status = "authenticated"; vi.stubGlobal("navigator", { serviceWorker: { getRegistration: vi.fn().mockResolvedValue({ pushManager: { getSubscription: vi.fn().mockResolvedValue(subscription) } }) } }); });
afterEach(() => vi.unstubAllGlobals());

describe("notification language synchronization", () => {
  it("does not re-enroll missing or foreign-account subscriptions", async () => {
    vi.mocked(notificationApi.updateLanguage).mockRejectedValueOnce(new ApiError("missing", 404, "NOT_FOUND"));
    render(<LanguageProvider><LanguageSwitcher /><NotificationLanguageSync /></LanguageProvider>);
    await waitFor(() => expect(notificationApi.updateLanguage).toHaveBeenCalledTimes(1));
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });
  it("does not mutate after logout while looking up the subscription", async () => {
    let finish: (registration: unknown) => void = () => {};
    vi.stubGlobal("navigator", { serviceWorker: { getRegistration: vi.fn().mockImplementation(() => new Promise((resolve) => { finish = resolve; })) } });
    const view = render(<LanguageProvider><NotificationLanguageSync /></LanguageProvider>);
    await waitFor(() => expect(navigator.serviceWorker.getRegistration).toHaveBeenCalled());
    auth.account = null;
    auth.status = "anonymous";
    view.rerender(<LanguageProvider><NotificationLanguageSync /></LanguageProvider>);
    await act(async () => finish({ pushManager: { getSubscription: vi.fn().mockResolvedValue(subscription) } }));
    expect(notificationApi.updateLanguage).not.toHaveBeenCalled();
  });
  it("reconciles a completed enrollment using the latest selected language", async () => {
    render(<LanguageProvider><LanguageSwitcher /><NotificationLanguageSync /></LanguageProvider>);
    await waitFor(() => expect(notificationApi.updateLanguage).toHaveBeenCalledTimes(1));
    fireEvent.click(screen.getByRole("button", { name: "EN" }));
    await waitFor(() => expect(notificationApi.updateLanguage).toHaveBeenCalledTimes(2));
    act(() => window.dispatchEvent(new Event("floodwatch:push-enrolled")));
    await waitFor(() => expect(notificationApi.updateLanguage).toHaveBeenCalledTimes(3));
    expect(notificationApi.updateLanguage).toHaveBeenLastCalledWith(subscription.endpoint, "en");
  });

  it("saves the selected language for existing subscriptions without prompting", async () => {
    render(<LanguageProvider><LanguageSwitcher /><NotificationLanguageSync /></LanguageProvider>);
    await waitFor(() => expect(notificationApi.updateLanguage).toHaveBeenLastCalledWith(subscription.endpoint, "th"));
    fireEvent.click(screen.getByRole("button", { name: "EN" }));
    await waitFor(() => expect(notificationApi.updateLanguage).toHaveBeenLastCalledWith(subscription.endpoint, "en"));
  });
  it("shows an honest retry action when saving the language fails", async () => {
    vi.mocked(notificationApi.updateLanguage).mockRejectedValueOnce(new Error("offline"));
    render(<LanguageProvider><LanguageSwitcher /><NotificationLanguageSync /></LanguageProvider>);
    expect(await screen.findByRole("alert")).toHaveTextContent("บันทึกภาษาสำหรับการแจ้งเตือนไม่สำเร็จ");
    fireEvent.click(screen.getByRole("button", { name: "ลองอีกครั้ง" }));
    await waitFor(() => expect(screen.queryByRole("alert")).not.toBeInTheDocument());
  });
  it("serializes requests and ignores obsolete choices during rapid switching", async () => {
    let finish: () => void = () => {};
    vi.mocked(notificationApi.updateLanguage).mockImplementationOnce(() => new Promise<void>((resolve) => { finish = resolve; }));
    render(<LanguageProvider><LanguageSwitcher /><NotificationLanguageSync /></LanguageProvider>);
    await waitFor(() => expect(notificationApi.updateLanguage).toHaveBeenCalledTimes(1));
    fireEvent.click(screen.getByRole("button", { name: "EN" }));
    fireEvent.click(screen.getByRole("button", { name: "TH" }));
    fireEvent.click(screen.getByRole("button", { name: "EN" }));
    expect(notificationApi.updateLanguage).toHaveBeenCalledTimes(1);
    finish();
    await waitFor(() => expect(notificationApi.updateLanguage).toHaveBeenCalledTimes(2));
    expect(notificationApi.updateLanguage).toHaveBeenLastCalledWith(subscription.endpoint, "en");
  });
});
