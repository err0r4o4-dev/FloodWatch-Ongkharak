import { act, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { manualAnnouncementApi } from "./api";
import { ManualAnnouncementsPanel } from "./ManualAnnouncementsPanel";
vi.mock("./api", () => ({ manualAnnouncementApi: { listPublic: vi.fn() } }));
beforeEach(() => { vi.mocked(manualAnnouncementApi.listPublic).mockReset(); });
afterEach(() => vi.useRealTimers());
describe("public manual announcement panel", () => {
  it("distinguishes no announcements from safety", async () => {
    vi.mocked(manualAnnouncementApi.listPublic).mockResolvedValue({ items: [], total: 0, limit: 20, offset: 0 });
    render(<ManualAnnouncementsPanel />);
    expect(await screen.findByText("ยังไม่มีประกาศจากผู้ดูแล")).toBeInTheDocument();
    expect(screen.getByText("การไม่มีประกาศไม่ได้ยืนยันว่าพื้นที่หรือถนนปลอดภัย")).toBeInTheDocument();
  });
  it("shows errors and allows retry", async () => {
    vi.mocked(manualAnnouncementApi.listPublic).mockRejectedValueOnce(new Error("offline")).mockResolvedValueOnce({ items: [], total: 0, limit: 20, offset: 0 });
    render(<ManualAnnouncementsPanel />);
    expect(await screen.findByText("ไม่สามารถโหลดประกาศจากผู้ดูแลได้")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "ลองโหลดประกาศจากผู้ดูแลอีกครั้ง" }));
    expect(await screen.findByText("ยังไม่มีประกาศจากผู้ดูแล")).toBeInTheDocument();
  });
  it("bounds stalled requests", async () => {
    vi.useFakeTimers();
    vi.mocked(manualAnnouncementApi.listPublic).mockImplementation(() => new Promise(() => {}));
    render(<ManualAnnouncementsPanel />);
    await act(() => vi.advanceTimersByTimeAsync(10001));
    expect(screen.getByText("ไม่สามารถโหลดประกาศจากผู้ดูแลได้")).toBeInTheDocument();
  });
});
