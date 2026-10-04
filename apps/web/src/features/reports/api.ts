import { ApiError, apiClient } from "../auth/api";
import { derivePublicationStatus } from "./types";
import type { CommunityReport, ReportInput, ReportMedia, ReportReviewHistory, ReportStatus } from "./types";

export type ReportPage = { items: CommunityReport[]; limit: number; offset: number; total?: number };

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
    (value.displayName !== undefined && typeof value.displayName !== "string") ||
    (value.locationName !== undefined && typeof value.locationName !== "string") ||
    (value.locationNote !== undefined && typeof value.locationNote !== "string") ||
    (value.observedAt !== undefined && value.observedAt !== null && (typeof value.observedAt !== "string" || !Number.isFinite(Date.parse(value.observedAt)))) ||
    (value.waterTrend !== undefined && !["UNKNOWN", "RISING", "STABLE", "FALLING"].includes(String(value.waterTrend))) ||
    !["FLOOD", "ROAD", "OTHER"].includes(String(value.type)) || typeof value.description !== "string" ||
    !validPoint(value.location) || !["PENDING", "VERIFIED", "REJECTED"].includes(String(value.status)) ||
    !["PENDING", "VERIFIED", "REJECTED"].includes(String(value.verificationStatus)) || value.verificationStatus !== value.status ||
    value.sourceType !== "COMMUNITY" || (value.publicationStatus !== undefined && value.publicationStatus !== derivePublicationStatus(value.status as ReportStatus)) ||
    typeof value.createdAt !== "string" || typeof value.updatedAt !== "string" ||
    !Array.isArray(value.media)) return false;

  return (value.estimatedWaterDepthM === null || typeof value.estimatedWaterDepthM === "number" && Number.isFinite(value.estimatedWaterDepthM) && value.estimatedWaterDepthM >= 0 && value.estimatedWaterDepthM <= 100) &&
    (value.verifiedAt === null || typeof value.verifiedAt === "string") &&
    (value.verifiedBy === undefined || value.verifiedBy === null || typeof value.verifiedBy === "string") &&
    (value.reviewedAt === undefined || value.reviewedAt === null || typeof value.reviewedAt === "string") &&
    (value.reviewedBy === undefined || value.reviewedBy === null || typeof value.reviewedBy === "string") &&
    (value.rejectionReason === undefined || value.rejectionReason === null || typeof value.rejectionReason === "string") &&
    (value.reviewHistory === undefined || Array.isArray(value.reviewHistory) && value.reviewHistory.every(validReviewHistory)) &&
    value.media.every((media) => record(media) && typeof media.id === "string" && typeof media.url === "string");
}

function validReviewHistory(value: unknown): value is ReportReviewHistory {
  return record(value) && typeof value.id === "string" && ["VERIFY", "REMOVE", "RESTORE"].includes(String(value.action)) &&
    ["PENDING", "VERIFIED", "REJECTED"].includes(String(value.previousStatus)) && ["PENDING", "VERIFIED", "REJECTED"].includes(String(value.status)) &&
    typeof value.actorId === "string" && (value.reason === null || typeof value.reason === "string") && typeof value.createdAt === "string";
}

function normalizeReport(value: CommunityReport): CommunityReport {
  return { ...value, publicationStatus: derivePublicationStatus(value.status) };
}

function page(value: unknown): ReportPage {
  if (!record(value) || !Array.isArray(value.items) || typeof value.limit !== "number" || typeof value.offset !== "number" ||
    (value.total !== undefined && typeof value.total !== "number") || !value.items.every(validReport)) {
    throw new Error("Invalid reports API response");
  }
  return { items: value.items.map((item) => normalizeReport(item)), limit: value.limit, offset: value.offset, ...(value.total === undefined ? {} : { total: value.total }) };
}

function reportResponse(value: unknown): CommunityReport {
  if (!record(value) || !validReport(value.report)) throw new Error("Invalid report API response");
  return normalizeReport(value.report);
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
    const value = await apiClient.request<unknown>("/reports", {
      body: JSON.stringify(input),
      headers: { "Content-Type": "application/json" },
      method: "POST"
    }, { authenticate: false, retryUnauthorized: false });
    if (!record(value) || typeof value.uploadToken !== "string" || !value.uploadToken) throw new Error("Invalid guest report response");
    return { ...reportResponse(value), uploadToken: value.uploadToken };
  },

  async update(id: string, input: ReportInput) {
    return reportResponse(await apiClient.request<unknown>(reportPath(id), {
      body: JSON.stringify(input),
      headers: { "Content-Type": "application/json" },
      method: "PATCH"
    }));
  },

  async uploadMedia(id: string, file: File, uploadToken?: string) {
    const form = new FormData();
    form.append("file", file);
    return mediaResponse(await apiClient.request<unknown>(`${reportPath(id)}/media`, {
      body: form, method: "POST", ...(uploadToken ? { headers: { "X-Report-Upload-Token": uploadToken } } : {})
    }, { authenticate: false, retryUnauthorized: false }));
  },

  async listAdmin(status: ReportStatus, limit = 20, offset = 0, signal?: AbortSignal) {
    const query = new URLSearchParams({ status, limit: String(limit), offset: String(offset) });
    const result = await apiClient.request<unknown>(`/admin/reports?${query}`, { signal });
    return page(result);
  },

  async listPending(signal?: AbortSignal) {
    return this.listAdmin("PENDING", 20, 0, signal);
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
  },

  async restore(id: string, reason: string, expectedUpdatedAt: string) {
    return reportResponse(await apiClient.request<unknown>(`/admin/reports/${encodeURIComponent(id)}/restore`, {
      body: JSON.stringify({ expectedUpdatedAt, reason }),
      headers: { "Content-Type": "application/json" },
      method: "POST"
    }));
  }
};

export { ApiError };
