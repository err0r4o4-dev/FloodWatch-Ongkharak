import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { AuthProvider } from "../features/auth/AuthProvider";
import { AppRoutes } from "./AppRoutes";
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
    expect(screen.getByRole("button", { name: "EN" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "EN" })).toHaveAccessibleDescription("ภาษาอังกฤษยังไม่พร้อมใช้งาน");
    expect(screen.queryByRole("link", { name: /รู้จักระบบ|เลื่อนลง|เป้าหมายของเรา/ })).not.toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: /3 ขั้นตอน/ })).not.toBeInTheDocument();
    expect(screen.getByText("ข้อมูลสภาพอากาศยังไม่พร้อมใช้งาน")).toBeInTheDocument();
    expect(screen.queryByText(/25°C|ฝนเล็กน้อย/)).not.toBeInTheDocument();
  });

  it("opens the original overview without login and preserves navigation and brand return", async () => {
    render(<MemoryRouter><AppRoutes /></MemoryRouter>);
    fireEvent.click(screen.getByRole("link", { name: "ดูภาพรวมโดยไม่เข้าสู่ระบบ" }));
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

  it("offers direct and expanded map access and both login returns to overview", () => {
    render(<MemoryRouter><AppRoutes /></MemoryRouter>);
    expect(screen.getByRole("link", { name: "ดูแผนที่" })).toHaveAttribute("href", "/map");
    expect(screen.getByRole("link", { name: "เปิดแผนที่เต็มหน้า" })).toHaveAttribute("href", "/map");
    const signIn = screen.getByRole("link", { name: "เข้าสู่ระบบ" });
    expect(signIn).toHaveAttribute("href", "/login?returnTo=%2Foverview");
    expect(signIn).toHaveTextContent(/^เข้าสู่ระบบ$/);
    expect(screen.getByRole("link", { name: "เข้าสู่เมนู" })).toHaveAttribute("href", "/overview");
    expect(within(screen.getByRole("navigation", { name: "การนำทางหน้าแรก" })).queryByRole("link", { name: "เข้าสู่เมนู" })).not.toBeInTheDocument();
  });

  it.each(["USER", "ADMIN"] as const)("keeps the header sign-in label and authenticated %s menu destination", async (role) => {
    const account = { id: "member-1", email: "member@example.test", role, createdAt: "2026-10-02T00:00:00Z" };
    const api = { refresh: vi.fn().mockResolvedValue(undefined), me: vi.fn().mockResolvedValue(account), logout: vi.fn(), login: vi.fn(), register: vi.fn() };
    render(<MemoryRouter><AuthProvider api={api}><AppRoutes /></AuthProvider></MemoryRouter>);
    await waitFor(() => expect(screen.getAllByRole("link", { name: "เข้าสู่เมนู" })).toHaveLength(1));
    const header = screen.getByRole("navigation", { name: "การนำทางหน้าแรก" });
    expect(within(header).getByRole("link", { name: "เข้าสู่ระบบ" })).toHaveAttribute("href", "/overview");
    expect(within(header).getByRole("link", { name: "เข้าสู่ระบบ" })).toHaveTextContent(/^เข้าสู่ระบบ$/);
    expect(screen.getAllByRole("link", { name: "เข้าสู่เมนู" }).every((link) => link.getAttribute("href") === "/overview")).toBe(true);
  });

  it("loads existing map APIs and never represents an empty map as safe", async () => {
    render(<MemoryRouter><AppRoutes /></MemoryRouter>);
    expect(await screen.findByText("ยังไม่มีข้อมูลสถานการณ์ในขอบเขตนี้ · ไม่ทราบสถานะ")).toBeInTheDocument();
    expect(vi.mocked(fetch).mock.calls.map(([url]) => String(url))).toEqual(expect.arrayContaining([expect.stringContaining("/risk-areas?bbox="), expect.stringContaining("/flood-events?bbox=")]));
    expect(screen.getByText("ไม่ทราบสถานะ")).toBeInTheDocument();
    expect(screen.queryByText("ปลอดภัย")).not.toBeInTheDocument();
  });

  it("discloses failed layers and recovers with retry without inventing observations", async () => {
    const request = vi.fn().mockRejectedValue(new Error("offline"));
    vi.stubGlobal("fetch", request);
    render(<MemoryRouter><AppRoutes /></MemoryRouter>);
    expect(await screen.findByRole("alert")).toHaveTextContent("ไม่สามารถโหลดข้อมูลแผนที่ได้ · สถานะไม่ทราบ");
    request.mockResolvedValue({ ok: true, status: 200, json: async () => ({ items: [] }) });
    fireEvent.click(screen.getByRole("button", { name: "ลองอีกครั้ง" }));
    expect(await screen.findByText("ยังไม่มีข้อมูลสถานการณ์ในขอบเขตนี้ · ไม่ทราบสถานะ")).toBeInTheDocument();
    expect(request).toHaveBeenCalledTimes(10);
  });

  it("discloses a partial failure even if the other collections are empty", async () => {
    vi.stubGlobal("fetch", vi.fn((url: string) => url.includes("/risk-areas?") ? Promise.reject(new Error("offline")) : Promise.resolve({ ok: true, json: async () => ({ items: [] }) })));
    render(<MemoryRouter><AppRoutes /></MemoryRouter>);
    expect(await screen.findByText("ข้อมูลบางชั้นไม่พร้อมใช้งาน · ไม่สามารถสรุปว่าปลอดภัย")).toBeInTheDocument();
    expect(screen.queryByText("ยังไม่มีข้อมูลสถานการณ์ในขอบเขตนี้ · ไม่ทราบสถานะ")).not.toBeInTheDocument();
  });

  it("offers a keyboard-accessible record with source, timestamps and stale UNKNOWN status", async () => {
    const risk = { id: "risk-test", name: "พื้นที่สังเคราะห์สำหรับทดสอบ", description: "ไม่ใช่ข้อมูลจริง", geometry: { type: "MultiPolygon", coordinates: [[[[101, 14], [101.1, 14], [101.1, 14.1], [101, 14]]]] }, source: { id: "source-test", name: "ชุดทดสอบเท่านั้น", sourceType: "AI" }, riskLevel: "HIGH", effectiveRiskLevel: "HIGH", freshness: "STALE", validFrom: "2026-10-02T00:00:00Z", validTo: null };
    vi.stubGlobal("fetch", vi.fn((url: string) => Promise.resolve({ ok: true, json: async () => ({ items: url.includes("/risk-areas?") ? [risk] : [] }) })));
    render(<MemoryRouter><AppRoutes /></MemoryRouter>);
    fireEvent.click(await screen.findByRole("button", { name: risk.name }));
    const details = screen.getByRole("region", { name: "ข้อมูลตำแหน่งที่เลือก" });
    expect(within(details).getByText("ไม่ทราบสถานะ")).toBeInTheDocument();
    expect(within(details).getByText("ข้อมูลเก่า")).toBeInTheDocument();
    expect(within(details).getByText("หลักฐาน AI")).toBeInTheDocument();
    expect(within(details).getByText("แหล่งข้อมูล: ชุดทดสอบเท่านั้น")).toBeInTheDocument();
    expect(details.querySelector("time")).toHaveAttribute("datetime", risk.validFrom);
    fireEvent.click(screen.getByRole("button", { name: "เผยแพร่ขอบเขตเดิมอีกครั้ง" }));
    expect(screen.getByRole("region", { name: "ข้อมูลตำแหน่งที่เลือก" })).toBeInTheDocument();
    const close = within(details).getByRole("button", { name: "ปิดข้อมูลตำแหน่ง" });
    expect(close).toHaveFocus();
    fireEvent.keyDown(close, { key: "Escape" });
    expect(screen.queryByRole("region", { name: "ข้อมูลตำแหน่งที่เลือก" })).not.toBeInTheDocument();
  });
});
