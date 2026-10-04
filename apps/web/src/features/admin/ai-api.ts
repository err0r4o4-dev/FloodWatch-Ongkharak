import { apiClient } from "../auth/api";

export const AI_EVIDENCE_PAGE_SIZE = 20;

export type AIEvidenceStatus = "UNVERIFIED" | "VERIFIED" | "REJECTED";
export type AIFloodSeverity = "INFO" | "WATCH" | "WARNING" | "CRITICAL" | "UNKNOWN";
export type AIRoadStatus = "PASSABLE" | "WATCH" | "SMALL_VEHICLE_NOT_RECOMMENDED" | "CLOSED" | "UNKNOWN";
export type AIBoundingBox = [number, number, number, number] | null;
export type AIFloodEvidence = {
  severity: AIFloodSeverity;
  water_depth_m: number | null;
  confidence: number;
  bounding_box: AIBoundingBox;
};
export type AIRoadEvidence = {
  status: AIRoadStatus;
  confidence: number;
  bounding_box: AIBoundingBox;
};
export type AIEvidence = {
  id: string;
  reportId: string;
  mediaId: string;
  sourceType: "AI";
  modelVersion: string;
  flood: AIFloodEvidence | null;
  road: AIRoadEvidence | null;
  observedAt: string;
  receivedAt: string;
  limitations: string[];
  status: AIEvidenceStatus;
};
export type AIEvidencePage = {
  items: AIEvidence[];
  pagination: { total: number; limit: number; offset: number };
};
export type AIEvidenceListOptions = {
  limit: number;
  offset: number;
  signal?: AbortSignal;
};
export type RoadOption = { id: string; name: string };

function record(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}

function nonNegativeInteger(value: unknown): value is number {
  return typeof value === "number" && Number.isInteger(value) && value >= 0;
}

function confidence(value: unknown): value is number {
  return typeof value === "number" && Number.isFinite(value) && value >= 0 && value <= 1;
}

function boundingBox(value: unknown): value is AIBoundingBox {
  return value === null || (Array.isArray(value) && value.length === 4 && value.every((coordinate) => typeof coordinate === "number" && Number.isFinite(coordinate) && coordinate >= 0 && coordinate <= 1) && value[0] <= value[2] && value[1] <= value[3]);
}

function flood(value: unknown): value is AIFloodEvidence {
  return record(value) && ["INFO", "WATCH", "WARNING", "CRITICAL", "UNKNOWN"].includes(String(value.severity)) &&
    (value.water_depth_m === null || (typeof value.water_depth_m === "number" && Number.isFinite(value.water_depth_m))) &&
    confidence(value.confidence) && boundingBox(value.bounding_box);
}

function road(value: unknown): value is AIRoadEvidence {
  return record(value) && ["PASSABLE", "WATCH", "SMALL_VEHICLE_NOT_RECOMMENDED", "CLOSED", "UNKNOWN"].includes(String(value.status)) && confidence(value.confidence) && boundingBox(value.bounding_box);
}

function parseEvidence(value: unknown): AIEvidence {
  if (!record(value) || typeof value.id !== "string" || typeof value.reportId !== "string" || typeof value.mediaId !== "string" || value.sourceType !== "AI" || typeof value.modelVersion !== "string" || (value.flood !== null && !flood(value.flood)) || (value.road !== null && !road(value.road)) || typeof value.observedAt !== "string" || typeof value.receivedAt !== "string" || !Array.isArray(value.limitations) || !value.limitations.every((limitation) => typeof limitation === "string") || !["UNVERIFIED", "VERIFIED", "REJECTED"].includes(String(value.status))) {
    throw new Error("Invalid AI evidence response");
  }
  return value as AIEvidence;
}

function parseEvidenceEnvelope(value: unknown): AIEvidence {
  if (!record(value) || !("evidence" in value)) {
    throw new Error("Invalid AI evidence response");
  }
  return parseEvidence(value.evidence);
}

function parsePage(value: unknown): AIEvidencePage {
  if (!record(value) || !Array.isArray(value.items) || !record(value.pagination) || !value.items.every((item) => {
    try { parseEvidence(item); return true; } catch { return false; }
  })) {
    throw new Error("Invalid AI evidence response");
  }
  const { total, limit, offset } = value.pagination;
  if (!nonNegativeInteger(total) || !nonNegativeInteger(limit) || limit === 0 || !nonNegativeInteger(offset)) {
    throw new Error("Invalid AI evidence response");
  }
  return { items: value.items.map(parseEvidence), pagination: { total, limit, offset } };
}

function parseRoads(value: unknown): RoadOption[] {
  if (!record(value) || !Array.isArray(value.items) || !value.items.every((item) => record(item) && typeof item.id === "string" && typeof item.name === "string")) {
    throw new Error("Invalid roads response");
  }
  return value.items as RoadOption[];
}

async function requestAction(path: string, body: unknown) {
  await apiClient.request<void>(path, {
    body: JSON.stringify(body),
    headers: { "Content-Type": "application/json" },
    method: "POST"
  });
}

export const aiApi = {
  async list({ limit, offset, signal }: AIEvidenceListOptions): Promise<AIEvidencePage> {
    const query = new URLSearchParams({ status: "UNVERIFIED", limit: String(limit), offset: String(offset) });
    return parsePage(await apiClient.request<unknown>(`/admin/ai-evidence?${query}`, { signal }));
  },

  async get(id: string): Promise<AIEvidence> {
    return parseEvidenceEnvelope(await apiClient.request<unknown>(`/admin/ai-evidence/${encodeURIComponent(id)}`));
  },

  async listRoads({ limit, offset, signal }: AIEvidenceListOptions): Promise<RoadOption[]> {
    const query = new URLSearchParams({ limit: String(limit), offset: String(offset) });
    return parseRoads(await apiClient.request<unknown>(`/roads?${query}`, { signal }));
  },

  async verify(id: string, input: { roadId?: string }): Promise<void> {
    await requestAction(`/admin/ai-evidence/${encodeURIComponent(id)}/verify`, input);
  },

  async reject(id: string, input: { reason: string }): Promise<void> {
    await requestAction(`/admin/ai-evidence/${encodeURIComponent(id)}/reject`, input);
  }
};
