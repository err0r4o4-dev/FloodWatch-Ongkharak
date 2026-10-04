import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useOptionalAuth } from "../auth/AuthProvider";
import { notificationApi } from "./api";
import { NotificationsPage } from "./NotificationsPage";

vi.mock("../auth/AuthProvider");
vi.mock("./api");
vi.mock("./NotificationToggle", () => ({ NotificationToggle: () => <p>Push controls</p> }));

const account = { id: "test-user", email: "user@example.test", role: "USER" as const, createdAt: "2026-01-01T00:00:00Z" };
const notification = {
  id: "notification-1",
  alertId: "alert-1",
  title: "ฝนตกหนักในพื้นที่",
  message: "โปรดติดตามประกาศจากหน่วยงานทางการ",
  severity: "WARNING" as const,
  sourceType: "OFFICIAL" as const,
  createdAt: "2026-10-04T08:00:00Z",
  readAt: null
};
const otherAccount = { id: "other-user", email: "other@example.test", role: "USER" as const, createdAt: "2026-01-01T00:00:00Z" };
const otherNotification = { ...notification, id: "notification-2", title: "ประกาศสำหรับบัญชีใหม่" };

function renderPage() {
  return render(<MemoryRouter><NotificationsPage /></MemoryRouter>);
}

function authenticated() {
  vi.mocked(useOptionalAuth).mockReturnValue({ account, status: "authenticated", login: vi.fn(), logout: vi.fn(), register: vi.fn() });
}

