import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ReportForm } from "./ReportForm";
import { notifyError, notifySuccess, notifyWarning } from "../../shared/alerts";

vi.mock("../../shared/alerts", () => ({ notifyError: vi.fn(), notifySuccess: vi.fn(), notifyWarning: vi.fn() }));

const reportsApi = vi.hoisted(() => ({ create: vi.fn(), update: vi.fn(), uploadMedia: vi.fn() }));

vi.mock("./api", () => ({
  ApiError: class ApiError extends Error {},
  reportsApi
}));

vi.mock("./ReportLocationPicker", () => ({
  ReportLocationPicker: ({ onChange }: { onChange: (location: typeof pendingReport.location) => void }) =>
    <button type="button" onClick={() => onChange(pendingReport.location)}>เลือกพิกัด</button>
}));

const pendingReport = {
  id: "report-1", userId: "user-1", type: "FLOOD" as const, description: "น้ำท่วมขัง",
  location: { type: "Point" as const, coordinates: [101.1, 14.2] as [number, number] }, estimatedWaterDepthM: null,
  status: "PENDING" as const, verificationStatus: "PENDING" as const, sourceType: "COMMUNITY" as const,
  createdAt: "2026-10-02T08:00:00Z", updatedAt: "2026-10-02T08:00:00Z", verifiedAt: null, verifiedBy: null, media: []
};

describe("ReportForm uploads", () => {
  beforeEach(() => { vi.clearAllMocks(); });
  it("keeps a pending edit available when an uploaded file fails", async () => {
    reportsApi.update.mockClear();
    reportsApi.uploadMedia.mockReset();
    reportsApi.update.mockResolvedValue(pendingReport);
    reportsApi.uploadMedia.mockRejectedValue(new Error("storage unavailable"));
    const onComplete = vi.fn();
    render(<ReportForm editing={pendingReport} onComplete={onComplete} onCancel={vi.fn()} />);

    fireEvent.change(screen.getByLabelText("ภาพประกอบ (JPEG, PNG หรือ WebP; ไม่บังคับ)"), { target: { files: [new File(["image"], "water.png", { type: "image/png" })] } });
    fireEvent.click(screen.getByRole("button", { name: "บันทึกการแก้ไข" }));

    expect(await screen.findByRole("alert")).toHaveTextContent("บางไฟล์อัปโหลดไม่สำเร็จ");
    expect(onComplete).toHaveBeenCalledWith(true);
    expect(notifyWarning).toHaveBeenCalledTimes(1);
    expect(notifySuccess).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "อัปโหลดไฟล์ที่เหลืออีกครั้ง" })).toBeInTheDocument();
  });

  it("retries only failed files against the created report without duplicating it", async () => {
    reportsApi.create.mockReset().mockResolvedValue(pendingReport);
    reportsApi.uploadMedia.mockReset().mockResolvedValueOnce({ id: "image-1" }).mockRejectedValueOnce(new Error("storage unavailable")).mockResolvedValueOnce({ id: "image-2" });
    const onComplete = vi.fn();
    const successfulFile = new File(["first"], "same.png", { type: "image/png" });
    const failedFile = new File(["second"], "same.png", { type: "image/png" });
    render(<ReportForm editing={null} onComplete={onComplete} onCancel={vi.fn()} />);

    fireEvent.change(screen.getByLabelText("รายละเอียด"), { target: { value: "น้ำท่วมขัง" } });
    fireEvent.click(screen.getByRole("button", { name: "เลือกพิกัด" }));
    fireEvent.change(screen.getByLabelText("ภาพประกอบ (JPEG, PNG หรือ WebP; ไม่บังคับ)"), { target: { files: [
      successfulFile, failedFile
    ] } });
    fireEvent.click(screen.getByRole("button", { name: "ส่งรายงาน" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("บางไฟล์อัปโหลดไม่สำเร็จ");
    expect(onComplete).toHaveBeenCalledWith(true);
    expect(reportsApi.create).toHaveBeenCalledTimes(1);
    expect(reportsApi.uploadMedia).toHaveBeenCalledTimes(2);
    expect(reportsApi.uploadMedia).toHaveBeenNthCalledWith(1, pendingReport.id, successfulFile);
    expect(reportsApi.uploadMedia).toHaveBeenNthCalledWith(2, pendingReport.id, failedFile);

    fireEvent.click(screen.getByRole("button", { name: "อัปโหลดไฟล์ที่เหลืออีกครั้ง" }));
    await waitFor(() => expect(onComplete).toHaveBeenLastCalledWith());
    expect(reportsApi.create).toHaveBeenCalledTimes(1);
    expect(reportsApi.uploadMedia).toHaveBeenCalledTimes(3);
    expect(reportsApi.uploadMedia).toHaveBeenNthCalledWith(3, pendingReport.id, failedFile);
    expect(notifySuccess).toHaveBeenCalledTimes(1);
  });

  it("notifies on save failure while keeping the form error and draft", async () => {
    reportsApi.update.mockRejectedValue(new Error("offline"));
    render(<ReportForm editing={pendingReport} onComplete={vi.fn()} onCancel={vi.fn()} />);
    fireEvent.click(screen.getByRole("button", { name: "บันทึกการแก้ไข" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("ไม่สามารถบันทึกรายงานได้");
    expect(notifyError).toHaveBeenCalledTimes(1);
    expect(notifySuccess).not.toHaveBeenCalled();
    expect(screen.getByLabelText("รายละเอียด")).toHaveValue(pendingReport.description);
  });
});
