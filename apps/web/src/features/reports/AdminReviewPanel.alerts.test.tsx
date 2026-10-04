import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { AdminReviewPanel } from "./AdminReviewPanel";
import { ApiError, reportsApi } from "./api";
import { confirmAction } from "../../shared/alerts";

vi.mock("./api", () => ({ ApiError: class extends Error { status = 409; }, reportsApi: { listAdmin: vi.fn(), getAdminReport: vi.fn(), review: vi.fn(), restore: vi.fn() } }));
vi.mock("../../shared/alerts", () => ({ confirmAction: vi.fn() }));
vi.mock("./ReportCard", () => ({ ReportCard: ({ report }: { report: { id: string } }) => <><p>รายละเอียดรายงานทดสอบ</p><p data-testid="selected-report">{report.id}</p></> }));
const report = { id: "report", type: "FLOOD" as const, description: "ทดสอบ", location: { type: "Point" as const, coordinates: [101, 14] as [number, number] }, estimatedWaterDepthM: null, status: "PENDING" as const, verificationStatus: "PENDING" as const, sourceType: "COMMUNITY" as const, createdAt: "2026-10-02T00:00:00Z", updatedAt: "2026-10-02T01:00:00Z", verifiedAt: null, media: [] };
const reviewedReport = { ...report, id: "reviewed-report", type: "ROAD" as const, description: "ตรวจแล้ว" };

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((nextResolve) => { resolve = nextResolve; });
  return { promise, resolve };
}

