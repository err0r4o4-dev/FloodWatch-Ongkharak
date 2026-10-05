import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ReportForm } from "./ReportForm";
import { notifyError, notifySuccess, notifyWarning } from "../../shared/alerts";

vi.mock("../../shared/alerts", () => ({ notifyError: vi.fn(), notifySuccess: vi.fn(), notifyWarning: vi.fn() }));

const reportsApi = vi.hoisted(() => ({ create: vi.fn(), update: vi.fn(), uploadMedia: vi.fn() }));
const roadSegmentsApi = vi.hoisted(() => ({ candidates: vi.fn() }));

vi.mock("./api", () => ({
  ApiError: class ApiError extends Error {},
  reportsApi
}));

vi.mock("../road-segments/api", () => ({ roadSegmentsApi }));

vi.mock("./ReportLocationPicker", () => ({
  ReportLocationPicker: ({ onChange }: { onChange: (location: typeof pendingReport.location) => void }) => <>
    <button type="button" onClick={() => onChange(pendingReport.location)}>เลือกพิกัด</button>
    <button type="button" onClick={() => onChange({ type: "Point", coordinates: [101.2, 14.3] })}>เลือกพิกัดใหม่</button>
  </>
}));

const pendingReport = {
  id: "report-1", userId: "user-1", type: "FLOOD" as const, description: "น้ำท่วมขัง",
  location: { type: "Point" as const, coordinates: [101.1, 14.2] as [number, number] }, estimatedWaterDepthM: null,
  status: "PENDING" as const, verificationStatus: "PENDING" as const, sourceType: "COMMUNITY" as const,
  createdAt: "2026-10-02T08:00:00Z", updatedAt: "2026-10-02T08:00:00Z", verifiedAt: null, verifiedBy: null, media: []
};

