import { ApiError, apiClient } from "../auth/api";
import type { CommunityReport, ReportInput, ReportMedia } from "./types";

type ReportPage = { items: CommunityReport[]; limit: number; offset: number };

function record(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}

function validPoint(value: unknown): boolean {
  return record(value) && value.type === "Point" && Array.isArray(value.coordinates) && value.coordinates.length === 2 &&
    value.coordinates.every((coordinate) => typeof coordinate === "number" && Number.isFinite(coordinate)) &&
    Math.abs(value.coordinates[0] as number) <= 180 && Math.abs(value.coordinates[1] as number) <= 90;
}

function validReport(value: unknown): value is CommunityReport {
  if (!record(value) || typeof value.id !== "string" || (value.userId !== undefined && typeof value.userId !== "string") ||
    !["FLOOD", "ROAD", "OTHER"].includes(String(value.type)) || typeof value.description !== "string" ||
    !validPoint(value.location) || !["PENDING", "VERIFIED", "REJECTED"].includes(String(value.status)) ||
    !["PENDING", "VERIFIED", "REJECTED"].includes(String(value.verificationStatus)) ||
    value.sourceType !== "COMMUNITY" || typeof value.createdAt !== "string" || typeof value.updatedAt !== "string" ||
    !Array.isArray(value.media)) return false;

  return (value.estimatedWaterDepthM === null || typeof value.estimatedWaterDepthM === "number") &&
    (value.verifiedAt === null || typeof value.verifiedAt === "string") &&
    (value.verifiedBy === undefined || value.verifiedBy === null || typeof value.verifiedBy === "string") &&
    value.media.every((media) => record(media) && typeof media.id === "string" && typeof media.url === "string");
}

function page(value: unknown): ReportPage {
  if (!record(value) || !Array.isArray(value.items) || typeof value.limit !== "number" || typeof value.offset !== "number" || !value.items.every(validReport)) {
    throw new Error("Invalid reports API response");
  }
  return value as ReportPage;
}

function reportResponse(value: unknown): CommunityReport {
  if (!record(value) || !validReport(value.report)) throw new Error("Invalid report API response");
  return value.report;
}

function mediaResponse(value: unknown): ReportMedia {
  if (!record(value) || !record(value.media) || typeof value.media.id !== "string" || typeof value.media.url !== "string") {
    throw new Error("Invalid report media API response");
  }
  return value.media as ReportMedia;
}

function reportPath(id: string): string {
  return `/reports/${encodeURIComponent(id)}`;
}

export const reportsApi = {
  async listPublic({ bbox, signal }: { bbox?: string; signal?: AbortSignal } = {}) {
    const query = new URLSearchParams({ limit: "20", offset: "0" });
    if (bbox) query.set("bbox", bbox);
    const result = await apiClient.request<unknown>(`/reports?${query}`, { signal }, { authenticate: false, retryUnauthorized: false });
    return page(result);
  },

  async mine(signal?: AbortSignal) {
    const result = await apiClient.request<unknown>("/reports/mine", { signal });
    return page(result);
  },

  async create(input: ReportInput) {
    return reportResponse(await apiClient.request<unknown>("/reports", {
      body: JSON.stringify(input),
      headers: { "Content-Type": "application/json" },
      method: "POST"
    }));
  },

  async update(id: string, input: ReportInput) {
    return reportResponse(await apiClient.request<unknown>(reportPath(id), {
      body: JSON.stringify(input),
      headers: { "Content-Type": "application/json" },
      method: "PATCH"
    }));
  },

  async uploadMedia(id: string, file: File) {
    const form = new FormData();
    form.append("file", file);
    return mediaResponse(await apiClient.request<unknown>(`${reportPath(id)}/media`, { body: form, method: "POST" }));
  },

  async listPending(signal?: AbortSignal) {
    const result = await apiClient.request<unknown>("/admin/reports?status=PENDING", { signal });
    return page(result);
  },

  async getAdminReport(id: string) {
    return reportResponse(await apiClient.request<unknown>(`/admin/reports/${encodeURIComponent(id)}`));
  },

  async review(id: string, action: "verify" | "reject", reason: string, expectedUpdatedAt: string) {
    return reportResponse(await apiClient.request<unknown>(`/admin/reports/${encodeURIComponent(id)}/${action}`, {
      body: JSON.stringify({ expectedUpdatedAt, ...(action === "reject" && reason ? { reason } : {}) }),
      headers: { "Content-Type": "application/json" },
      method: "POST"
    }));
  }
};

export { ApiError };
