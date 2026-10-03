import { fireEvent, render, screen, within } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { afterEach, describe, expect, it, vi } from "vitest";
import { AuthProvider } from "../features/auth/AuthProvider";
import { AppRoutes } from "./AppRoutes";

afterEach(() => { vi.unstubAllGlobals(); vi.restoreAllMocks(); });

function emptyPublicFeeds() {
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: true, status: 200, json: async () => ({ items: [], limit: 20, offset: 0 }) }));
}

describe("landing and overview navigation", () => {
  it("shows an introductory home without application navigation or live-data requests", () => {
    const fetch = vi.fn();
    vi.stubGlobal("fetch", fetch);
    render(<MemoryRouter><AppRoutes /></MemoryRouter>);
    expect(screen.getByRole("heading", { level: 1, name: "ติดตามสถานการณ์น้ำ ก่อนออกเดินทาง" })).toBeInTheDocument();
    expect(screen.getByRole("main")).toHaveAttribute("id", "main-content");
    expect(screen.queryByRole("navigation", { name: "การนำทางหลัก" })).not.toBeInTheDocument();
    expect(screen.queryByRole("navigation", { name: "การนำทางมือถือ" })).not.toBeInTheDocument();
    expect(screen.getByText("ภาพประกอบการใช้งาน ไม่ใช่ข้อมูลสถานการณ์น้ำจริง")).toBeInTheDocument();
    expect(screen.getByText("ข้อมูลไม่พร้อมใช้งานไม่ได้หมายถึงพื้นที่ปลอดภัย")).toBeInTheDocument();
    expect(fetch).not.toHaveBeenCalled();
  });

  it("opens the original overview from the main call to action without login", async () => {
    emptyPublicFeeds();
    render(<MemoryRouter><AppRoutes /></MemoryRouter>);
    fireEvent.click(within(screen.getByRole("main")).getByRole("link", { name: "เข้าสู่เมนู" }));
    expect(await screen.findByRole("heading", { level: 1, name: "ภาพรวมสถานการณ์น้ำ" })).toBeInTheDocument();
    const navigation = screen.getByRole("navigation", { name: "การนำทางหลัก" });
    expect(within(navigation).getByRole("link", { name: "ภาพรวม" })).toHaveAttribute("href", "/overview");
    expect(within(navigation).getByRole("link", { name: "ภาพรวม" })).toHaveAttribute("aria-current", "page");
    expect(within(screen.getByRole("navigation", { name: "การนำทางมือถือ" })).getAllByRole("link")).toHaveLength(4);
    fireEvent.click(screen.getAllByRole("link", { name: "Flood Watch Ongkharak หน้าหลัก" })[0]);
    expect(screen.getByRole("heading", { level: 1, name: "ติดตามสถานการณ์น้ำ ก่อนออกเดินทาง" })).toBeInTheDocument();
  });

  it("keeps overview bookmarks and original application destinations", () => {
    emptyPublicFeeds();
    render(<MemoryRouter initialEntries={["/overview"]}><AppRoutes /></MemoryRouter>);
    expect(screen.getByRole("heading", { level: 1, name: "ภาพรวมสถานการณ์น้ำ" })).toBeInTheDocument();
    const navigation = screen.getByRole("navigation", { name: "การนำทางหลัก" });
    for (const [name, path] of [["ประกาศ", "/announcements"], ["แผนที่และพื้นที่เสี่ยง", "/map"], ["รายงาน", "/reports"]]) {
      expect(within(navigation).getByRole("link", { name })).toHaveAttribute("href", path);
    }
  });

  it("offers direct map access, an in-page introduction and a login return to overview", () => {
    render(<MemoryRouter><AppRoutes /></MemoryRouter>);
    expect(screen.getByRole("link", { name: "ดูแผนที่" })).toHaveAttribute("href", "/map");
    expect(screen.getByRole("link", { name: "รู้จักระบบ" })).toHaveAttribute("href", "#how-it-works");
    expect(screen.getByRole("heading", { name: "เริ่มติดตามสถานการณ์ได้ใน 3 ขั้นตอน" }).closest("section")).toHaveAttribute("id", "how-it-works");
    expect(screen.getByRole("link", { name: "เข้าสู่ระบบ" })).toHaveAttribute("href", "/login?returnTo=%2Foverview");
  });

  it("does not offer sign in again to an authenticated member", async () => {
    const account = { id: "member-1", email: "member@example.test", role: "USER" as const, createdAt: "2026-10-02T00:00:00Z" };
    const api = { refresh: vi.fn().mockResolvedValue(undefined), me: vi.fn().mockResolvedValue(account), logout: vi.fn(), login: vi.fn(), register: vi.fn() };
    render(<MemoryRouter><AuthProvider api={api}><AppRoutes /></AuthProvider></MemoryRouter>);
    expect(await screen.findByRole("link", { name: "หน้ารายงาน" })).toHaveAttribute("href", "/reports");
    expect(screen.queryByRole("link", { name: "เข้าสู่ระบบ" })).not.toBeInTheDocument();
    expect(screen.getAllByRole("link", { name: "เข้าสู่เมนู" }).every((link) => link.getAttribute("href") === "/overview")).toBe(true);
  });
});
