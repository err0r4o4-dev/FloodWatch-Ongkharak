import { fireEvent, render, screen } from "@testing-library/react";
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
  it.each([
    ["/announcements", "ประกาศ", "ยังไม่มีประกาศที่เผยแพร่"],
    ["/cctv", "กล้อง CCTV", "ยังไม่มีกล้องที่เผยแพร่"],
    ["/reports", "รายงานสถานการณ์", "ยังไม่มีรายงานที่เผยแพร่"],
    ["/map", "แผนที่สถานการณ์", "ข้อมูลในขอบเขตที่โหลด"]
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
    expect(await screen.findByRole("heading", { level: 1, name: "ประกาศ" })).toBeInTheDocument();
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
