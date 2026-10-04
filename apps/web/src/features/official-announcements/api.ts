import { apiClient } from "../auth/api";

export type OfficialAnnouncementStatus = "ACTIVE" | "UPDATED" | "CANCELLED" | "EXPIRED";
export type OfficialAnnouncementEffectiveStatus = OfficialAnnouncementStatus | "UNKNOWN";
export type OfficialAnnouncementFreshness = "FRESH" | "STALE" | "UNKNOWN";
export type OfficialAnnouncementSourceStatus = "NOT_CONFIGURED" | "UNAVAILABLE" | "DEGRADED" | "AVAILABLE";
export type OfficialAnnouncementMessageType = "Alert" | "Update" | "Cancel";

export type OfficialAnnouncementHistory = {
  evidenceId: string;
  identifier: string;
  messageType: OfficialAnnouncementMessageType;
  issuedAt: string;
  retrievedAt: string;
  expiresAt: string | null;
  headline: string;
  description: string;
  areaCodes: string[];
  reason: string | null;
};

export type OfficialAnnouncementItem = {
  id: string;
  identifier: string;
  headline: string;
  description: string;
  status: OfficialAnnouncementStatus;
  effectiveStatus: OfficialAnnouncementEffectiveStatus;
  freshness: OfficialAnnouncementFreshness;
  issuedAt: string;
  expiresAt: string | null;
  areaCodes: string[];
  reason: string | null;
  historyTotal: number;
  history: OfficialAnnouncementHistory[];
};

export type OfficialAnnouncementSource = {
  code: "TMD_CAP";
  name: string;
  sourceType: "OFFICIAL";
  url: string;
  enabled: boolean;
  status: OfficialAnnouncementSourceStatus;
  freshness: OfficialAnnouncementFreshness;
  lastSuccessAt: string | null;
};

export type OfficialAnnouncementsResponse = {
  retrievedAt: string;
  source: OfficialAnnouncementSource;
  items: OfficialAnnouncementItem[];
  limit: number;
  offset: number;
  total: number;
};

const officialAnnouncementsPath = "/official-announcements?limit=20&offset=0";
const statuses = ["ACTIVE", "UPDATED", "CANCELLED", "EXPIRED"] as const;
const effectiveStatuses = [...statuses, "UNKNOWN"] as const;
const freshnesses = ["FRESH", "STALE", "UNKNOWN"] as const;
const sourceStatuses = ["NOT_CONFIGURED", "UNAVAILABLE", "DEGRADED", "AVAILABLE"] as const;
const messageTypes = ["Alert", "Update", "Cancel"] as const;
const uuidPattern = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

function isRecord(value: unknown): value is Record<string, unknown> {
  return Boolean(value) && typeof value === "object";
}

function isUtcTimestamp(value: unknown): value is string {
  return typeof value === "string" && value.endsWith("Z") && !Number.isNaN(Date.parse(value));
}

function nullableTimestamp(value: unknown): value is string | null {
  return value === null || isUtcTimestamp(value);
}

function nullableText(value: unknown): value is string | null {
  return value === null || typeof value === "string";
}

function boundedStringArray(value: unknown, maximum: number): value is string[] {
  return Array.isArray(value) && value.length <= maximum && value.every((item) => typeof item === "string" && item.length <= 256);
}

function isIntegerInRange(value: unknown, minimum: number, maximum: number): value is number {
  return typeof value === "number" && Number.isInteger(value) && value >= minimum && value <= maximum;
}

function validateHistory(value: unknown): OfficialAnnouncementHistory | null {
  if (!isRecord(value) || !uuidPattern.test(String(value.evidenceId)) || typeof value.identifier !== "string" ||
    !messageTypes.includes(value.messageType as OfficialAnnouncementMessageType) || !isUtcTimestamp(value.issuedAt) ||
    !isUtcTimestamp(value.retrievedAt) || !nullableTimestamp(value.expiresAt) || typeof value.headline !== "string" ||
    typeof value.description !== "string" || !boundedStringArray(value.areaCodes, 100) || !nullableText(value.reason)) return null;
  return {
    evidenceId: value.evidenceId as string,
    identifier: value.identifier,
    messageType: value.messageType as OfficialAnnouncementMessageType,
    issuedAt: value.issuedAt,
    retrievedAt: value.retrievedAt,
    expiresAt: value.expiresAt,
    headline: value.headline,
    description: value.description,
    areaCodes: value.areaCodes,
    reason: value.reason
  };
}

