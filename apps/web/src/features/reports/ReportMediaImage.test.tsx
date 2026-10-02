import { render, screen } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { apiClient } from "../auth/api";
import { ReportMediaImage } from "./ReportMediaImage";

afterEach(() => {
  apiClient.clearAccessToken();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

it("loads a pending report image through the authenticated root-relative API URL", async () => {
  const fetchMock = vi.fn().mockResolvedValue({ ok: true, status: 200, blob: vi.fn().mockResolvedValue(new Blob(["image"], { type: "image/png" })) });
  vi.stubGlobal("fetch", fetchMock);
  vi.stubGlobal("URL", { createObjectURL: vi.fn().mockReturnValue("blob:report-image"), revokeObjectURL: vi.fn() });
  apiClient.setAccessToken("access-token");

  render(<ReportMediaImage media={{ id: "media-1", url: "/api/v1/reports/report-1/media/media-1" }} />);

  expect(await screen.findByRole("img", { name: "ภาพประกอบรายงาน" })).toHaveAttribute("src", "blob:report-image");
  expect(fetchMock).toHaveBeenCalledWith("/api/v1/reports/report-1/media/media-1", expect.objectContaining({
    headers: expect.any(Headers)
  }));
  expect(((fetchMock.mock.calls[0][1] as RequestInit).headers as Headers).get("Authorization")).toBe("Bearer access-token");
});
