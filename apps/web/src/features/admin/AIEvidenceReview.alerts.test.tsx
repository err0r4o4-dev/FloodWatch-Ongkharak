import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { AIEvidenceReview } from "./AIEvidenceReview";
import { confirmAction } from "../../shared/alerts";

vi.mock("../../shared/alerts", () => ({ confirmAction: vi.fn() }));

describe("AI evidence confirmations", () => {
  const fetchMock = vi.fn();
  beforeEach(() => {
    vi.clearAllMocks();
    vi.stubGlobal("fetch", fetchMock);
    fetchMock.mockResolvedValue({ ok: true, json: async () => ({ items: [{ id: "evidence", title: "หลักฐานทดสอบ", description: "ข้อมูลจำลองเฉพาะการทดสอบ", verificationStatus: "UNVERIFIED", createdAt: "2026-10-02T00:00:00Z" }] }) });
    vi.mocked(confirmAction).mockResolvedValue(false);
  });
  afterEach(() => { vi.unstubAllGlobals(); });

  it.each(["ยืนยัน (อนุมัติ)", "ปฏิเสธ (ลบ)"])("requires confirmation before %s", async (label) => {
    render(<AIEvidenceReview />);
    fireEvent.click(await screen.findByRole("button", { name: label }));
    await waitFor(() => expect(confirmAction).toHaveBeenCalledTimes(1));
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(screen.getByText("หลักฐานทดสอบ")).toBeInTheDocument();
  });

  it("throws a failed review to the shared error flow and retains evidence", async () => {
    let failure: unknown;
    vi.mocked(confirmAction).mockImplementation(async (options) => {
      try { await options.action(); } catch (cause) { failure = cause; }
      return false;
    });
    render(<AIEvidenceReview />);
    await screen.findByRole("button", { name: "ยืนยัน (อนุมัติ)" });
    fetchMock.mockResolvedValueOnce({ ok: false });
    fireEvent.click(screen.getByRole("button", { name: "ยืนยัน (อนุมัติ)" }));
    await waitFor(() => expect(failure).toBeInstanceOf(Error));
    expect(screen.getByText("หลักฐานทดสอบ")).toBeInTheDocument();
  });
});
