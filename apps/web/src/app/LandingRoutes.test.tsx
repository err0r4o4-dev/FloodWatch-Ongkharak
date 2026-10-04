import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { AuthProvider } from "../features/auth/AuthProvider";
import { AppRoutes } from "./AppRoutes";
import { LanguageProvider } from "../i18n";
import type { MapBounds } from "../features/map/types";

vi.mock("../features/map/components/SituationMap", async () => {
  const React = await import("react");
  return { SituationMap: ({ onBoundsChange }: { onBoundsChange: (bounds: MapBounds) => void }) => {
    React.useEffect(() => onBoundsChange({ west: 100.9, south: 14, east: 101.2, north: 14.3 }), [onBoundsChange]);
    return <div aria-label="แผนที่สถานการณ์แบบโต้ตอบ"><button type="button" onClick={() => onBoundsChange({ west: 100.9, south: 14, east: 101.2, north: 14.3 })}>เผยแพร่ขอบเขตเดิมอีกครั้ง</button></div>;
  } };
});

beforeEach(() => {
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: true, status: 200, json: async () => ({ items: [], limit: 20, offset: 0 }) }));
});
afterEach(() => { vi.unstubAllGlobals(); vi.restoreAllMocks(); });

describe("landing and overview navigation", () => {
  it("matches the reference content with four goals, language availability and no scroll-down action", () => {
    render(<MemoryRouter><AppRoutes /></MemoryRouter>);
    expect(screen.getByRole("heading", { level: 1, name: "ติดตามสถานการณ์น้ำ เพื่อความปลอดภัยของทุกคน" })).toBeInTheDocument();
    expect(screen.getByRole("main")).toHaveAttribute("id", "main-content");
    expect(screen.queryByRole("navigation", { name: "การนำทางหลัก" })).not.toBeInTheDocument();
    expect(screen.queryByRole("navigation", { name: "การนำทางมือถือ" })).not.toBeInTheDocument();
    const goals = screen.getByRole("region", { name: "สร้างชุมชนที่ปลอดภัย พร้อมรับมือสถานการณ์น้ำ" });
    expect(within(goals).getAllByRole("heading", { level: 3 }).map((heading) => heading.textContent)).toEqual(["เชื่อถือได้", "เข้าถึงง่าย", "ใช้เทคโนโลยี AI", "ลดความเสี่ยง"]);
    expect(screen.getByRole("button", { name: "EN" })).toBeEnabled();
    expect(screen.getByRole("button", { name: "TH" })).toHaveAttribute("aria-pressed", "true");
    expect(screen.queryByRole("link", { name: /รู้จักระบบ|เลื่อนลง|เป้าหมายของเรา/ })).not.toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: /3 ขั้นตอน/ })).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "ดูภาพรวมโดยไม่เข้าสู่ระบบ" })).not.toBeInTheDocument();
    expect(screen.getByText("ตัวอย่างสภาพอากาศ")).toBeInTheDocument();
    expect(screen.queryByRole("group", { name: "ชั้นข้อมูลที่แสดง" })).not.toBeInTheDocument();
    expect(screen.getByText("28°C")).toBeInTheDocument();
    expect(screen.getByText("ตัวอย่างการแสดงผล · ไม่ใช่ข้อมูลสถานการณ์จริง")).toBeInTheDocument();
  });

  it("opens the original overview without login and preserves navigation and brand return", async () => {
    render(<MemoryRouter><AppRoutes /></MemoryRouter>);
    fireEvent.click(screen.getByRole("link", { name: "เข้าสู่เมนู" }));
    expect(await screen.findByRole("heading", { level: 1, name: "ภาพรวมสถานการณ์น้ำ" })).toBeInTheDocument();
    const navigation = screen.getByRole("navigation", { name: "การนำทางหลัก" });
    expect(within(navigation).getByRole("link", { name: "ภาพรวม" })).toHaveAttribute("href", "/overview");
    expect(within(navigation).getByRole("link", { name: "ภาพรวม" })).toHaveAttribute("aria-current", "page");
    expect(within(screen.getByRole("navigation", { name: "การนำทางมือถือ" })).getAllByRole("link")).toHaveLength(4);
    fireEvent.click(screen.getAllByRole("link", { name: "Flood Watch Ongkharak หน้าหลัก" })[0]);
    expect(screen.getByRole("heading", { level: 1, name: "ติดตามสถานการณ์น้ำ เพื่อความปลอดภัยของทุกคน" })).toBeInTheDocument();
  });

  it("keeps overview bookmarks and original application destinations", () => {
    render(<MemoryRouter initialEntries={["/overview"]}><AppRoutes /></MemoryRouter>);
    expect(screen.getByRole("heading", { level: 1, name: "ภาพรวมสถานการณ์น้ำ" })).toBeInTheDocument();
    const navigation = screen.getByRole("navigation", { name: "การนำทางหลัก" });
    for (const [name, path] of [["ประกาศ", "/announcements"], ["แผนที่และพื้นที่เสี่ยง", "/map"], ["รายงาน", "/reports"]]) {
      expect(within(navigation).getByRole("link", { name })).toHaveAttribute("href", path);
    }
  });

  it("offers direct and expanded map access without public auth entry links", () => {
    render(<MemoryRouter><AppRoutes /></MemoryRouter>);
    expect(screen.getByRole("link", { name: "ดูแผนที่" })).toHaveAttribute("href", "/map");
    expect(screen.getByRole("link", { name: "เปิดแผนที่เต็มหน้า" })).toHaveAttribute("href", "/map");
    expect(screen.queryByRole("link", { name: "เข้าสู่ระบบ" })).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "สมัครสมาชิก" })).not.toBeInTheDocument();
    expect(screen.getByRole("link", { name: "เข้าสู่เมนู" })).toHaveAttribute("href", "/overview");
    expect(within(screen.getByRole("navigation", { name: "การนำทางหน้าแรก" })).queryByRole("link", { name: "เข้าสู่เมนู" })).not.toBeInTheDocument();
  });

  it.each(["USER", "ADMIN"] as const)("keeps public auth entry links out of the landing header for authenticated %s users", async (role) => {
    const account = { id: "member-1", email: "member@example.test", role, createdAt: "2026-10-02T00:00:00Z" };
    const api = { refresh: vi.fn().mockResolvedValue(undefined), me: vi.fn().mockResolvedValue(account), logout: vi.fn(), login: vi.fn(), register: vi.fn() };
    render(<MemoryRouter><AuthProvider api={api}><AppRoutes /></AuthProvider></MemoryRouter>);
    await waitFor(() => expect(screen.getAllByRole("link", { name: "เข้าสู่เมนู" })).toHaveLength(1));
    const header = screen.getByRole("navigation", { name: "การนำทางหน้าแรก" });
    expect(within(header).queryByRole("link", { name: "เข้าสู่ระบบ" })).not.toBeInTheDocument();
    expect(within(header).queryByRole("link", { name: "สมัครสมาชิก" })).not.toBeInTheDocument();
    expect(screen.getAllByRole("link", { name: "เข้าสู่เมนู" }).every((link) => link.getAttribute("href") === "/overview")).toBe(true);
  });

  it.each(["unavailable", "pending"])("shows illustrative map and weather without requesting %s APIs", (state) => {
    vi.stubGlobal("fetch", state === "unavailable" ? vi.fn().mockRejectedValue(new Error("offline")) : vi.fn(() => new Promise(() => {})));
    render(<MemoryRouter><AppRoutes /></MemoryRouter>);
    expect(screen.getByRole("img", { name: "แผนที่ตัวอย่าง" })).toBeInTheDocument();
    expect(screen.getByText("ตัวอย่างสภาพอากาศ")).toBeInTheDocument();
    expect(screen.getByText("28°C")).toBeInTheDocument();
    expect(fetch).not.toHaveBeenCalled();
    expect(screen.queryByRole("button", { name: /ขยายแผนที่|ย่อแผนที่|ลองอีกครั้ง/ })).not.toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    expect(screen.queryByRole("region", { name: "ข้อมูลตำแหน่งที่เลือก" })).not.toBeInTheDocument();
  });

  it("keeps the real map destination requesting its existing APIs", async () => {
    render(<MemoryRouter><AppRoutes /></MemoryRouter>);
    expect(fetch).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("link", { name: "เปิดแผนที่เต็มหน้า" }));
    expect(await screen.findByRole("heading", { level: 1, name: "แผนที่สถานการณ์น้ำ" })).toBeInTheDocument();
    await waitFor(() => expect(vi.mocked(fetch).mock.calls.map(([url]) => String(url))).toEqual(expect.arrayContaining([expect.stringContaining("/risk-areas?bbox="), expect.stringContaining("/flood-events?bbox=")])));
  });

  it("localizes the demonstration labels and embeds no remote map assets", () => {
    render(<LanguageProvider><MemoryRouter><AppRoutes /></MemoryRouter></LanguageProvider>);
    const map = screen.getByRole("img", { name: "แผนที่ตัวอย่าง" });
    expect(map).toHaveAccessibleDescription(/ไม่ใช่แผนที่อ้างอิงตำแหน่งหรือสถานการณ์จริง/);
    expect(map.querySelector("image, foreignObject, a")).toBeNull();
    expect(document.querySelector(".landing-map .leaflet-map, .landing-map iframe")).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: /^EN$/ }));
    expect(screen.getByRole("img", { name: "Example map" })).toBeInTheDocument();
    expect(screen.getByText("Display example · Not actual situation data")).toBeInTheDocument();
    expect(screen.getByText("Example weather")).toBeInTheDocument();
    expect(screen.getByText("UI demo values · Not current weather")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Open full map" })).toHaveAttribute("href", "/map");
    fireEvent.click(screen.getByRole("button", { name: /^TH$/ }));
  });
});
