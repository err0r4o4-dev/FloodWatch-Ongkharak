import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { AnnouncementsPage } from "./AnnouncementsPage";
import { listPublicAlerts } from "./api";
import { manualAnnouncementApi } from "../manual-announcements/api";

vi.mock("./api", () => ({ listPublicAlerts: vi.fn() }));
vi.mock("../manual-announcements/api", () => ({ manualAnnouncementApi: { listPublic: vi.fn() } }));

beforeEach(() => {
  vi.mocked(listPublicAlerts).mockReset();
  vi.mocked(manualAnnouncementApi.listPublic).mockResolvedValue({ items: [], total: 0, limit: 20, offset: 0 });
});

describe("public alerts", () => {
  it("shows an honest empty state rather than implying safety", async () => {
    vi.mocked(listPublicAlerts).mockResolvedValue([]);
    render(<AnnouncementsPage />);
    expect(await screen.findByText("ยังไม่มีประกาศที่เผยแพร่")).toBeInTheDocument();
    expect(await screen.findByRole("heading", { name: "ประกาศจากผู้ดูแล" })).toBeInTheDocument();
    expect(screen.getByText(/ไม่ได้หมายความว่าพื้นที่ปลอดภัย/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "ทั้งหมด (0)" })).toBeInTheDocument();
    expect(screen.getByRole("complementary", { name: "สรุปประกาศ" })).toHaveTextContent("ยังไม่มีข้อมูลสรุป");
  });

  it("renders severity, provenance and trigger time from the canonical API", async () => {
    vi.mocked(listPublicAlerts).mockResolvedValue([{
      id: "alert-1", ruleId: "rule-1", severity: "WARNING", title: "กฎทดสอบ", message: "ข้อมูลทดสอบจากชุมชน",
      sourceType: "COMMUNITY", resourceType: "road", resourceId: "road-1", triggeredAt: "2026-10-02T08:00:00Z", createdAt: "2026-10-02T08:01:00Z"
    }]);
    render(<AnnouncementsPage />);
    expect(await screen.findByRole("heading", { name: "ประกาศและแจ้งเตือน" })).toBeInTheDocument();
    expect(screen.getAllByRole("heading", { name: "กฎทดสอบ" })).toHaveLength(2);
    expect(screen.getAllByText("ข้อมูลทดสอบจากชุมชน")).toHaveLength(2);
    expect(screen.getAllByText("รายงานชุมชน")).toHaveLength(3);
    expect(screen.getAllByText("เตือนภัย")).toHaveLength(2);
    expect(screen.getAllByText(/เริ่มแจ้งเตือน/)).toHaveLength(2);
  });

  it("filters the published alerts by severity without refetching canonical data", async () => {
    vi.mocked(listPublicAlerts).mockResolvedValue([
      {
        id: "alert-warning", ruleId: "rule-warning", severity: "WARNING", title: "ประกาศเตือนภัย", message: "โปรดระวัง",
        sourceType: "OFFICIAL", resourceType: "road", resourceId: "road-1", triggeredAt: "2026-10-02T08:00:00Z", createdAt: "2026-10-02T08:01:00Z"
      },
      {
        id: "alert-info", ruleId: "rule-info", severity: "INFO", title: "ประกาศข้อมูลทั่วไป", message: "ติดตามสถานการณ์",
        sourceType: "AI", resourceType: "water", resourceId: "water-1", triggeredAt: "2026-10-02T07:00:00Z", createdAt: "2026-10-02T07:01:00Z"
      }
    ]);
    render(<AnnouncementsPage />);
    expect(await screen.findAllByRole("heading", { name: "ประกาศเตือนภัย" })).toHaveLength(2);
    expect(screen.getByRole("heading", { name: "ประกาศข้อมูลทั่วไป" })).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "เตือนภัย (1)" }));

    expect(screen.getAllByRole("heading", { name: "ประกาศเตือนภัย" })).toHaveLength(2);
    expect(screen.queryByRole("heading", { name: "ประกาศข้อมูลทั่วไป" })).not.toBeInTheDocument();
    expect(listPublicAlerts).toHaveBeenCalledTimes(1);
  });

  it("preserves REST retry when the API fails", async () => {
    vi.mocked(listPublicAlerts).mockRejectedValueOnce(new Error("offline")).mockResolvedValueOnce([]);
    render(<AnnouncementsPage />);
    expect(await screen.findByRole("alert")).toHaveTextContent("ไม่สามารถโหลดข้อมูลสถานการณ์ได้");
    expect(screen.getByRole("button", { name: "ทั้งหมด" })).toBeInTheDocument();
    expect(screen.getByRole("complementary", { name: "สรุปประกาศ" })).toHaveTextContent("ข้อมูลสรุปไม่พร้อมใช้งาน");
    fireEvent.click(screen.getByRole("button", { name: "ลองอีกครั้ง" }));
    expect(await screen.findByText("ยังไม่มีประกาศที่เผยแพร่")).toBeInTheDocument();
    expect(listPublicAlerts).toHaveBeenCalledTimes(2);
  });
});