describe("community review confirmations", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(reportsApi.listAdmin).mockResolvedValue({ items: [report], limit: 20, offset: 0 });
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
    vi.mocked(reportsApi.listAdmin).mockResolvedValueOnce({ items: [report], limit: 20, offset: 0 }).mockResolvedValueOnce({ items: [], limit: 20, offset: 0 });
    await select();
    fireEvent.change(screen.getByLabelText("เหตุผลการนำออก"), { target: { value: " ไม่ตรงหลักฐาน " } });
    fireEvent.click(screen.getByRole("button", { name: "นำรายงานออก" }));
    await waitFor(() => expect(reportsApi.review).toHaveBeenCalledWith(report.id, "reject", "ไม่ตรงหลักฐาน", report.updatedAt));
    await waitFor(() => expect(reportsApi.listAdmin).toHaveBeenCalledTimes(2));
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

  it("requires a reason before removing a verified report and renders review history", async () => {
    const verified = { ...report, status: "VERIFIED" as const, verificationStatus: "VERIFIED" as const, verifiedAt: "2026-10-02T02:00:00Z", reviewHistory: [{ id: "history-1", action: "VERIFY" as const, previousStatus: "PENDING" as const, status: "VERIFIED" as const, actorId: "admin-1", reason: null, createdAt: "2026-10-02T02:00:00Z" }] };
    vi.mocked(reportsApi.listAdmin).mockResolvedValue({ items: [verified], limit: 20, offset: 0 });
    vi.mocked(reportsApi.getAdminReport).mockResolvedValue(verified);
    vi.mocked(confirmAction).mockImplementation(async (options) => { await options.action(); return true; });

    render(<AdminReviewPanel />);
    fireEvent.click(await screen.findByRole("button", { name: "เปิดรายงาน FLOOD" }));
    expect(await screen.findByText("ผู้ดำเนินการ: admin-1")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "นำรายงานออก" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("กรุณาระบุเหตุผลก่อนดำเนินการ");
    expect(reportsApi.review).not.toHaveBeenCalled();
    fireEvent.change(screen.getByLabelText("เหตุผลการนำออก"), { target: { value: "ข้อมูลซ้ำ" } });
    fireEvent.click(screen.getByRole("button", { name: "นำรายงานออก" }));
    await waitFor(() => expect(reportsApi.review).toHaveBeenCalledWith(verified.id, "reject", "ข้อมูลซ้ำ", verified.updatedAt));
  });

  it("restores a removed report with a reason and keeps it unreviewed", async () => {
    const removed = { ...report, status: "REJECTED" as const, verificationStatus: "REJECTED" as const, reviewHistory: [] };
    vi.mocked(reportsApi.listAdmin).mockResolvedValue({ items: [removed], limit: 20, offset: 0 });
    vi.mocked(reportsApi.getAdminReport).mockResolvedValue(removed);
    vi.mocked(confirmAction).mockImplementation(async (options) => { await options.action(); return true; });

    render(<AdminReviewPanel />);
    fireEvent.click(screen.getByRole("button", { name: "นำออกแล้ว" }));
    fireEvent.click(await screen.findByRole("button", { name: "เปิดรายงาน FLOOD" }));
    await screen.findByText("รายละเอียดรายงานทดสอบ");
    fireEvent.change(screen.getByLabelText("เหตุผลการนำออกหรือกู้คืน"), { target: { value: "ตรวจสอบซ้ำได้" } });
    fireEvent.click(screen.getByRole("button", { name: "กู้คืนรายงาน" }));
    await waitFor(() => expect(reportsApi.restore).toHaveBeenCalledWith(removed.id, "ตรวจสอบซ้ำได้", removed.updatedAt));
  });

  it("ignores a slow queue response after switching tabs", async () => {
    const pending = deferred<{ items: typeof report[]; limit: number; offset: number }>();
    const reviewed = deferred<{ items: typeof reviewedReport[]; limit: number; offset: number }>();
    vi.mocked(reportsApi.listAdmin).mockImplementation((status) => status === "PENDING" ? pending.promise : reviewed.promise);
    render(<AdminReviewPanel />);

    fireEvent.click(screen.getByRole("button", { name: "ตรวจสอบแล้ว" }));
    expect(await screen.findByText("กำลังโหลดคิวรายงาน")).toBeInTheDocument();
    pending.resolve({ items: [report], limit: 20, offset: 0 });
    await Promise.resolve();
    expect(screen.queryByRole("button", { name: "เปิดรายงาน FLOOD" })).not.toBeInTheDocument();

    reviewed.resolve({ items: [reviewedReport], limit: 20, offset: 0 });
    expect(await screen.findByRole("button", { name: "เปิดรายงาน ROAD" })).toBeInTheDocument();
  });

  it("keeps the latest detail when an earlier detail response arrives late", async () => {
    const first = deferred<typeof report>();
    const second = deferred<typeof reviewedReport>();
    vi.mocked(reportsApi.listAdmin).mockResolvedValue({ items: [report, reviewedReport], limit: 20, offset: 0 });
    vi.mocked(reportsApi.getAdminReport).mockImplementation((id) => id === report.id ? first.promise : second.promise);
    render(<AdminReviewPanel />);

    const firstButton = await screen.findByRole("button", { name: "เปิดรายงาน FLOOD" });
    const secondButton = screen.getByRole("button", { name: "เปิดรายงาน ROAD" });
    fireEvent.click(firstButton);
    expect(await screen.findByText("กำลังโหลดรายละเอียดรายงาน")).toBeInTheDocument();
    fireEvent.click(secondButton);
    second.resolve(reviewedReport);
    expect(await screen.findByTestId("selected-report")).toHaveTextContent(reviewedReport.id);
    first.resolve(report);
    await Promise.resolve();
    expect(screen.getByTestId("selected-report")).toHaveTextContent(reviewedReport.id);
  });

  it("does not reopen a detail after its queue changes", async () => {
    const removed = { ...report, id: "removed-report", type: "OTHER" as const, status: "REJECTED" as const, verificationStatus: "REJECTED" as const };
    const detail = deferred<typeof report>();
    vi.mocked(reportsApi.listAdmin).mockImplementation((status) => Promise.resolve({ items: [status === "PENDING" ? report : removed], limit: 20, offset: 0 }));
    vi.mocked(reportsApi.getAdminReport).mockReturnValue(detail.promise);
    render(<AdminReviewPanel />);

    fireEvent.click(await screen.findByRole("button", { name: "เปิดรายงาน FLOOD" }));
    fireEvent.click(screen.getByRole("button", { name: "นำออกแล้ว" }));
    await screen.findByRole("button", { name: "เปิดรายงาน OTHER" });
    detail.resolve(report);
    await Promise.resolve();
    expect(screen.queryByTestId("selected-report")).not.toBeInTheDocument();
  });

  it("dismisses the detail with Escape and restores focus to its opener", async () => {
    vi.mocked(reportsApi.listAdmin).mockResolvedValue({ items: [report], limit: 20, offset: 0 });
    const view = render(<AdminReviewPanel />);
    const opener = await screen.findByRole("button", { name: "เปิดรายงาน FLOOD" });
    fireEvent.click(opener);
    expect(await screen.findByRole("dialog", { name: "เปิดรายงาน FLOOD" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "ปิดรายละเอียดรายงาน" })).toHaveFocus();
    fireEvent.keyDown(screen.getByRole("dialog", { name: "เปิดรายงาน FLOOD" }), { key: "Escape" });
    expect(screen.queryByRole("dialog", { name: "เปิดรายงาน FLOOD" })).not.toBeInTheDocument();
    expect(opener).toHaveFocus();
    view.unmount();
  });
});
