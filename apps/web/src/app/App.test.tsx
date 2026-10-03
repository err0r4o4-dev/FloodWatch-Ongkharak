import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { afterEach, vi } from "vitest";
import { AuthProvider } from "../features/auth/AuthProvider";
import { AppRoutes } from "./AppRoutes";

afterEach(() => { vi.unstubAllGlobals(); vi.restoreAllMocks(); });

describe("AppRoutes", () => {
  it("shows the development placeholder at its bookmarkable route", () => {
    render(
      <MemoryRouter initialEntries={["/system-status"]}>
        <AppRoutes />
      </MemoryRouter>
    );

    expect(screen.getByRole("link", { name: "สถานะระบบ" })).toHaveAttribute("href", "/system-status");
    expect(
      screen.getByRole("heading", { name: "สถานะระบบสำหรับการพัฒนา" })
    ).toBeInTheDocument();
    expect(screen.getByText("ยังไม่มีการเผยแพร่ข้อมูลสถานการณ์น้ำ")).toBeInTheDocument();
  });
});

describe("public routes", () => {
  it("keeps the scroll region full width while constraining page content", () => {
    render(<MemoryRouter initialEntries={["/system-status"]}><AppRoutes /></MemoryRouter>);
    const main = screen.getByRole("main");
    expect(main).toHaveClass("page-content");
    expect(main).not.toHaveClass("max-w-[1600px]");
    expect(main.firstElementChild).toHaveClass("page-content__inner", "max-w-[1600px]");
  });

  it.each(["/login", "/register", "/forgot-password"])("keeps a main landmark on %s", async (path) => {
    const api = {
      refresh: vi.fn().mockRejectedValue(new Error("anonymous")),
      me: vi.fn(), logout: vi.fn(), login: vi.fn(), register: vi.fn()
    };
    render(<MemoryRouter initialEntries={[path]}><AuthProvider api={api}><AppRoutes /></AuthProvider></MemoryRouter>);
    await act(async () => { await api.refresh.mock.results[0]?.value.catch(() => undefined); });
    expect(screen.getByRole("main")).toBeInTheDocument();
  });

  it("shows the institutional header and marks the pending language action unavailable", () => {
    render(<MemoryRouter initialEntries={["/system-status"]}><AppRoutes /></MemoryRouter>);
    expect(screen.getByText("ศูนย์ติดตามสถานการณ์น้ำ อำเภอองครักษ์ จังหวัดนครนายก")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "EN" })).toBeDisabled();
    expect(screen.getByRole("link", { name: "ตั้งค่าการแจ้งเตือน" })).toHaveAttribute("href", "/#notification-settings");
    fireEvent.click(screen.getByLabelText("เมนูบัญชี"));
    expect(screen.getByRole("link", { name: "เข้าสู่ระบบ" })).toHaveAttribute("href", "/login");
  });

  it("links to risk areas from both navigation surfaces without CCTV", () => {
    render(<MemoryRouter initialEntries={["/system-status"]}><AppRoutes /></MemoryRouter>);
    expect(screen.getAllByRole("link", { name: "พื้นที่เสี่ยง" })).toHaveLength(2);
    expect(screen.queryByRole("link", { name: /CCTV/ })).not.toBeInTheDocument();
  });

  it("uses consistent navigation icons and keeps the data caution in a sidebar card", () => {
    render(<MemoryRouter initialEntries={["/system-status"]}><AppRoutes /></MemoryRouter>);
    const navigation = screen.getByRole("navigation", { name: "การนำทางหลัก" });
    for (const link of within(navigation).getAllByRole("link")) {
      expect(link.querySelector("svg")).toHaveAttribute("width", "20");
      expect(link.querySelector("svg")).toHaveAttribute("stroke-width", "2");
    }
    const caution = screen.getByRole("complementary", { name: "ข้อควรทราบ" });
    expect(within(caution).getByText("ข้อมูลไม่พร้อมใช้งานไม่ได้หมายถึงพื้นที่ปลอดภัย")).toBeInTheDocument();
  });

  it("keeps five mobile destinations for an admin and exposes admin from the account menu", async () => {
    const account = { id: "admin-1", email: "admin@example.com", role: "ADMIN" as const, createdAt: "2026-10-02T00:00:00Z" };
    const api = { refresh: vi.fn().mockResolvedValue(undefined), me: vi.fn().mockResolvedValue(account), logout: vi.fn(), login: vi.fn(), register: vi.fn() };
    render(<MemoryRouter initialEntries={["/system-status"]}><AuthProvider api={api}><AppRoutes /></AuthProvider></MemoryRouter>);
    await waitFor(() => expect(within(screen.getByRole("navigation", { name: "การนำทางหลัก" })).getByRole("link", { name: "จัดการระบบ" })).toBeInTheDocument());
    expect(within(screen.getByRole("navigation", { name: "การนำทางมือถือ" })).getAllByRole("link")).toHaveLength(5);
    fireEvent.click(screen.getByLabelText("เมนูบัญชี"));
    expect(screen.getAllByRole("link", { name: "จัดการระบบ" })).toHaveLength(2);
  });

  it("moves focus to notification settings after following the header link", async () => {
    render(<MemoryRouter initialEntries={["/system-status"]}><AppRoutes /></MemoryRouter>);
    fireEvent.click(screen.getByRole("link", { name: "ตั้งค่าการแจ้งเตือน" }));
    expect(await screen.findByRole("heading", { name: "ภาพรวมสถานการณ์น้ำ" })).toBeInTheDocument();
    await waitFor(() => expect(document.getElementById("notification-settings")).toHaveFocus());
  });

  it("keeps the situation map free of camera controls", () => {
    render(<MemoryRouter initialEntries={["/map"]}><AppRoutes /></MemoryRouter>);
    expect(screen.queryByText(/CCTV/i)).not.toBeInTheDocument();
  });

  it.each([
    ["/announcements", "ประกาศและแจ้งเตือน", "ยังไม่มีประกาศที่เผยแพร่"],
    ["/reports", "รายงานสถานการณ์", "ยังไม่มีรายงานที่เผยแพร่"],
    ["/map", "แผนที่สถานการณ์น้ำ", "ข้อมูลในขอบเขตที่โหลด"]
  ])("opens %s without login", async (path, heading, empty) => {
    if (path === "/reports" || path === "/announcements") {
      vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: true, status: 200, json: () => Promise.resolve({ items: [], limit: 20, offset: 0 }) }));
    }
    render(<MemoryRouter initialEntries={[path]}><AppRoutes /></MemoryRouter>);
    expect(screen.getByRole("heading", { level: 1, name: heading })).toBeInTheDocument();
    expect(await screen.findByText(empty)).toBeInTheDocument();
  });

  it("retains guest access while auth refresh fails", async () => {
    const api = {
      refresh: vi.fn().mockRejectedValue(new Error("unauthenticated")),
      me: vi.fn(), logout: vi.fn(), login: vi.fn(), register: vi.fn()
    };
    render(<MemoryRouter initialEntries={["/announcements"]}><AuthProvider api={api}><AppRoutes /></AuthProvider></MemoryRouter>);
    expect(await screen.findByRole("heading", { level: 1, name: "ประกาศและแจ้งเตือน" })).toBeInTheDocument();
    expect(api.me).not.toHaveBeenCalled();
  });

  it("provides desktop and mobile navigation with active state and touch targets", () => {
    render(<MemoryRouter initialEntries={["/map"]}><AppRoutes /></MemoryRouter>);
    expect(screen.getByRole("navigation", { name: "การนำทางหลัก" })).toBeInTheDocument();
    expect(screen.getByRole("navigation", { name: "การนำทางมือถือ" })).toBeInTheDocument();
    expect(screen.getAllByRole("link", { name: "แผนที่" })).toHaveLength(2);
    expect(screen.getAllByRole("link", { name: "แผนที่" })[0]).toHaveAttribute("aria-current", "page");
    fireEvent.click(screen.getByRole("button", { name: "เส้นทาง" }));
    expect(screen.getByText(/ยังไม่มีบริการคำนวณเส้นทาง/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "เส้นทาง" })).toHaveAttribute("aria-pressed", "true");
  });
});
