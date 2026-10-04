import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError, apiClient } from "../auth/api";
import { confirmAction } from "../../shared/alerts";
import { aiApi } from "./ai-api";
import { AIEvidenceReview } from "./AIEvidenceReview";
import type { AIEvidence, AIEvidencePage } from "./ai-api";

vi.mock("../../shared/alerts", () => ({ confirmAction: vi.fn() }));
vi.mock("./ai-api", () => ({ AI_EVIDENCE_PAGE_SIZE: 20, aiApi: { list: vi.fn(), get: vi.fn(), listRoads: vi.fn(), verify: vi.fn(), reject: vi.fn() } }));

const floodEvidence: AIEvidence = {
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
const roadEvidence: AIEvidence = { ...floodEvidence, id: "evidence-road", road: { status: "CLOSED", confidence: 0.8, bounding_box: [0, 0, 1, 1] } };
const page: AIEvidencePage = { items: [floodEvidence], pagination: { total: 1, limit: 20, offset: 0 } };

function renderPage() {
  return render(<MemoryRouter><AIEvidenceReview /></MemoryRouter>);
}

describe("AIEvidenceReview", () => {
  afterEach(() => vi.restoreAllMocks());
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
    renderPage();
    expect(await screen.findByText("vision-1", { exact: false })).toBeInTheDocument();
    expect(screen.getByText("หลักฐาน AI")).toBeInTheDocument();
    expect(screen.getByText("น้ำท่วม: เตือนภัย · ความมั่นใจ 0%", { exact: false })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "ดูรายงานที่เกี่ยวข้อง" })).toHaveAttribute("href", "/reports?report=report-1");
    fireEvent.click(screen.getByRole("button", { name: "เปิดรายละเอียด: evidence-1" }));
    expect(await screen.findByText("รายละเอียดหลักฐาน")).toBeInTheDocument();
    expect(screen.getByText("เวลาของรายงานต้นทาง")).toBeInTheDocument();
    expect(screen.getAllByText("0%", { exact: false }).length).toBeGreaterThanOrEqual(2);
    expect(screen.getByText("ภาพมุมเดียว")).toBeInTheDocument();
    expect(screen.getAllByRole("link", { name: "ดูรายงานที่เกี่ยวข้อง" })[1]).toHaveAttribute("href", "/reports?report=report-1");
  });

  it("loads the source image only through the protected report media API and exposes failure", async () => {
    const loadMedia = vi.spyOn(apiClient, "requestBlob").mockRejectedValue(new Error("Private media unavailable"));
    renderPage();
    fireEvent.click(await screen.findByRole("button", { name: "เปิดรายละเอียด: evidence-1" }));
    expect(await screen.findByText("ไม่สามารถโหลดภาพประกอบได้")).toBeInTheDocument();
    expect(loadMedia).toHaveBeenCalledWith("/api/v1/reports/report-1/media/media-1");
    expect(screen.getByRole("button", { name: "อนุมัติหลักฐาน AI" })).toHaveClass("action-button");
  });

  it("requires an actual road selection before approving a road result", async () => {
    vi.mocked(aiApi.list).mockResolvedValue({ items: [roadEvidence], pagination: { total: 1, limit: 20, offset: 0 } });
    vi.mocked(aiApi.get).mockResolvedValue(roadEvidence);
    renderPage();
    fireEvent.click(await screen.findByRole("button", { name: "เปิดรายละเอียด: evidence-road" }));
    expect(await screen.findByRole("combobox", { name: "จับคู่ถนนสำหรับอนุมัติ" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "อนุมัติหลักฐาน AI" })).toBeDisabled();
    fireEvent.change(screen.getByRole("combobox", { name: "จับคู่ถนนสำหรับอนุมัติ" }), { target: { value: "road-1" } });
    expect(screen.getByRole("button", { name: "อนุมัติหลักฐาน AI" })).toBeEnabled();
  });

  it("runs the real review after confirmation and reloads the unverified list", async () => {
    vi.mocked(confirmAction).mockImplementation(async (options) => { await options.action(); return true; });
    renderPage();
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
    renderPage();
    fireEvent.click(await screen.findByRole("button", { name: "เปิดรายละเอียด: evidence-1" }));
    fireEvent.click(await screen.findByRole("button", { name: "อนุมัติหลักฐาน AI" }));

    expect(await screen.findByRole("alert")).toHaveTextContent("รายการนี้ถูกเปลี่ยนแปลงแล้ว");
    fireEvent.click(screen.getByRole("button", { name: "โหลดรายการอีกครั้ง" }));
    await waitFor(() => expect(aiApi.list).toHaveBeenCalledTimes(2));
  });

  it("shows loading, empty, and retry states without making guest assumptions", async () => {
    let resolveList: (value: typeof page) => void = () => undefined;
    vi.mocked(aiApi.list).mockReturnValue(new Promise((resolve) => { resolveList = resolve; }));
    renderPage();
    expect(screen.getByRole("status")).toHaveTextContent("กำลังโหลดหลักฐาน AI");
    resolveList({ items: [], pagination: { total: 0, limit: 20, offset: 0 } });
    expect(await screen.findByText("ไม่มีหลักฐาน AI ที่รอการตรวจสอบ")).toBeInTheDocument();
  });

  it("does not review evidence when confirmation is canceled", async () => {
    renderPage();
    fireEvent.click(await screen.findByRole("button", { name: "เปิดรายละเอียด: evidence-1" }));
    fireEvent.click(await screen.findByRole("button", { name: "อนุมัติหลักฐาน AI" }));

    await waitFor(() => expect(confirmAction).toHaveBeenCalledTimes(1));
    expect(aiApi.verify).not.toHaveBeenCalled();
    expect(screen.getByText("รายละเอียดหลักฐาน")).toBeInTheDocument();
  });

  it("keeps the detail open when a confirmed review request fails", async () => {
    let failure: unknown;
    vi.mocked(confirmAction).mockImplementation(async (options) => {
      try {
        await options.action();
      } catch (cause) {
        failure = cause;
      }
      return false;
    });
    vi.mocked(aiApi.verify).mockRejectedValue(new Error("offline"));
    renderPage();
    fireEvent.click(await screen.findByRole("button", { name: "เปิดรายละเอียด: evidence-1" }));
    fireEvent.click(await screen.findByRole("button", { name: "อนุมัติหลักฐาน AI" }));

    await waitFor(() => expect(failure).toBeInstanceOf(Error));
    expect(aiApi.verify).toHaveBeenCalledWith("evidence-1", {});
    expect(screen.getByText("รายละเอียดหลักฐาน")).toBeInTheDocument();
  });
});
