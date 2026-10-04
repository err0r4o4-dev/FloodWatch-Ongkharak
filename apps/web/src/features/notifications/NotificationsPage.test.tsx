import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useOptionalAuth } from "../auth/AuthProvider";
import { notificationApi } from "./api";
import { NotificationsPage } from "./NotificationsPage";
import { LanguageProvider, LanguageSwitcher } from "../../i18n";

const realtime = vi.hoisted(() => ({ revision: 0 }));

vi.mock("../auth/AuthProvider");
vi.mock("./api");
vi.mock("../realtime/RealtimeProvider", () => ({ useRealtimeRevision: () => realtime.revision }));

const publicItem = {
  id: "report:public-1",
  severity: "INFO" as const,
  sourceType: "COMMUNITY" as const,
  createdAt: "2026-10-04T08:00:00Z",
  title: "Community report",
  message: "Water near the market",
  url: "/reports?report=public-1",
  location: { type: "Point" as const, coordinates: [101, 14] as [number, number] }
};

const account = { id: "test-user", email: "user@example.test", role: "USER" as const, createdAt: "2026-01-01T00:00:00Z" };

function renderPage() {
  return render(<MemoryRouter><NotificationsPage /></MemoryRouter>);
}

describe("NotificationsPage public feed", () => {
  it("switches public system copy while preserving originals, filters, drafts, and read state without refetching", async () => {
    const original = { ...publicItem, title: "รายงานต้นฉบับ", message: "ข้อความชุมชนต้นฉบับ" };
    vi.mocked(notificationApi.listPublic).mockResolvedValue({ items: [original], pagination: { total: 1, limit: 10, offset: 0 } });
    render(<LanguageProvider><LanguageSwitcher /><MemoryRouter><NotificationsPage /></MemoryRouter></LanguageProvider>);
    await screen.findByText(original.title);
    fireEvent.change(screen.getByLabelText("ขอบเขตพื้นที่บนแผนที่"), { target: { value: "100,13,102,15" } });
    fireEvent.click(screen.getByRole("button", { name: "ข้อมูล" }));
    fireEvent.click(screen.getByRole("button", { name: `ทำเครื่องหมายว่าอ่านแล้ว: ${original.title}` }));
    fireEvent.click(screen.getByRole("button", { name: "EN" }));
    expect(screen.getByText(original.title)).toBeInTheDocument();
    expect(screen.getByText(original.message)).toBeInTheDocument();
    expect(screen.getByText("Original announcement · Not automatically translated")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Information" })).toHaveAttribute("aria-pressed", "true");
    expect(screen.getByLabelText("Map area bounds")).toHaveValue("100,13,102,15");
    expect(screen.getByText("Read")).toBeInTheDocument();
    expect(notificationApi.listPublic).toHaveBeenCalledTimes(1);
  });
  beforeEach(() => {
    vi.resetAllMocks();
    realtime.revision = 0;
    localStorage.clear();
    vi.mocked(useOptionalAuth).mockReturnValue({ account: null, status: "anonymous", login: vi.fn(), logout: vi.fn(), register: vi.fn(), updateUsername: vi.fn() });
    vi.mocked(notificationApi.listPublic).mockResolvedValue({ items: [publicItem], pagination: { total: 1, limit: 10, offset: 0 } });
  });

  it("loads the public feed without waiting for authentication", async () => {
    renderPage();
    expect(await screen.findByText(publicItem.title)).toBeInTheDocument();
    expect(notificationApi.listPublic).toHaveBeenCalledWith({ limit: 10, offset: 0, bbox: undefined, signal: expect.any(AbortSignal) });
    expect(notificationApi.list).not.toHaveBeenCalled();
    expect(notificationApi.getUnreadCount).not.toHaveBeenCalled();
    expect(screen.queryByRole("link", { name: /เข้าสู่ระบบ/ })).not.toBeInTheDocument();
  });

  it("keeps the same public feed for authenticated sessions", async () => {
    vi.mocked(useOptionalAuth).mockReturnValue({ account, status: "authenticated", login: vi.fn(), logout: vi.fn(), register: vi.fn(), updateUsername: vi.fn() });
    renderPage();
    expect(await screen.findByText(publicItem.title)).toBeInTheDocument();
    expect(notificationApi.listPublic).toHaveBeenCalledTimes(1);
    expect(notificationApi.list).not.toHaveBeenCalled();
  });

  it("stores public read state in the browser", async () => {
    renderPage();
    fireEvent.click(await screen.findByRole("button", { name: `ทำเครื่องหมายว่าอ่านแล้ว: ${publicItem.title}` }));
    await waitFor(() => expect(localStorage.getItem("floodwatch.public-notifications.read")).toContain(publicItem.id));
    expect(screen.getByText("อ่านแล้ว")).toBeInTheDocument();
  });

  it("restores the selected area from safe browser storage", async () => {
    localStorage.setItem("floodwatch.public-notifications.area", "100,13,102,15");
    renderPage();
    expect(await screen.findByText(publicItem.title)).toBeInTheDocument();
    expect(screen.getByLabelText("ขอบเขตพื้นที่บนแผนที่")).toHaveValue("100,13,102,15");
    expect(notificationApi.listPublic).toHaveBeenCalledWith({ limit: 10, offset: 0, bbox: "100,13,102,15", signal: expect.any(AbortSignal) });
  });

  it("marks the initial public feed error as an alert", async () => {
    vi.mocked(notificationApi.listPublic).mockRejectedValueOnce(new Error("unavailable"));
    renderPage();
    expect(await screen.findByRole("alert")).toBeInTheDocument();
  });

  it("does not keep an old area's results when loading the new area fails", async () => {
    vi.mocked(notificationApi.listPublic)
      .mockResolvedValueOnce({ items: [publicItem], pagination: { total: 1, limit: 10, offset: 0 } })
      .mockRejectedValueOnce(new Error("unavailable"))
      .mockResolvedValueOnce({ items: [publicItem], pagination: { total: 1, limit: 10, offset: 0 } });
    renderPage();
    await screen.findByText(publicItem.title);
    fireEvent.change(screen.getByLabelText("ขอบเขตพื้นที่บนแผนที่"), { target: { value: "102,15,103,16" } });
    fireEvent.click(screen.getByRole("button", { name: "ใช้พื้นที่นี้" }));
    expect(await screen.findByRole("alert")).toBeInTheDocument();
    expect(screen.queryByText(publicItem.title)).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "ลองอีกครั้ง" }));
    expect(await screen.findByText(publicItem.title)).toBeInTheDocument();
  });

  it("keeps the area controls available when the selected bbox is empty and can clear it", async () => {
    vi.mocked(notificationApi.listPublic)
      .mockResolvedValueOnce({ items: [publicItem], pagination: { total: 1, limit: 10, offset: 0 } })
      .mockResolvedValueOnce({ items: [], pagination: { total: 0, limit: 10, offset: 0 } })
      .mockResolvedValueOnce({ items: [publicItem], pagination: { total: 1, limit: 10, offset: 0 } });
    renderPage();
    await screen.findByText(publicItem.title);
    fireEvent.change(screen.getByLabelText("ขอบเขตพื้นที่บนแผนที่"), { target: { value: "102,15,103,16" } });
    fireEvent.click(screen.getByRole("button", { name: "ใช้พื้นที่นี้" }));
    expect(await screen.findByRole("heading", { name: "ยังไม่มีการแจ้งเตือนสาธารณะ" })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "ล้างพื้นที่" }));
    await waitFor(() => expect(screen.getByText(publicItem.title)).toBeInTheDocument());
  });

  it("refetches the public feed when alert or verified-report revisions change", async () => {
    vi.mocked(notificationApi.listPublic)
      .mockResolvedValueOnce({ items: [publicItem], pagination: { total: 1, limit: 10, offset: 0 } })
      .mockResolvedValueOnce({ items: [{ ...publicItem, id: "alert:public-2", title: "New alert" }], pagination: { total: 1, limit: 10, offset: 0 } });
    const view = renderPage();
    await screen.findByText(publicItem.title);
    realtime.revision = 1;
    view.rerender(<MemoryRouter><NotificationsPage /></MemoryRouter>);
    await waitFor(() => expect(notificationApi.listPublic).toHaveBeenCalledTimes(2));
    expect(await screen.findByText("New alert")).toBeInTheDocument();
  });
});
