import { apiConfig } from "../../shared/config/api";
import type { LayerItems, LayerKey, MapBounds, WaterReading } from "./types";
import { validBounds, validGeometry } from "./utils";

const paths: Record<LayerKey, string> = {
  flood: "/flood-events", water: "/water-stations", risk: "/risk-areas", roads: "/roads", reports: "/reports"
};
const pageLimit = 100;
const freshnessValues = ["FRESH", "STALE", "EXPIRED", "UNKNOWN"];
const sourceValues = ["OFFICIAL", "COMMUNITY", "AI"];
const roadValues = ["PASSABLE", "WATCH", "SMALL_VEHICLE_NOT_RECOMMENDED", "CLOSED", "UNKNOWN"];

function record(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}

function source(value: unknown): boolean {
  return record(value) && typeof value.id === "string" && typeof value.name === "string" && sourceValues.includes(String(value.sourceType));
}

function confidence(value: unknown): boolean {
  return value === null || (typeof value === "number" && Number.isFinite(value) && value >= 0 && value <= 1);
}

function validItem(key: LayerKey, value: unknown): boolean {
  if (!record(value) || typeof value.id !== "string") return false;
  const geometry = key === "water" || key === "reports" ? value.location : value.geometry;
  if (!validGeometry(geometry)) return false;
  if (key === "water") return geometry.type === "Point" && typeof value.name === "string" && source(value.source);
  if (key === "flood") return ["Point", "Polygon", "MultiPolygon"].includes(geometry.type) &&
    typeof value.title === "string" && source(value.source) && typeof value.observedAt === "string" &&
    typeof value.effectiveSeverity === "string" && freshnessValues.includes(String(value.freshness)) &&
    typeof value.verificationStatus === "string" && confidence(value.confidence);
  if (key === "risk") return geometry.type === "MultiPolygon" && typeof value.name === "string" && source(value.source) &&
    typeof value.effectiveRiskLevel === "string" && freshnessValues.includes(String(value.freshness));
  if (key === "reports") return geometry.type === "Point" && ["FLOOD", "ROAD", "OTHER"].includes(String(value.type)) &&
    typeof value.description === "string" && (value.estimatedWaterDepthM === null || typeof value.estimatedWaterDepthM === "number") &&
    value.status === "VERIFIED" && value.verificationStatus === "VERIFIED" && value.sourceType === "COMMUNITY" &&
    typeof value.createdAt === "string" && typeof value.updatedAt === "string" && (value.verifiedAt === null || typeof value.verifiedAt === "string") &&
    (value.verifiedBy === undefined || value.verifiedBy === null || typeof value.verifiedBy === "string") && Array.isArray(value.media) &&
    value.media.every((media) => record(media) && typeof media.id === "string" && typeof media.url === "string");
  if (geometry.type !== "MultiLineString" || typeof value.name !== "string" || !record(value.currentCondition)) return false;
  const condition = value.currentCondition;
  return roadValues.includes(String(condition.status)) && freshnessValues.includes(String(condition.freshness)) &&
    (condition.source === null || source(condition.source)) && confidence(condition.confidence);
}

export function formatBbox(bounds: MapBounds): string {
  if (!validBounds(bounds)) throw new Error("Invalid map bounds");
  return [bounds.west, bounds.south, bounds.east, bounds.north].join(",");
}

async function readPage<T>(url: string, signal: AbortSignal): Promise<{ items: T[]; hasMore: boolean }> {
  const response = await fetch(url, { signal });
  if (!response.ok) throw new Error(`Map API HTTP ${response.status}`);
  const payload: unknown = await response.json();
  if (!payload || typeof payload !== "object" || !("items" in payload) || !Array.isArray(payload.items)) {
    throw new Error("Invalid map API response");
  }
  return { items: payload.items as T[], hasMore: payload.items.length === pageLimit };
}

export async function fetchMapLayer<Key extends LayerKey>(key: Key, bounds: MapBounds, signal: AbortSignal): Promise<{ items: LayerItems[Key]; hasMore: boolean }> {
  const query = new URLSearchParams({ bbox: formatBbox(bounds), limit: String(pageLimit), offset: "0" });
  const page = await readPage<LayerItems[Key][number]>(`${apiConfig.baseUrl}${paths[key]}?${query}`, signal);
  if (!page.items.every((item) => validItem(key, item))) throw new Error("Invalid map geometry or evidence");
  return { items: page.items as LayerItems[Key], hasMore: page.hasMore };
}

export async function fetchLatestReading(stationId: string, signal: AbortSignal): Promise<WaterReading | null> {
  const page = await readPage<WaterReading>(`${apiConfig.baseUrl}/water-stations/${encodeURIComponent(stationId)}/readings?limit=1`, signal);
  const reading = page.items[0];
  if (!reading) return null;
  if (!record(reading) || !source(reading.source) || typeof reading.observedAt !== "string" ||
    !freshnessValues.includes(String(reading.freshness)) || typeof reading.verificationStatus !== "string" || !confidence(reading.confidence)) {
    throw new Error("Invalid water evidence");
  }
  return reading;
}