describe("NotificationsPage", () => {
  beforeEach(() => {
    vi.resetAllMocks();
    authenticated();
    vi.mocked(notificationApi.list).mockResolvedValue({ items: [], pagination: { total: 0, limit: 10, offset: 0 } });
    vi.mocked(notificationApi.getUnreadCount).mockResolvedValue({ count: 0 });
    vi.mocked(notificationApi.markRead).mockResolvedValue(undefined);
    vi.mocked(notificationApi.markAllRead).mockResolvedValue(undefined);
  });

  it("does not request an authenticated inbox for guests", () => {
    vi.mocked(useOptionalAuth).mockReturnValue({ account: null, status: "anonymous", login: vi.fn(), logout: vi.fn(), register: vi.fn() });
    renderPage();

    expect(screen.getByRole("heading", { name: "ลงชื่อเข้าใช้เพื่อดูการแจ้งเตือน" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "เข้าสู่ระบบ" })).toHaveAttribute("href", "/login");
    expect(notificationApi.list).not.toHaveBeenCalled();
    expect(notificationApi.getUnreadCount).not.toHaveBeenCalled();
  });

  it("shows account loading while authentication is unresolved", () => {
    vi.mocked(useOptionalAuth).mockReturnValue({ account: null, status: "loading", login: vi.fn(), logout: vi.fn(), register: vi.fn() });
    renderPage();

    expect(screen.getByRole("status")).toHaveTextContent("กำลังตรวจสอบบัญชี");
    expect(notificationApi.list).not.toHaveBeenCalled();
  });

  it("shows inbox loading, then an honest empty state", async () => {
    let resolveList: (value: Awaited<ReturnType<typeof notificationApi.list>>) => void = () => undefined;
    vi.mocked(notificationApi.list).mockReturnValue(new Promise((resolve) => { resolveList = resolve; }));
    renderPage();

    expect(screen.getByRole("status")).toHaveTextContent("กำลังโหลดการแจ้งเตือน");
    resolveList({ items: [], pagination: { total: 0, limit: 10, offset: 0 } });

    expect(await screen.findByRole("heading", { name: "ยังไม่มีการแจ้งเตือน" })).toBeInTheDocument();
    expect(screen.getByText("เมื่อมีการแจ้งเตือนใหม่ รายการจะแสดงที่นี่")).toBeInTheDocument();
  });

  it("shows an error and retries both inbox requests", async () => {
    vi.mocked(notificationApi.list).mockRejectedValueOnce(new Error("offline"));
    renderPage();

    expect(await screen.findByRole("alert")).toHaveTextContent("โหลดการแจ้งเตือนไม่สำเร็จ");
    vi.mocked(notificationApi.list).mockResolvedValueOnce({ items: [notification], pagination: { total: 1, limit: 10, offset: 0 } });
    vi.mocked(notificationApi.getUnreadCount).mockResolvedValueOnce({ count: 1 });
    fireEvent.click(screen.getByRole("button", { name: "ลองอีกครั้ง" }));

    expect(await screen.findByText(notification.title)).toBeInTheDocument();
    expect(notificationApi.list).toHaveBeenCalledTimes(2);
    expect(notificationApi.getUnreadCount).toHaveBeenCalledTimes(2);
  });

  it("does not present zero when the unread count request fails", async () => {
    vi.mocked(notificationApi.list).mockResolvedValue({ items: [notification], pagination: { total: 1, limit: 10, offset: 0 } });
    vi.mocked(notificationApi.getUnreadCount).mockRejectedValue(new Error("count unavailable"));
    renderPage();

    expect(await screen.findByRole("alert")).toHaveTextContent("โหลดการแจ้งเตือนไม่สำเร็จ");
    expect(screen.getByRole("button", { name: "อ่านทั้งหมดแล้ว" })).toBeDisabled();
    expect(screen.queryByRole("button", { name: "อ่านทั้งหมดแล้ว (0)" })).not.toBeInTheDocument();
    expect(screen.getByText("จำนวนที่ยังไม่ได้อ่านยังไม่พร้อมใช้งาน")).toBeInTheDocument();
  });

  it("marks an unread item and updates the unread state", async () => {
    vi.mocked(notificationApi.list)
      .mockResolvedValueOnce({ items: [notification], pagination: { total: 1, limit: 10, offset: 0 } })
      .mockResolvedValueOnce({ items: [{ ...notification, readAt: "2026-10-04T08:01:00Z" }], pagination: { total: 1, limit: 10, offset: 0 } });
    vi.mocked(notificationApi.getUnreadCount).mockResolvedValueOnce({ count: 1 }).mockResolvedValueOnce({ count: 3 });
    renderPage();

    const markRead = await screen.findByRole("button", { name: `ทำเครื่องหมายว่าอ่านแล้ว: ${notification.title}` });
    expect(screen.getByText("ยังไม่ได้อ่าน")).toBeInTheDocument();
    fireEvent.click(markRead);

    await waitFor(() => expect(notificationApi.markRead).toHaveBeenCalledWith(notification.id));
    expect(await screen.findByText("อ่านแล้ว")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "อ่านทั้งหมดแล้ว (3)" })).toBeEnabled();
    expect(notificationApi.list).toHaveBeenCalledTimes(2);
    expect(notificationApi.getUnreadCount).toHaveBeenCalledTimes(2);
    expect(screen.queryByRole("button", { name: `ทำเครื่องหมายว่าอ่านแล้ว: ${notification.title}` })).not.toBeInTheDocument();
  });

  it("marks all visible notifications read and bounds pagination", async () => {
    vi.mocked(notificationApi.list)
      .mockResolvedValueOnce({ items: [notification], pagination: { total: 21, limit: 10, offset: 0 } })
      .mockResolvedValueOnce({ items: [{ ...notification, readAt: "2026-10-04T08:02:00Z" }], pagination: { total: 21, limit: 10, offset: 0 } })
      .mockResolvedValueOnce({ items: [{ ...notification, readAt: "2026-10-04T08:03:00Z" }], pagination: { total: 21, limit: 10, offset: 10 } });
    vi.mocked(notificationApi.getUnreadCount).mockResolvedValueOnce({ count: 2 }).mockResolvedValueOnce({ count: 1 }).mockResolvedValueOnce({ count: 0 });
    renderPage();

    expect(await screen.findByRole("button", { name: "อ่านทั้งหมดแล้ว (2)" })).toBeEnabled();
    expect(screen.getByRole("button", { name: "หน้าก่อนหน้า" })).toBeDisabled();
    fireEvent.click(screen.getByRole("button", { name: "อ่านทั้งหมดแล้ว (2)" }));
    await waitFor(() => expect(notificationApi.markAllRead).toHaveBeenCalledTimes(1));
    expect(await screen.findByRole("button", { name: "อ่านทั้งหมดแล้ว (1)" })).toBeEnabled();

    fireEvent.click(screen.getByRole("button", { name: "หน้าถัดไป" }));
    await waitFor(() => expect(notificationApi.list).toHaveBeenLastCalledWith({ limit: 10, offset: 10, signal: expect.any(AbortSignal) }));
    expect(screen.getByRole("button", { name: "หน้าถัดไป" })).toBeEnabled();
    expect(screen.getByRole("button", { name: "หน้าก่อนหน้า" })).toBeEnabled();
  });

  it("shows an unknown unread count when the canonical reload after mark-all fails", async () => {
    vi.mocked(notificationApi.list)
      .mockResolvedValueOnce({ items: [notification], pagination: { total: 1, limit: 10, offset: 0 } })
      .mockRejectedValueOnce(new Error("reload failed"));
    vi.mocked(notificationApi.getUnreadCount).mockResolvedValueOnce({ count: 1 }).mockResolvedValueOnce({ count: 0 });
    renderPage();

    fireEvent.click(await screen.findByRole("button", { name: "อ่านทั้งหมดแล้ว (1)" }));
    await waitFor(() => expect(notificationApi.markAllRead).toHaveBeenCalledTimes(1));
    expect(await screen.findByRole("alert")).toHaveTextContent("โหลดการแจ้งเตือนไม่สำเร็จ");
    expect(screen.getByRole("button", { name: "อ่านทั้งหมดแล้ว" })).toBeDisabled();
    expect(screen.queryByRole("button", { name: "อ่านทั้งหมดแล้ว (0)" })).not.toBeInTheDocument();
    expect(screen.getByText("จำนวนที่ยังไม่ได้อ่านยังไม่พร้อมใช้งาน")).toBeInTheDocument();
  });

  it("ignores a pending mark-read completion after the account changes", async () => {
    let activeAccount = account;
    let resolveMarkRead: () => void = () => undefined;
    vi.mocked(useOptionalAuth).mockImplementation(() => ({ account: activeAccount, status: "authenticated", login: vi.fn(), logout: vi.fn(), register: vi.fn() }));
    vi.mocked(notificationApi.list)
      .mockResolvedValueOnce({ items: [notification], pagination: { total: 1, limit: 10, offset: 0 } })
      .mockResolvedValueOnce({ items: [otherNotification], pagination: { total: 1, limit: 10, offset: 0 } });
    vi.mocked(notificationApi.getUnreadCount).mockResolvedValueOnce({ count: 1 }).mockResolvedValueOnce({ count: 1 });
    vi.mocked(notificationApi.markRead).mockReturnValue(new Promise((resolve) => { resolveMarkRead = resolve; }));
    const view = renderPage();

    fireEvent.click(await screen.findByRole("button", { name: `ทำเครื่องหมายว่าอ่านแล้ว: ${notification.title}` }));
    activeAccount = otherAccount;
    view.rerender(<MemoryRouter><NotificationsPage /></MemoryRouter>);
    expect(screen.queryByText(notification.title)).not.toBeInTheDocument();
    expect(await screen.findByText(otherNotification.title)).toBeInTheDocument();

    await act(async () => { resolveMarkRead(); });
    expect(screen.getByText("ยังไม่ได้อ่าน")).toBeInTheDocument();
    expect(screen.getByText("ยังไม่ได้อ่าน 1 รายการ")).toBeInTheDocument();
    expect(notificationApi.list).toHaveBeenCalledTimes(2);
    expect(notificationApi.getUnreadCount).toHaveBeenCalledTimes(2);
  });
});
