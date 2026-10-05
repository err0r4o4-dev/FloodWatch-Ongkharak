import { apiConfig } from "../../shared/config/api";
import { apiClient } from "../auth/api";
import { validBounds } from "../map/utils";
import type { MapBounds, Position } from "../map/types";
import type { RoadOption, RoadOptionPage, RoadSegment, RoadSegmentCandidate, RoadSegmentCandidateResponse, RoadSegmentGeometry, RoadSegmentPage } from "./types";

const defaultLimit = 100;

function record(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}

function position(value: unknown): value is Position {
  return Array.isArray(value) && value.length === 2 && typeof value[0] === "number" && Number.isFinite(value[0]) && value[0] >= -180 && value[0] <= 180 &&
    typeof value[1] === "number" && Number.isFinite(value[1]) && value[1] >= -90 && value[1] <= 90;
}

function lineString(value: unknown): value is RoadSegmentGeometry {
  return record(value) && value.type === "LineString" && Array.isArray(value.coordinates) && value.coordinates.length >= 2 && value.coordinates.every(position);
}

function point(value: unknown): boolean {
  return record(value) && value.type === "Point" && position(value.coordinates);
}

function segment(value: unknown): value is RoadSegment {
  return record(value) && typeof value.id === "string" && value.id.length > 0 && typeof value.roadId === "string" && value.roadId.length > 0 &&
    typeof value.name === "string" && typeof value.startLabel === "string" && typeof value.endLabel === "string" && lineString(value.geometry) && point(value.location) &&
    typeof value.version === "number" && Number.isInteger(value.version) && value.version >= 1 && (value.supersedesId === null || typeof value.supersedesId === "string") &&
    typeof value.active === "boolean" && typeof value.createdAt === "string" && Number.isFinite(Date.parse(value.createdAt));
}

function candidate(value: unknown): value is RoadSegmentCandidate {
  if (!segment(value)) return false;
  const distance = (value as RoadSegment & { distanceM?: unknown }).distanceM;
  return typeof distance === "number" && Number.isFinite(distance) && distance >= 0;
}

function page(value: unknown): RoadSegmentPage {
  if (!record(value) || !Array.isArray(value.items) || !value.items.every(segment) || typeof value.limit !== "number" || !Number.isInteger(value.limit) || value.limit < 1 ||
    typeof value.offset !== "number" || !Number.isInteger(value.offset) || value.offset < 0) throw new Error("Invalid road segment response");
  return { items: value.items, limit: value.limit, offset: value.offset };
}

export function formatRoadSegmentBbox(bounds: MapBounds): string {
  if (!validBounds(bounds)) throw new Error("Invalid map bounds");
  return [bounds.west, bounds.south, bounds.east, bounds.north].join(",");
}

async function read(url: string, signal?: AbortSignal): Promise<unknown> {
  const response = await fetch(url, { signal });
  if (!response.ok) throw new Error(`Road segment API HTTP ${response.status}`);
  return response.json();
}

export async function fetchRoadSegments(bounds: MapBounds | null, limit = defaultLimit, offset = 0, signal?: AbortSignal): Promise<RoadSegmentPage> {
  if (!Number.isInteger(limit) || limit < 1 || !Number.isInteger(offset) || offset < 0) throw new Error("Invalid road segment pagination");
  const query = new URLSearchParams({ limit: String(limit), offset: String(offset) });
  if (bounds) query.set("bbox", formatRoadSegmentBbox(bounds));
  const value = page(await read(`${apiConfig.baseUrl}/road-segments?${query}`, signal));
  return value;
}

export async function fetchRoadSegmentCandidates(longitude: number, latitude: number, signal?: AbortSignal): Promise<RoadSegmentCandidateResponse> {
  if (!position([longitude, latitude])) throw new Error("Invalid road segment candidate coordinates");
  const query = new URLSearchParams({ longitude: String(longitude), latitude: String(latitude) });
  const value = await read(`${apiConfig.baseUrl}/road-segments/candidates?${query}`, signal);
  if (!record(value) || !Array.isArray(value.items) || !value.items.every(candidate) ||
    !["UNASSIGNED", "SINGLE_CANDIDATE", "AMBIGUOUS"].includes(String(value.assignmentStatus)) || value.matchingDistanceM !== 30) {
    throw new Error("Invalid road segment candidate response");
  }
  return value as RoadSegmentCandidateResponse;
}

export async function listRoadOptions(signal?: AbortSignal): Promise<RoadOption[]> {
  return (await listRoadOptionPage(defaultLimit, 0, signal)).items;
}

function roadOption(value: unknown): value is RoadOption {
  return record(value) && typeof value.id === "string" && value.id.length > 0 && typeof value.name === "string" &&
    (value.code === undefined || typeof value.code === "string");
}

export async function listRoadOptionPage(limit = defaultLimit, offset = 0, signal?: AbortSignal): Promise<RoadOptionPage> {
  if (!Number.isInteger(limit) || limit < 1 || !Number.isInteger(offset) || offset < 0) throw new Error("Invalid road option pagination");
  const response = await apiClient.request<unknown>(`/roads?${new URLSearchParams({ limit: String(limit), offset: String(offset) })}`, { signal });
  if (!record(response) || !Array.isArray(response.items) || !response.items.every(roadOption)) throw new Error("Invalid roads response");
  const responseLimit = typeof response.limit === "number" && Number.isInteger(response.limit) && response.limit > 0 ? response.limit : limit;
  const responseOffset = typeof response.offset === "number" && Number.isInteger(response.offset) && response.offset >= 0 ? response.offset : offset;
  const hasMore = typeof response.hasMore === "boolean" ? response.hasMore : response.items.length === responseLimit;
  return { items: response.items, limit: responseLimit, offset: responseOffset, hasMore };
}

export async function createRoadSegment(input: { roadId: string; name: string; startLabel: string; endLabel: string; geometry: RoadSegmentGeometry; supersedesId?: string }): Promise<void> {
  await apiClient.request<void>("/road-segments", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(input) });
}

export const roadSegmentsApi = {
  candidates: fetchRoadSegmentCandidates,
  list: fetchRoadSegments,
  listRoads: listRoadOptions,
  listRoadOptionPage,
  create: createRoadSegment
};
