import { act, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { AnnouncementCard } from "./AnnouncementCard";
import type { ManualAnnouncement } from "./api";
const announcement: ManualAnnouncement = {
  id: "123e4567-e89b-12d3-a456-426614174000", title: "ประกาศสำหรับทดสอบ", summary: "สรุปหลักฐานทดสอบ", body: "รายละเอียดหลักฐานทดสอบ", advice: "คำแนะนำจากรายงานทดสอบ", area: "พื้นที่ทดสอบ", severity: "WARNING",
  sourceType: "COMMUNITY", publisherType: "ADMIN", referenceKind: "MEDIA", status: "PUBLISHED", effectiveStatus: "WARNING",
  observedAt: "2026-10-04T14:00:00Z", expiresAt: "2026-10-04T16:00:00Z", references: [{ name: "สำนักข่าวทดสอบ", url: "https://news.example.com/article", observedAt: "2026-10-04T13:00:00Z" }],
  createdAt: "2026-10-04T14:01:00Z", updatedAt: "2026-10-04T14:02:00Z", publishedAt: "2026-10-04T14:02:00Z", withdrawnAt: null, withdrawalReason: "", revision: 2
};
afterEach(() => vi.useRealTimers());
describe("source-aware manual announcement card", () => {
  it("separates publication from source observations", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-10-04T15:00:00Z"));
    render(<AnnouncementCard announcement={announcement} />);
    expect(screen.getByText("ประกาศโดยผู้ดูแล · อ้างอิงข่าวจากสื่อ")).toBeInTheDocument();
    expect(screen.getByText(/ไม่ใช่ประกาศปิดถนนจากหน่วยงานทางหลวง/)).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "สำนักข่าวทดสอบ" })).toHaveAttribute("rel", "noopener noreferrer");
    expect(screen.getAllByText(/เวลาที่แหล่งข่าวรายงาน/)).toHaveLength(1);
    expect(screen.getByText("เตือนภัย")).toBeInTheDocument();
  });
  it("expires to UNKNOWN while the page remains open", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-10-04T15:59:59Z"));
    const { container } = render(<AnnouncementCard announcement={announcement} />);
    act(() => vi.advanceTimersByTime(1100));
    expect(screen.getByText("ไม่ทราบสถานะ")).toBeInTheDocument();
    expect(screen.getByText(/ข้อมูลเก่า — ยังไม่มีข้อมูลรอบใหม่/)).toBeInTheDocument();
    expect(container.querySelector(".manual-announcement--warning")).toBeNull();
  });
  it("withdraws urgency without claiming safety", () => {
    const { container } = render(<AnnouncementCard announcement={{ ...announcement, status: "WITHDRAWN", effectiveStatus: "UNKNOWN", withdrawalReason: "แหล่งข่าวแก้ไขข้อมูล", withdrawnAt: "2026-10-04T15:00:00Z" }} />);
    expect(screen.getByText("ถอนประกาศแล้ว")).toBeInTheDocument();
    expect(screen.getByText("แหล่งข่าวแก้ไขข้อมูล")).toBeInTheDocument();
    expect(container.querySelector(".manual-announcement--warning")).toBeNull();
  });
  it("labels preview without publication timestamps", () => {
    render(<AnnouncementCard announcement={{ ...announcement, status: "DRAFT", publishedAt: null }} preview />);
    expect(screen.getByText("ตัวอย่างประกาศ — ยังไม่เผยแพร่การเปลี่ยนแปลงนี้")).toBeInTheDocument();
    expect(screen.queryByText(/^เผยแพร่เมื่อ:/)).not.toBeInTheDocument();
    expect(screen.getByText("เตือนภัย")).toBeInTheDocument();
  });
});
