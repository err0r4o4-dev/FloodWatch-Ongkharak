import { beforeEach, describe, expect, it, vi } from "vitest";
import { adminApi } from "./api";

const request = vi.hoisted(() => vi.fn());

vi.mock("../auth/api", () => ({
  apiClient: { request },
}));

describe("adminApi", () => {
  beforeEach(() => {
    request.mockReset();
  });

  it("loads dashboard stats through the shared authenticated API client", async () => {
    const stats = {
      totalUsers: 12,
      activeUsers: 9,
      totalReports: 7,
      pendingReports: 3,
      recentActivity: 5,
    };
    request.mockResolvedValue(stats);

    await expect(adminApi.getDashboardStats()).resolves.toEqual(stats);
    expect(request).toHaveBeenCalledWith(
      "/admin/dashboard/stats",
      expect.objectContaining({ headers: expect.any(Headers) })
    );
  });

  it("does not read access tokens from sessionStorage", async () => {
    const getItem = vi.spyOn(Storage.prototype, "getItem");
    request.mockResolvedValue({ users: [], pagination: { total: 0, limit: 50, offset: 0 } });

    await adminApi.listUsers();

    expect(getItem).not.toHaveBeenCalled();
    expect(request).toHaveBeenCalledWith(
      "/admin/users?",
      expect.objectContaining({ headers: expect.any(Headers) })
    );
    getItem.mockRestore();
  });
});
