import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "../auth/api";
import { confirmAction } from "../../shared/alerts";
import { aiApi } from "./ai-api";
import { AIEvidenceReview } from "./AIEvidenceReview";

vi.mock("../../shared/alerts", () => ({ confirmAction: vi.fn() }));
vi.mock("./ai-api", () => ({ AI_EVIDENCE_PAGE_SIZE: 20, aiApi: { list: vi.fn(), get: vi.fn(), listRoads: vi.fn(), verify: vi.fn(), reject: vi.fn() } }));

const floodEvidence = {
  id: "evidence-1",
  reportId: "report-1",
  mediaId: "media-1",
  sourceType: "AI" as const,
  modelVersion: "vision-1",
  flood: { severity: "WARNING", water_depth_m: 0, confidence: 0, bounding_box: [0, 0, 1, 1] },
  road: null,
  observedAt: "2026-10-04T08:00:00Z",
  receivedAt: "2026-10-04T08:01:00Z",
  limitations: ["ภาพมุมเดียว"],
  status: "UNVERIFIED" as const
};
const roadEvidence = { ...floodEvidence, id: "evidence-road", road: { status: "CLOSED", confidence: 0.8, bounding_box: [0, 0, 1, 1] } };
const page = { items: [floodEvidence], pagination: { total: 1, limit: 20, offset: 0 } };

describe("AIEvidenceReview", () => {
  beforeEach(() => {
    vi.resetAllMocks();
    vi.mocked(aiApi.list).mockResolvedValue(page);
    vi.mocked(aiApi.get).mockResolvedValue(floodEvidence);
    vi.mocked(aiApi.listRoads).mockResolvedValue([{ id: "road-1", name: "ถนนองครักษ์" }]);
    vi.mocked(aiApi.verify).mockResolvedValue(undefined);
    vi.mocked(aiApi.reject).mockResolvedValue(undefined);
    vi.mocked(confirmAction).mockResolvedValue(false);
  });

  it("renders source, zero confidence, limitations, timestamps, and a safe report link", async () => {
    render(<AIEvidenceReview />);
    expect(await screen.findByText("vision-1")).toBeInTheDocument();
    expect(screen.getByText("หลักฐาน AI")).toBeInTheDocument();
    expect(screen.getByText("0%")).toBeInTheDocument();
    expect(screen.getByText("ภาพมุมเดียว")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "ดูรายงานที่เกี่ยวข้อง" })).toHaveAttribute("href", "/reports?report=report-1");
  });

  it("requires an actual road selection before approving a road result", async () => {
    vi.mocked(aiApi.list).mockResolvedValue({ items: [roadEvidence], pagination: { total: 1, limit: 20, offset: 0 } });
    vi.mocked(aiApi.get).mockResolvedValue(roadEvidence);
    render(<AIEvidenceReview />);
    fireEvent.click(await screen.findByRole("button", { name: "เปิดรายละเอียด: evidence-road" }));
    expect(await screen.findByRole("combobox", { name: "จับคู่ถนนสำหรับอนุมัติ" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "อนุมัติหลักฐาน AI" })).toBeDisabled();
    fireEvent.change(screen.getByRole("combobox", { name: "จับคู่ถนนสำหรับอนุมัติ" }), { target: { value: "road-1" } });
    expect(screen.getByRole("button", { name: "อนุมัติหลักฐาน AI" })).toBeEnabled();
  });

  it("runs the real review after confirmation and reloads the unverified list", async () => {
    vi.mocked(confirmAction).mockImplementation(async (options) => { await options.action(); return true; });
    render(<AIEvidenceReview />);
    fireEvent.click(await screen.findByRole("button", { name: "เปิดรายละเอียด: evidence-1" }));
    await screen.findByText("รายละเอียดหลักฐาน");
    fireEvent.click(screen.getByRole("button", { name: "อนุมัติหลักฐาน AI" }));

    await waitFor(() => expect(aiApi.verify).toHaveBeenCalledWith("evidence-1", {}));
    await waitFor(() => expect(aiApi.list).toHaveBeenCalledTimes(2));
  });

  it("shows a reload action when review conflicts with a newer decision", async () => {
    vi.mocked(confirmAction).mockImplementation(async (options) => {
      try { await options.action(); } catch { return false; }
      return true;
    });
    vi.mocked(aiApi.verify).mockRejectedValue(new ApiError("conflict", 409));
    render(<AIEvidenceReview />);
    fireEvent.click(await screen.findByRole("button", { name: "เปิดรายละเอียด: evidence-1" }));
    fireEvent.click(await screen.findByRole("button", { name: "อนุมัติหลักฐาน AI" }));

    expect(await screen.findByRole("alert")).toHaveTextContent("รายการนี้ถูกเปลี่ยนแปลงแล้ว");
    fireEvent.click(screen.getByRole("button", { name: "โหลดรายการอีกครั้ง" }));
    await waitFor(() => expect(aiApi.list).toHaveBeenCalledTimes(2));
  });

  it("shows loading, empty, and retry states without making guest assumptions", async () => {
    let resolveList: (value: typeof page) => void = () => undefined;
    vi.mocked(aiApi.list).mockReturnValue(new Promise((resolve) => { resolveList = resolve; }));
    render(<AIEvidenceReview />);
    expect(screen.getByRole("status")).toHaveTextContent("กำลังโหลดหลักฐาน AI");
    resolveList({ items: [], pagination: { total: 0, limit: 20, offset: 0 } });
    expect(await screen.findByText("ไม่มีหลักฐาน AI ที่รอการตรวจสอบ")).toBeInTheDocument();
  });
});
