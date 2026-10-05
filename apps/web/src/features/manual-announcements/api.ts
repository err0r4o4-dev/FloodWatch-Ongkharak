import { apiClient } from "../auth/api";
import { isPublicReferenceUrl } from "./validation";

export type Severity = "INFO" | "WATCH" | "WARNING" | "CRITICAL";
export type AnnouncementStatus = "DRAFT" | "PUBLISHED" | "EXPIRED" | "WITHDRAWN";
export type AnnouncementReference = { name: string; url: string; observedAt: string | null };
export type AnnouncementInput = {
  title: string; summary: string; body: string; advice: string; area: string; severity: Severity;
  observedAt: string | null; expiresAt: string | null; references: AnnouncementReference[];
};
export type ManualAnnouncement = AnnouncementInput & {
  id: string; sourceType: "COMMUNITY"; publisherType: "ADMIN"; referenceKind: "MEDIA";
  status: AnnouncementStatus; effectiveStatus: Severity | "UNKNOWN"; createdAt: string; updatedAt: string;
  publishedAt: string | null; withdrawnAt: string | null; withdrawalReason: string; revision: number;
};
export type AnnouncementHistory = {
  revision: number; action: "CREATED" | "UPDATED" | "PUBLISHED" | "WITHDRAWN";
  recordedAt: string; reason: string; announcement: ManualAnnouncement;
};
export type AnnouncementPage = { items: ManualAnnouncement[]; total: number; limit: number; offset: number };
export type AdminAnnouncementDetail = { announcement: ManualAnnouncement; history: AnnouncementHistory[] };

const severities = ["INFO", "WATCH", "WARNING", "CRITICAL"];
const statuses = ["DRAFT", "PUBLISHED", "EXPIRED", "WITHDRAWN"];
const uuidPattern = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
function object(value: unknown): value is Record<string, unknown> { return value !== null && typeof value === "object" && !Array.isArray(value); }
function timestamp(value: unknown): value is string { return typeof value === "string" && /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})$/.test(value) && Number.isFinite(Date.parse(value)); }
function nullableTimestamp(value: unknown) { return value === null || timestamp(value); }
function validAnnouncement(value: unknown): value is ManualAnnouncement {
  if (!object(value)) return false;
  return typeof value.id === "string" && uuidPattern.test(value.id) &&
    ["title", "summary", "body", "advice", "area", "withdrawalReason"].every((key) => typeof value[key] === "string") &&
    severities.includes(String(value.severity)) && statuses.includes(String(value.status)) && [...severities, "UNKNOWN"].includes(String(value.effectiveStatus)) &&
    value.sourceType === "COMMUNITY" && value.publisherType === "ADMIN" && value.referenceKind === "MEDIA" &&
    Number.isSafeInteger(value.revision) && Number(value.revision) > 0 && timestamp(value.createdAt) && timestamp(value.updatedAt) &&
    ["observedAt", "expiresAt", "publishedAt", "withdrawnAt"].every((key) => nullableTimestamp(value[key])) &&
    Array.isArray(value.references) && value.references.length <= 10 && value.references.every((reference) => object(reference) && typeof reference.name === "string" && typeof reference.url === "string" && (!reference.url || isPublicReferenceUrl(reference.url)) && nullableTimestamp(reference.observedAt));
}
function parsePage(value: unknown, publicOnly: boolean): AnnouncementPage {
  if (!object(value) || !Array.isArray(value.items) || !value.items.every((item) => validAnnouncement(item) && (!publicOnly || item.status !== "DRAFT")) ||
    !Number.isSafeInteger(value.total) || Number(value.total) < value.items.length || !Number.isSafeInteger(value.limit) || Number(value.limit) < 1 || Number(value.limit) > 50 ||
    !Number.isSafeInteger(value.offset) || Number(value.offset) < 0 || Number(value.offset) > 10000 || value.items.length > Number(value.limit)) throw new Error("Invalid manual announcement page.");
  return value as AnnouncementPage;
}
function parseMutation(value: unknown): { announcement: ManualAnnouncement } {
  if (!object(value) || !validAnnouncement(value.announcement)) throw new Error("Invalid manual announcement response.");
  return { announcement: value.announcement };
}
function parseDetail(value: unknown): AdminAnnouncementDetail {
  const { announcement } = parseMutation(value);
  if (!object(value) || !Array.isArray(value.history) || !value.history.every((entry) => object(entry) && Number.isSafeInteger(entry.revision) && Number(entry.revision) > 0 &&
    ["CREATED", "UPDATED", "PUBLISHED", "WITHDRAWN"].includes(String(entry.action)) && timestamp(entry.recordedAt) && typeof entry.reason === "string" && validAnnouncement(entry.announcement))) throw new Error("Invalid manual announcement history.");
  return { announcement, history: value.history as AnnouncementHistory[] };
}
const adminPath = "/admin/manual-announcements";
function itemPath(id: string) { return `${adminPath}/${encodeURIComponent(id)}`; }
async function write(path: string, method: string, body: unknown) {
  return parseMutation(await apiClient.request<unknown>(path, { method, headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) }));
}
export const manualAnnouncementApi = {
  async listPublic(offset = 0, signal?: AbortSignal) {
    return parsePage(await apiClient.request<unknown>(`/manual-announcements?limit=20&offset=${offset}`, { signal, cache: "no-store" }, { authenticate: false, retryUnauthorized: false }), true);
  },
  async adminList(status: AnnouncementStatus | "ALL", offset = 0, signal?: AbortSignal) {
    return parsePage(await apiClient.request<unknown>(`${adminPath}?status=${status}&limit=20&offset=${offset}`, { signal, cache: "no-store" }), false);
  },
  async adminGet(id: string, signal?: AbortSignal) { return parseDetail(await apiClient.request<unknown>(itemPath(id), { signal, cache: "no-store" })); },
  create(input: AnnouncementInput) { return write(adminPath, "POST", input); },
  update(id: string, input: AnnouncementInput, expectedRevision: number) { return write(itemPath(id), "PUT", { ...input, expectedRevision }); },
  publish(id: string, expectedRevision: number) { return write(`${itemPath(id)}/publish`, "POST", { expectedRevision }); },
  withdraw(id: string, expectedRevision: number, reason: string) { return write(`${itemPath(id)}/withdraw`, "POST", { expectedRevision, reason }); }
};
