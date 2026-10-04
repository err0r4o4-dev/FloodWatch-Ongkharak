import { afterEach, describe, expect, it, vi } from "vitest";
import { reportsApi } from "./api";

const publicReport = {
  id: "report-1",
  type: "FLOOD",
  description: "น้ำท่วมขัง",
  location: { type: "Point", coordinates: [101.1, 14.2] },
  estimatedWaterDepthM: null,
  status: "VERIFIED",
  verificationStatus: "VERIFIED",
  sourceType: "COMMUNITY",
  createdAt: "2026-10-02T08:00:00Z",
  updatedAt: "2026-10-02T08:00:00Z",
  verifiedAt: "2026-10-02T09:00:00Z",
  verifiedBy: "admin-1",
  media: []
};

function response(body: unknown, status = 200): Response {
  return { ok: status >= 200 && status < 300, status, statusText: "Request failed", json: vi.fn().mockResolvedValue(body) } as unknown as Response;
}

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("reportsApi", () => {
  it("submits a guest name without bearer authentication and keeps the upload capability", async () => {
    const fetchMock = vi.fn().mockResolvedValue(response({ report: publicReport, uploadToken: "private-upload-capability" }, 201));
    vi.stubGlobal("fetch", fetchMock);
    const created = await reportsApi.create({ type: "FLOOD", description: "Water", location: { type: "Point", coordinates: [101, 14] }, displayName: "Guest" });
    expect(created.uploadToken).toBe("private-upload-capability");
    const options = fetchMock.mock.calls[0][1] as RequestInit;
    expect(new Headers(options.headers).has("Authorization")).toBe(false);
  });

  it("uses a report-scoped capability for guest uploads", async () => {
    const fetchMock = vi.fn().mockResolvedValue(response({ media: { id: "media-1", url: "/api/v1/reports/report-1/media/media-1" } }, 201));
    vi.stubGlobal("fetch", fetchMock);
    await reportsApi.uploadMedia("report-1", new File(["image"], "image.png", { type: "image/png" }), "private-upload-capability");
    const options = fetchMock.mock.calls[0][1] as RequestInit;
    expect(new Headers(options.headers).get("X-Report-Upload-Token")).toBe("private-upload-capability");
    expect(new Headers(options.headers).has("Authorization")).toBe(false);
  });

  it("accepts public-safe reports without an owner identifier", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(response({ items: [publicReport], limit: 20, offset: 0 })));

    await expect(reportsApi.listPublic()).resolves.toEqual({ items: [publicReport], limit: 20, offset: 0 });
  });

  it("parses upload metadata and sends the agreed multipart field", async () => {
    const fetchMock = vi.fn().mockResolvedValue(response({ media: { id: "media-1", url: "/api/v1/reports/report-1/media/media-1" } }, 201));
    vi.stubGlobal("fetch", fetchMock);

    await expect(reportsApi.uploadMedia("report-1", new File(["image"], "image.png", { type: "image/png" }))).resolves.toEqual({
      id: "media-1",
      url: "/api/v1/reports/report-1/media/media-1"
    });
    expect(fetchMock).toHaveBeenCalledWith("/api/v1/reports/report-1/media", expect.objectContaining({
      body: expect.any(FormData),
      method: "POST"
    }));
  });
});
