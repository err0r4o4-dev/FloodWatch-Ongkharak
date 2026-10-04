import { apiConfig } from "../../shared/config/api";
import type { MapBounds } from "../map/types";
import { validBounds } from "../map/utils";
import type { CommunityReport } from "../reports/types";
import type { ReportArea, ReportAreaDetail, ReportAreaPage, ReportAreaReportsPage, ReportAreaTimelineWindow } from "./types";

const pageLimit = 100;
const depthBands = ["UNKNOWN", "LOW", "MODERATE", "HIGH", "VERY_HIGH"];
const trends = ["UNKNOWN", "RISING", "STABLE", "FALLING"];
const freshness = ["FRESH", "STALE", "UNKNOWN"];
const summaryReasons = ["SUMMARIZED", "PROVISIONAL", "NO_RECENT_REPORTS", "INSUFFICIENT_MEASUREMENTS", "CONFLICTING_REPORTS", "UPDATING"];
const reportTypes = ["FLOOD", "ROAD", "OTHER"];
const reportStatuses = ["PENDING", "VERIFIED"];
const waterTrends = ["UNKNOWN", "RISING", "STABLE", "FALLING"];

function record(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}

function finiteNonNegative(value: unknown): value is number {
  return typeof value === "number" && Number.isFinite(value) && value >= 0;
}

function count(value: unknown): value is number {
  return finiteNonNegative(value) && Number.isInteger(value);
}

function iso(value: unknown): value is string {
  return typeof value === "string" && !Number.isNaN(new Date(value).getTime());
}

function point(value: unknown): value is ReportArea["location"] {
  if (!record(value) || value.type !== "Point" || !Array.isArray(value.coordinates) || value.coordinates.length !== 2) return false;
  const [longitude, latitude] = value.coordinates;
  return typeof longitude === "number" && Number.isFinite(longitude) && longitude >= -180 && longitude <= 180 &&
    typeof latitude === "number" && Number.isFinite(latitude) && latitude >= -90 && latitude <= 90;
}

function summaryFields(value: unknown): boolean {
  return record(value) && typeof value.windowStart === "string" && iso(value.windowStart) &&
    typeof value.windowEnd === "string" && iso(value.windowEnd) && new Date(value.windowStart).getTime() <= new Date(value.windowEnd).getTime() &&
    depthBands.includes(String(value.depthBand)) && depthBands.includes(String(value.previousDepthBand)) &&
    trends.includes(String(value.trend)) && trends.includes(String(value.reportedTrend)) &&
    freshness.includes(String(value.freshness)) && summaryReasons.includes(String(value.summaryReason)) &&
    (value.latestObservedAt === null || iso(value.latestObservedAt)) && iso(value.computedAt) &&
    count(value.reportCount) && count(value.sampleCount) && Array.isArray(value.reportIds) && value.reportIds.every((id) => typeof id === "string" && id.length > 0);
}

export function isValidReportArea(value: unknown): value is ReportArea {
  return summaryFields(value) && record(value) && typeof value.id === "string" && value.id.length > 0 &&
    typeof value.name === "string" && value.name.length > 0 && point(value.location) && value.radiusM === 150 &&
    value.sourceType === "COMMUNITY" && count(value.totalReportCount);
}

export function isValidReportAreaTimelineWindow(value: unknown): value is ReportAreaTimelineWindow {
  return summaryFields(value) && record(value);
}

function validReport(value: unknown): value is CommunityReport {
  if (!record(value) || typeof value.id !== "string" || !reportTypes.includes(String(value.type)) || typeof value.description !== "string" ||
    !point(value.location) || (value.estimatedWaterDepthM !== null && !finiteNonNegative(value.estimatedWaterDepthM)) ||
    !reportStatuses.includes(String(value.status)) || value.verificationStatus !== value.status || value.sourceType !== "COMMUNITY" ||
    (value.publicationStatus !== undefined && value.publicationStatus !== "PUBLISHED") || !iso(value.createdAt) || !iso(value.updatedAt) ||
    (value.verifiedAt !== null && !iso(value.verifiedAt)) || !Array.isArray(value.media)) return false;
  if (value.observedAt !== undefined && value.observedAt !== null && !iso(value.observedAt)) return false;
  if (value.waterTrend !== undefined && !waterTrends.includes(String(value.waterTrend))) return false;
  return (value.displayName === undefined || typeof value.displayName === "string") &&
    (value.locationName === undefined || typeof value.locationName === "string") &&
    (value.locationNote === undefined || typeof value.locationNote === "string") &&
    (value.verifiedBy === undefined || value.verifiedBy === null || typeof value.verifiedBy === "string") &&
    value.media.every((media) => record(media) && typeof media.id === "string" && typeof media.url === "string");
}

function pageMeta(value: unknown): boolean {
  return record(value) && count(value.limit) && value.limit >= 1 && count(value.offset) && value.limit <= pageLimit;
}

async function json(url: string, signal: AbortSignal): Promise<unknown> {
  const response = await fetch(url, { signal });
  if (!response.ok) throw new Error(`Report area API HTTP ${response.status}`);
  return response.json();
}

export function formatReportAreaBbox(bounds: MapBounds): string {
  if (!validBounds(bounds)) throw new Error("Invalid map bounds");
  return [bounds.west, bounds.south, bounds.east, bounds.north].join(",");
}

export async function fetchReportAreas(bounds: MapBounds, signal: AbortSignal): Promise<ReportAreaPage> {
  const query = new URLSearchParams({ bbox: formatReportAreaBbox(bounds), limit: String(pageLimit), offset: "0" });
  const value = await json(`${apiConfig.baseUrl}/report-areas?${query}`, signal);
  if (!record(value) || !pageMeta(value) || !Array.isArray(value.items) || !value.items.every(isValidReportArea)) throw new Error("Invalid report area response");
  const limit = value.limit;
  const offset = value.offset;
  if (!count(limit) || !count(offset)) throw new Error("Invalid report area response");
  return { items: value.items, limit, offset, hasMore: value.items.length >= limit };
}

export async function fetchReportAreaDetail(id: string, signal: AbortSignal): Promise<ReportAreaDetail> {
  const value = await json(`${apiConfig.baseUrl}/report-areas/${encodeURIComponent(id)}`, signal);
  if (!record(value) || !isValidReportArea(value.area) || !Array.isArray(value.timeline) || !value.timeline.every(isValidReportAreaTimelineWindow)) throw new Error("Invalid report area detail response");
  return { area: value.area, timeline: value.timeline };
}

export async function fetchReportAreaReports(id: string, limit = 20, offset = 0, signal?: AbortSignal): Promise<ReportAreaReportsPage> {
  if (!Number.isInteger(limit) || limit < 1 || limit > pageLimit || !Number.isInteger(offset) || offset < 0) throw new Error("Invalid report area pagination");
  const query = new URLSearchParams({ limit: String(limit), offset: String(offset) });
  const value = await json(`${apiConfig.baseUrl}/report-areas/${encodeURIComponent(id)}/reports?${query}`, signal ?? new AbortController().signal);
  if (!record(value) || !pageMeta(value) || !Array.isArray(value.items) || !value.items.every(validReport)) throw new Error("Invalid original report response");
  const responseLimit = value.limit;
  const responseOffset = value.offset;
  if (!count(responseLimit) || !count(responseOffset)) throw new Error("Invalid original report response");
  return { items: value.items, limit: responseLimit, offset: responseOffset, hasMore: value.items.length >= responseLimit };
}
