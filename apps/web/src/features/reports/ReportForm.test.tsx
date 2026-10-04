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
  it("rejects empty or excessive custom depths while preserving the draft", () => {
    reportsApi.create.mockReset();
    render(<ReportForm editing={null} onComplete={vi.fn()} onCancel={vi.fn()} />);
    fireEvent.change(screen.getByLabelText("ชื่อสถานที่ / ชื่อซอย *"), { target: { value: "อาคารทดสอบ" } });
    fireEvent.change(screen.getByLabelText("รายละเอียด"), { target: { value: "น้ำขัง" } });
    fireEvent.click(screen.getByRole("button", { name: "เลือกพิกัด" }));
    fireEvent.click(screen.getByRole("radio", { name: "ระบุค่าอื่น" }));
    fireEvent.click(screen.getByRole("button", { name: "ส่งรายงาน" }));
    expect(screen.getByRole("alert")).toHaveTextContent("ระบุระดับน้ำ");
    for (const value of ["-1", "10001"]) {
      fireEvent.change(screen.getByLabelText("ระดับน้ำ (เซนติเมตร)"), { target: { value } });
      fireEvent.click(screen.getByRole("button", { name: "ส่งรายงาน" }));
      expect(screen.getByRole("alert")).toHaveTextContent("ระบุระดับน้ำ");
    }
    expect(reportsApi.create).not.toHaveBeenCalled();
    expect(screen.getByLabelText("ชื่อสถานที่ / ชื่อซอย *")).toHaveValue("อาคารทดสอบ");
  });

  it("retains a historical non-preset depth and place metadata when editing", async () => {
    const editing = { ...pendingReport, estimatedWaterDepthM: 0.75, locationName: "อาคารเดิม", locationNote: "ฝั่งประตู" };
    reportsApi.update.mockReset().mockResolvedValue(editing);
    render(<ReportForm editing={editing} onComplete={vi.fn()} onCancel={vi.fn()} />);
    expect(screen.getByRole("radio", { name: "ระบุค่าอื่น" })).toBeChecked();
    expect(screen.getByLabelText("ระดับน้ำ (เซนติเมตร)")).toHaveValue(75);
    fireEvent.click(screen.getByRole("button", { name: "บันทึกการแก้ไข" }));
    await waitFor(() => expect(reportsApi.update).toHaveBeenCalledWith("report-1", expect.objectContaining({ locationName: "อาคารเดิม", locationNote: "ฝั่งประตู", estimatedWaterDepthM: 0.75 })));
  });
  it("requires a place name and submits trimmed location labels with a selected depth", async () => {
    reportsApi.create.mockReset().mockResolvedValue({ ...pendingReport, uploadToken: "private-upload-capability" });
    render(<ReportForm editing={null} onComplete={vi.fn()} onCancel={vi.fn()} />);
    fireEvent.change(screen.getByLabelText("รายละเอียด"), { target: { value: "น้ำขัง" } });
    fireEvent.click(screen.getByRole("button", { name: "เลือกพิกัด" }));
    fireEvent.click(screen.getByRole("button", { name: "ส่งรายงาน" }));
    expect(screen.getByRole("alert")).toHaveTextContent("กรอกชื่อสถานที่หรือชื่อซอย");
    expect(reportsApi.create).not.toHaveBeenCalled();
    fireEvent.change(screen.getByLabelText("ชื่อสถานที่ / ชื่อซอย *"), { target: { value: "  อาคารทดสอบ  " } });
    fireEvent.change(screen.getByLabelText("จุดเกิดเหตุ / จุดสังเกต (ไม่บังคับ)"), { target: { value: "  ฝั่งประตู  " } });
    fireEvent.click(screen.getByRole("radio", { name: "30 ซม." }));
    fireEvent.click(screen.getByRole("button", { name: "ส่งรายงาน" }));
    await waitFor(() => expect(reportsApi.create).toHaveBeenCalledWith(expect.objectContaining({ locationName: "อาคารทดสอบ", locationNote: "ฝั่งประตู", estimatedWaterDepthM: 0.3 })));
  });

  it("converts a custom depth to metres and clears it when unspecified is selected", async () => {
    reportsApi.create.mockReset().mockResolvedValue({ ...pendingReport, uploadToken: "private-upload-capability" });
    render(<ReportForm editing={null} onComplete={vi.fn()} onCancel={vi.fn()} />);
    expect(screen.getByRole("radio", { name: "ไม่ระบุ" })).toBeChecked();
    fireEvent.click(screen.getByRole("radio", { name: "ระบุค่าอื่น" }));
    fireEvent.change(screen.getByLabelText("ระดับน้ำ (เซนติเมตร)"), { target: { value: "75" } });
    fireEvent.change(screen.getByLabelText("ชื่อสถานที่ / ชื่อซอย *"), { target: { value: "อาคารทดสอบ" } });
    fireEvent.change(screen.getByLabelText("รายละเอียด"), { target: { value: "น้ำขัง" } });
    fireEvent.click(screen.getByRole("button", { name: "เลือกพิกัด" }));
    fireEvent.click(screen.getByRole("button", { name: "ส่งรายงาน" }));
    await waitFor(() => expect(reportsApi.create).toHaveBeenCalledWith(expect.objectContaining({ estimatedWaterDepthM: 0.75 })));
    fireEvent.change(screen.getByLabelText("ชื่อสถานที่ / ชื่อซอย *"), { target: { value: "อาคารทดสอบ" } });
    fireEvent.change(screen.getByLabelText("รายละเอียด"), { target: { value: "น้ำขัง" } });
    fireEvent.click(screen.getByRole("button", { name: "เลือกพิกัด" }));
    fireEvent.click(screen.getByRole("radio", { name: "50 ซม." }));
    fireEvent.click(screen.getByRole("radio", { name: "ไม่ระบุ" }));
    fireEvent.click(screen.getByRole("button", { name: "ส่งรายงาน" }));
    await waitFor(() => expect(reportsApi.create).toHaveBeenCalledTimes(2));
    expect(reportsApi.create.mock.calls[1][0]).not.toHaveProperty("estimatedWaterDepthM");
  });
  it("keeps a pending edit available when an uploaded file fails", async () => {
    reportsApi.update.mockClear();
    reportsApi.uploadMedia.mockReset();
    reportsApi.update.mockResolvedValue(pendingReport);
    reportsApi.uploadMedia.mockRejectedValue(new Error("storage unavailable"));
    const onComplete = vi.fn();
    render(<ReportForm editing={pendingReport} onComplete={onComplete} onCancel={vi.fn()} />);

    fireEvent.change(screen.getByLabelText("ภาพประกอบ (JPEG, PNG หรือ WebP; ไม่บังคับ)"), { target: { files: [new File(["image"], "water.png", { type: "image/png" })] } });
    fireEvent.click(screen.getByRole("button", { name: "บันทึกการแก้ไข" }));

    expect(await screen.findByRole("alert")).toHaveTextContent("รายงานเผยแพร่แล้ว แต่บางภาพอัปโหลดไม่สำเร็จ");
    expect(onComplete).toHaveBeenCalledWith(true);
    expect(notifyWarning).toHaveBeenCalledTimes(1);
    expect(notifySuccess).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "อัปโหลดไฟล์ที่เหลืออีกครั้ง" })).toBeInTheDocument();
  });

  it("retries only failed files against the created report without duplicating it", async () => {
    reportsApi.create.mockReset().mockResolvedValue({ ...pendingReport, uploadToken: "private-upload-capability" });
    reportsApi.uploadMedia.mockReset().mockResolvedValueOnce({ id: "image-1" }).mockRejectedValueOnce(new Error("storage unavailable")).mockResolvedValueOnce({ id: "image-2" });
    const onComplete = vi.fn();
    const successfulFile = new File(["first"], "same.png", { type: "image/png" });
    const failedFile = new File(["second"], "same.png", { type: "image/png" });
    render(<ReportForm editing={null} onComplete={onComplete} onCancel={vi.fn()} />);

    fireEvent.change(screen.getByLabelText("รายละเอียด"), { target: { value: "น้ำท่วมขัง" } });
    fireEvent.change(screen.getByLabelText("ชื่อสถานที่ / ชื่อซอย *"), { target: { value: "ตลาดทดสอบ" } });
    fireEvent.click(screen.getByRole("button", { name: "เลือกพิกัด" }));
    fireEvent.change(screen.getByLabelText("ภาพประกอบ (JPEG, PNG หรือ WebP; ไม่บังคับ)"), { target: { files: [
      successfulFile, failedFile
    ] } });
    fireEvent.click(screen.getByRole("button", { name: "ส่งรายงาน" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("รายงานเผยแพร่แล้ว แต่บางภาพอัปโหลดไม่สำเร็จ");
    expect(onComplete).toHaveBeenCalledWith(true);
    expect(reportsApi.create).toHaveBeenCalledTimes(1);
    expect(reportsApi.uploadMedia).toHaveBeenCalledTimes(2);
    expect(reportsApi.uploadMedia).toHaveBeenNthCalledWith(1, pendingReport.id, successfulFile, "private-upload-capability");
    expect(reportsApi.uploadMedia).toHaveBeenNthCalledWith(2, pendingReport.id, failedFile, "private-upload-capability");

    fireEvent.click(screen.getByRole("button", { name: "อัปโหลดไฟล์ที่เหลืออีกครั้ง" }));
    await waitFor(() => expect(onComplete).toHaveBeenLastCalledWith());
    expect(reportsApi.create).toHaveBeenCalledTimes(1);
    expect(reportsApi.uploadMedia).toHaveBeenCalledTimes(3);
    expect(reportsApi.uploadMedia).toHaveBeenNthCalledWith(3, pendingReport.id, failedFile, "private-upload-capability");
    expect(notifySuccess).toHaveBeenCalledTimes(1);
    expect(Object.values(window.localStorage).join(" ")).not.toContain("private-upload-capability");
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

  it("submits a trimmed guest-name snapshot and prevents duplicate creates", async () => {
    let finish!: (value: typeof pendingReport & { uploadToken: string }) => void;
    reportsApi.create.mockReset().mockImplementation(() => new Promise((resolve) => { finish = resolve; }));
    render(<ReportForm editing={null} onComplete={vi.fn()} onCancel={vi.fn()} />);
    fireEvent.change(screen.getByLabelText("ชื่อผู้รายงาน (ไม่บังคับ)"), { target: { value: "  ชุมชนทดสอบ  " } });
    fireEvent.change(screen.getByLabelText("ชื่อสถานที่ / ชื่อซอย *"), { target: { value: "อาคารทดสอบ" } });
    fireEvent.change(screen.getByLabelText("รายละเอียด"), { target: { value: "รายงานทดสอบ" } });
    fireEvent.click(screen.getByRole("button", { name: "เลือกพิกัด" }));
    const form = screen.getByLabelText("รายละเอียด").closest("form")!;
    fireEvent.submit(form);
    fireEvent.submit(form);
    expect(reportsApi.create).toHaveBeenCalledTimes(1);
    expect(reportsApi.create).toHaveBeenCalledWith(expect.objectContaining({ displayName: "ชุมชนทดสอบ" }));
    finish({ ...pendingReport, uploadToken: "private-upload-capability" });
    expect(await screen.findByRole("status")).toHaveTextContent("รายงานเผยแพร่แล้วในฐานะข้อมูล COMMUNITY");
  });
});