function validateItem(value: unknown): OfficialAnnouncementItem | null {
  if (!isRecord(value) || !uuidPattern.test(String(value.id)) || typeof value.identifier !== "string" ||
    typeof value.headline !== "string" || typeof value.description !== "string" || !statuses.includes(value.status as OfficialAnnouncementStatus) ||
    !effectiveStatuses.includes(value.effectiveStatus as OfficialAnnouncementEffectiveStatus) ||
    !freshnesses.includes(value.freshness as OfficialAnnouncementFreshness) || !isUtcTimestamp(value.issuedAt) ||
    !nullableTimestamp(value.expiresAt) || !boundedStringArray(value.areaCodes, 100) || !nullableText(value.reason) ||
    !isIntegerInRange(value.historyTotal, 0, Number.MAX_SAFE_INTEGER) || !Array.isArray(value.history) || value.history.length > 20) return null;
  const history = value.history.map(validateHistory);
  if (history.some((entry) => entry === null) || value.historyTotal < history.length) return null;
  return {
    id: value.id as string,
    identifier: value.identifier,
    headline: value.headline,
    description: value.description,
    status: value.status as OfficialAnnouncementStatus,
    effectiveStatus: value.effectiveStatus as OfficialAnnouncementEffectiveStatus,
    freshness: value.freshness as OfficialAnnouncementFreshness,
    issuedAt: value.issuedAt,
    expiresAt: value.expiresAt,
    areaCodes: value.areaCodes,
    reason: value.reason,
    historyTotal: value.historyTotal,
    history: history as OfficialAnnouncementHistory[]
  };
}

function validateSource(value: unknown): OfficialAnnouncementSource | null {
  if (!isRecord(value) || value.code !== "TMD_CAP" || typeof value.name !== "string" || value.sourceType !== "OFFICIAL" ||
    typeof value.url !== "string" || value.url !== "https://www.tmd.go.th" || typeof value.enabled !== "boolean" ||
    !sourceStatuses.includes(value.status as OfficialAnnouncementSourceStatus) ||
    !freshnesses.includes(value.freshness as OfficialAnnouncementFreshness) || !nullableTimestamp(value.lastSuccessAt)) return null;
  return {
    code: "TMD_CAP",
    name: value.name,
    sourceType: "OFFICIAL",
    url: value.url,
    enabled: value.enabled,
    status: value.status as OfficialAnnouncementSourceStatus,
    freshness: value.freshness as OfficialAnnouncementFreshness,
    lastSuccessAt: value.lastSuccessAt
  };
}

export function validateOfficialAnnouncementsResponse(value: unknown): OfficialAnnouncementsResponse | null {
  if (!isRecord(value) || !isUtcTimestamp(value.retrievedAt) || !isIntegerInRange(value.limit, 1, 50) ||
    !isIntegerInRange(value.offset, 0, Number.MAX_SAFE_INTEGER) || !isIntegerInRange(value.total, 0, Number.MAX_SAFE_INTEGER) ||
    !Array.isArray(value.items) || value.items.length > value.limit) return null;
  const source = validateSource(value.source);
  const items = value.items.map(validateItem);
  if (!source || items.some((item) => item === null)) return null;
  return {
    retrievedAt: value.retrievedAt,
    source,
    items: items as OfficialAnnouncementItem[],
    limit: value.limit,
    offset: value.offset,
    total: value.total
  };
}

export async function fetchOfficialAnnouncements(signal: AbortSignal): Promise<OfficialAnnouncementsResponse> {
  const body: unknown = await apiClient.request(officialAnnouncementsPath, { signal }, { authenticate: false, retryUnauthorized: false });
  const data = validateOfficialAnnouncementsResponse(body);
  if (!data) throw new Error("Invalid official announcements API response");
  return data;
}
