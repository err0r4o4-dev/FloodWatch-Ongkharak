import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { AdminReviewPanel } from "./AdminReviewPanel";
import { ApiError, reportsApi } from "./api";
import { confirmAction } from "../../shared/alerts";

vi.mock("./api", () => ({ ApiError: class extends Error { status = 409; }, reportsApi: { listPending: vi.fn(), getAdminReport: vi.fn(), review: vi.fn() } }));
vi.mock("../../shared/alerts", () => ({ confirmAction: vi.fn() }));
vi.mock("./ReportCard", () => ({ ReportCard: () => <p>รายละเอียดรายงานทดสอบ</p> }));
const report = { id: "report", type: "FLOOD" as const, description: "ทดสอบ", location: { type: "Point" as const, coordinates: [101, 14] as [number, number] }, estimatedWaterDepthM: null, status: "PENDING" as const, verificationStatus: "PENDING" as const, sourceType: "COMMUNITY" as const, createdAt: "2026-10-02T00:00:00Z", updatedAt: "2026-10-02T01:00:00Z", verifiedAt: null, media: [] };

describe("community review confirmations", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(reportsApi.listPending).mockResolvedValue({ items: [report], limit: 20, offset: 0 });
    vi.mocked(reportsApi.getAdminReport).mockResolvedValue(report);
    vi.mocked(reportsApi.review).mockResolvedValue(report);
    vi.mocked(confirmAction).mockResolvedValue(false);
  });
  async function select() {
    render(<AdminReviewPanel />);
    fireEvent.click(await screen.findByRole("button", { name: "เปิดรายงาน FLOOD" }));
    await screen.findByText("รายละเอียดรายงานทดสอบ");
  }

  it("does not review a canceled confirmation", async () => {
    await select();
    fireEvent.click(screen.getByRole("button", { name: "ยืนยันรายงาน" }));
    await waitFor(() => expect(confirmAction).toHaveBeenCalledTimes(1));
    expect(reportsApi.review).not.toHaveBeenCalled();
  });

  it("preserves the reviewed revision and rejection reason", async () => {
    vi.mocked(confirmAction).mockImplementation(async (options) => { await options.action(); return true; });
    await select();
    fireEvent.change(screen.getByLabelText("เหตุผลที่ปฏิเสธ (ไม่บังคับ)"), { target: { value: " ไม่ตรงหลักฐาน " } });
    fireEvent.click(screen.getByRole("button", { name: "ปฏิเสธรายงาน" }));
    await waitFor(() => expect(reportsApi.review).toHaveBeenCalledWith(report.id, "reject", "ไม่ตรงหลักฐาน", report.updatedAt));
    expect(await screen.findByText("ไม่มีรายงานที่รอตรวจสอบ")).toBeInTheDocument();
  });

  it("keeps revision conflicts visible and requires re-opening details", async () => {
    vi.mocked(reportsApi.review).mockRejectedValue(new ApiError("conflict", 409));
    vi.mocked(confirmAction).mockImplementation(async (options) => {
      try { await options.action(); } catch { return false; }
      return true;
    });
    await select();
    fireEvent.click(screen.getByRole("button", { name: "ยืนยันรายงาน" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("รายงานมีการแก้ไขหรือเพิ่มภาพแล้ว");
    expect(screen.queryByText("รายละเอียดรายงานทดสอบ")).not.toBeInTheDocument();
  });
});
