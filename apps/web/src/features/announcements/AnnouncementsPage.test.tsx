import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { AnnouncementsPage } from "./AnnouncementsPage";
import { listPublicAlerts } from "./api";

vi.mock("./api", () => ({ listPublicAlerts: vi.fn() }));

beforeEach(() => vi.mocked(listPublicAlerts).mockReset());

describe("public alerts", () => {
  it("shows an honest empty state rather than implying safety", async () => {
    vi.mocked(listPublicAlerts).mockResolvedValue([]);
    render(<AnnouncementsPage />);
    expect(await screen.findByText("ยังไม่มีประกาศที่เผยแพร่")).toBeInTheDocument();
    expect(screen.getByText(/ไม่ได้หมายความว่าพื้นที่ปลอดภัย/)).toBeInTheDocument();
  });

  it("renders severity, provenance and trigger time from the canonical API", async () => {
    vi.mocked(listPublicAlerts).mockResolvedValue([{
      id: "alert-1", ruleId: "rule-1", severity: "WARNING", title: "กฎทดสอบ", message: "ข้อมูลทดสอบจากชุมชน",
      sourceType: "COMMUNITY", resourceType: "road", resourceId: "road-1", triggeredAt: "2026-10-02T08:00:00Z", createdAt: "2026-10-02T08:01:00Z"
    }]);
    render(<AnnouncementsPage />);
    expect(await screen.findByText("กฎทดสอบ")).toBeInTheDocument();
    expect(screen.getByText("ข้อมูลทดสอบจากชุมชน")).toBeInTheDocument();
    expect(screen.getByText("รายงานชุมชน")).toBeInTheDocument();
    expect(screen.getByText("เตือนภัย")).toBeInTheDocument();
    expect(screen.getByText(/เริ่มแจ้งเตือน/)).toBeInTheDocument();
  });

  it("preserves REST retry when the API fails", async () => {
    vi.mocked(listPublicAlerts).mockRejectedValueOnce(new Error("offline")).mockResolvedValueOnce([]);
    render(<AnnouncementsPage />);
    expect(await screen.findByRole("alert")).toHaveTextContent("ไม่สามารถโหลดข้อมูลสถานการณ์ได้");
    fireEvent.click(screen.getByRole("button", { name: "ลองอีกครั้ง" }));
    expect(await screen.findByText("ยังไม่มีประกาศที่เผยแพร่")).toBeInTheDocument();
    expect(listPublicAlerts).toHaveBeenCalledTimes(2);
  });
});
