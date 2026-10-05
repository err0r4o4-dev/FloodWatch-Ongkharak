import { afterEach, describe, expect, it, vi } from "vitest";
import { apiClient } from "../auth/api";
import { manualAnnouncementApi } from "./api";
afterEach(() => vi.restoreAllMocks());
describe("manual announcement API boundary", () => {
  it("rejects incomplete public envelopes", async () => {
    const request = vi.spyOn(apiClient, "request").mockResolvedValue({ items: [], total: 0 });
    await expect(manualAnnouncementApi.listPublic()).rejects.toThrow();
    expect(request).toHaveBeenCalledWith("/manual-announcements?limit=20&offset=0", expect.objectContaining({ signal: undefined }), { authenticate: false, retryUnauthorized: false });
  });
  it("accepts a real empty page", async () => {
    vi.spyOn(apiClient, "request").mockResolvedValue({ items: [], total: 0, limit: 20, offset: 0 });
    await expect(manualAnnouncementApi.listPublic()).resolves.toEqual({ items: [], total: 0, limit: 20, offset: 0 });
  });
  it("never accepts a draft or OFFICIAL media announcement publicly", async () => {
    vi.spyOn(apiClient, "request").mockResolvedValue({ items: [{ id: "draft", status: "DRAFT", sourceType: "OFFICIAL" }], total: 1, limit: 20, offset: 0 });
    await expect(manualAnnouncementApi.listPublic()).rejects.toThrow();
  });
  it("authenticates administrator requests", async () => {
    const request = vi.spyOn(apiClient, "request").mockResolvedValue({ items: [], total: 0, limit: 20, offset: 0 });
    await manualAnnouncementApi.adminList("DRAFT");
    expect(request).toHaveBeenCalledWith("/admin/manual-announcements?status=DRAFT&limit=20&offset=0", expect.objectContaining({ signal: undefined }));
  });
});