describe("ReportForm uploads", () => {
  beforeEach(() => { vi.clearAllMocks(); roadSegmentsApi.candidates.mockResolvedValue({ items: [], assignmentStatus: "UNASSIGNED", matchingDistanceM: 30 }); });
  afterEach(() => cleanup());

  it("requires explicit confirmation for a single road-segment candidate", async () => {
    const candidate = { id: "segment-1", roadId: "road-1", name: "ถนนทดสอบ", startLabel: "สะพาน", endLabel: "ตลาด", geometry: { type: "LineString", coordinates: [[101, 14], [101.1, 14.1]] }, location: { type: "Point", coordinates: [101.05, 14.05] }, version: 1, supersedesId: null, active: true, createdAt: "2026-10-02T00:00:00Z", distanceM: 4 };
    roadSegmentsApi.candidates.mockResolvedValue({ items: [candidate], assignmentStatus: "SINGLE_CANDIDATE", matchingDistanceM: 30 });
    reportsApi.create.mockResolvedValue({ ...pendingReport, uploadToken: "token" });
    render(<ReportForm editing={null} onComplete={vi.fn()} onCancel={vi.fn()} />);
    fireEvent.change(screen.getByLabelText("ชื่อสถานที่ / ชื่อซอย *"), { target: { value: "อาคารทดสอบ" } });
    fireEvent.change(screen.getByLabelText("รายละเอียด"), { target: { value: "น้ำขัง" } });
    fireEvent.click(screen.getByRole("button", { name: "เลือกพิกัด" }));
    expect(await screen.findByRole("radio", { name: /ถนนทดสอบ/ })).toBeInTheDocument();
    expect(screen.getByRole("radio", { name: /ถนนทดสอบ/ })).not.toBeChecked();
    expect(reportsApi.create).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("radio", { name: /ถนนทดสอบ/ }));
    fireEvent.click(screen.getByRole("button", { name: "ส่งรายงาน" }));
    await waitFor(() => expect(reportsApi.create).toHaveBeenLastCalledWith(expect.objectContaining({ roadSegmentId: "segment-1" })));
  });

  it("clears stale segment selection when coordinates move and ignores the old response", async () => {
    let resolveFirst!: (value: unknown) => void;
    let resolveSecond!: (value: unknown) => void;
    roadSegmentsApi.candidates
      .mockImplementationOnce(() => new Promise((resolve) => { resolveFirst = resolve; }))
      .mockImplementationOnce(() => new Promise((resolve) => { resolveSecond = resolve; }));
    render(<ReportForm editing={null} onComplete={vi.fn()} onCancel={vi.fn()} />);
    fireEvent.click(screen.getByRole("button", { name: "เลือกพิกัด" }));
    await waitFor(() => expect(roadSegmentsApi.candidates).toHaveBeenCalledWith(101.1, 14.2, expect.any(AbortSignal)));
    resolveFirst({ items: [], assignmentStatus: "UNASSIGNED", matchingDistanceM: 30 });
    fireEvent.click(screen.getByRole("button", { name: "เลือกพิกัดใหม่" }));
    await waitFor(() => expect(roadSegmentsApi.candidates).toHaveBeenLastCalledWith(101.2, 14.3, expect.any(AbortSignal)));
    resolveSecond({ items: [], assignmentStatus: "UNASSIGNED", matchingDistanceM: 30 });
    await waitFor(() => expect(screen.getByText("ยังไม่ระบุช่วงถนน")).toBeInTheDocument());
    expect(screen.queryByText(/ถนนทดสอบ/)).not.toBeInTheDocument();
  });

  it("shows multiple roads without selecting one and permits an explicit unassigned save", async () => {
    const makeCandidate = (id: string, name: string) => ({ id, roadId: id, name, startLabel: "ต้นทาง", endLabel: "ปลายทาง", geometry: { type: "LineString", coordinates: [[101, 14], [101.1, 14.1]] }, location: { type: "Point", coordinates: [101.05, 14.05] }, version: 1, supersedesId: null, active: true, createdAt: "2026-10-02T00:00:00Z", distanceM: 8 });
    roadSegmentsApi.candidates.mockResolvedValue({ items: [makeCandidate("segment-1", "ถนนหนึ่ง"), makeCandidate("segment-2", "ถนนสอง")], assignmentStatus: "AMBIGUOUS", matchingDistanceM: 30 });
    reportsApi.create.mockResolvedValue({ ...pendingReport, uploadToken: "token" });
    render(<ReportForm editing={null} onComplete={vi.fn()} onCancel={vi.fn()} />);
    fireEvent.change(screen.getByLabelText("ชื่อสถานที่ / ชื่อซอย *"), { target: { value: "อาคารทดสอบ" } });
    fireEvent.change(screen.getByLabelText("รายละเอียด"), { target: { value: "น้ำขัง" } });
    fireEvent.click(screen.getByRole("button", { name: "เลือกพิกัด" }));
    expect(await screen.findByRole("radio", { name: /ถนนหนึ่ง/ })).toBeInTheDocument();
    expect(screen.getByRole("radio", { name: /ยังไม่ระบุช่วงถนน/ })).toBeChecked();
    fireEvent.click(screen.getByRole("button", { name: "ส่งรายงาน" }));
    await waitFor(() => expect(reportsApi.create).toHaveBeenCalledWith(expect.objectContaining({ roadSegmentId: "" })));
  });

  it("reports candidate failure while still saving explicitly unassigned", async () => {
    roadSegmentsApi.candidates.mockRejectedValue(new Error("offline"));
    reportsApi.create.mockResolvedValue({ ...pendingReport, uploadToken: "token" });
    render(<ReportForm editing={null} onComplete={vi.fn()} onCancel={vi.fn()} />);
    fireEvent.change(screen.getByLabelText("ชื่อสถานที่ / ชื่อซอย *"), { target: { value: "อาคารทดสอบ" } });
    fireEvent.change(screen.getByLabelText("รายละเอียด"), { target: { value: "น้ำขัง" } });
    fireEvent.click(screen.getByRole("button", { name: "เลือกพิกัด" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("ไม่สามารถโหลดช่วงถนนได้");
    fireEvent.click(screen.getByRole("button", { name: "ส่งรายงาน" }));
    await waitFor(() => expect(reportsApi.create).toHaveBeenCalledWith(expect.objectContaining({ roadSegmentId: "" })));
  });
  it("submits an observation time and a falling trend without inventing a depth", async () => {
    reportsApi.create.mockReset().mockResolvedValue({ ...pendingReport, uploadToken: "private-upload-capability" });
    render(<ReportForm editing={null} onComplete={vi.fn()} onCancel={vi.fn()} />);
    fireEvent.change(screen.getByLabelText("ชื่อสถานที่ / ชื่อซอย *"), { target: { value: "อาคารทดสอบ" } });
    fireEvent.change(screen.getByLabelText("รายละเอียด"), { target: { value: "ระดับน้ำกำลังลด" } });
    fireEvent.click(screen.getByRole("button", { name: "เลือกพิกัด" }));
    fireEvent.change(screen.getByLabelText("การเปลี่ยนแปลงที่สังเกต"), { target: { value: "FALLING" } });
    fireEvent.click(screen.getByRole("button", { name: "ส่งรายงาน" }));
    await waitFor(() => expect(reportsApi.create).toHaveBeenCalledWith(expect.objectContaining({ observedAt: expect.any(String), waterTrend: "FALLING" })));
    expect(reportsApi.create.mock.calls[0][0]).not.toHaveProperty("estimatedWaterDepthM");
  });

  it("rejects a future observation time and retains the report draft", () => {
    reportsApi.create.mockReset();
    render(<ReportForm editing={null} onComplete={vi.fn()} onCancel={vi.fn()} />);
    fireEvent.change(screen.getByLabelText("ชื่อสถานที่ / ชื่อซอย *"), { target: { value: "อาคารทดสอบ" } });
    fireEvent.change(screen.getByLabelText("รายละเอียด"), { target: { value: "น้ำขัง" } });
    fireEvent.click(screen.getByRole("button", { name: "เลือกพิกัด" }));
    fireEvent.change(screen.getByLabelText("เวลาที่พบเหตุ"), { target: { value: "custom" } });
    fireEvent.change(screen.getByLabelText("ระบุวันและเวลาที่พบเหตุ"), { target: { value: "2099-01-01T12:00" } });
    fireEvent.click(screen.getByRole("button", { name: "ส่งรายงาน" }));
    expect(screen.getByRole("alert")).toHaveTextContent("เวลาที่พบเหตุ");
    expect(reportsApi.create).not.toHaveBeenCalled();
    expect(screen.getByLabelText("รายละเอียด")).toHaveValue("น้ำขัง");
  });

  it("prefills an area update while leaving its incident point editable", async () => {
    reportsApi.create.mockReset().mockResolvedValue({ ...pendingReport, uploadToken: "private-upload-capability" });
    render(<ReportForm editing={null} initialLocation={pendingReport.location} initialLocationName="พื้นที่ทดสอบ" onComplete={vi.fn()} onCancel={vi.fn()} />);
    expect(screen.getByLabelText("ชื่อสถานที่ / ชื่อซอย *")).toHaveValue("พื้นที่ทดสอบ");
    fireEvent.change(screen.getByLabelText("รายละเอียด"), { target: { value: "รายงานล่าสุด" } });
    fireEvent.click(screen.getByRole("button", { name: "ส่งรายงาน" }));
    await waitFor(() => expect(reportsApi.create).toHaveBeenCalledWith(expect.objectContaining({ location: pendingReport.location, locationName: "พื้นที่ทดสอบ" })));
  });

  it("does not replace missing historical observation time when editing", async () => {
    reportsApi.update.mockReset().mockResolvedValue(pendingReport);
    render(<ReportForm editing={pendingReport} onComplete={vi.fn()} onCancel={vi.fn()} />);
    fireEvent.click(screen.getByRole("button", { name: "บันทึกการแก้ไข" }));
    await waitFor(() => expect(reportsApi.update).toHaveBeenCalled());
    expect(reportsApi.update.mock.calls[0][1]).not.toHaveProperty("observedAt");
  });

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
    expect(await screen.findByText("รายงานเผยแพร่แล้วในฐานะข้อมูล COMMUNITY และรอการตรวจสอบข้อเท็จจริง การเผยแพร่ไม่ได้หมายถึงยืนยันแล้วหรือปลอดภัย")).toBeInTheDocument();
  });
});
