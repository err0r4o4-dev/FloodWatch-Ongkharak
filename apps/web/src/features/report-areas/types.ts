import type { CommunityReport } from "../reports/types";

export type ReportAreaDepthBand = "UNKNOWN" | "LOW" | "MODERATE" | "HIGH" | "VERY_HIGH";
export type ReportAreaTrend = "UNKNOWN" | "RISING" | "STABLE" | "FALLING";
export type ReportAreaFreshness = "FRESH" | "STALE" | "UNKNOWN";
export type ReportAreaSummaryReason = "SUMMARIZED" | "PROVISIONAL" | "NO_RECENT_REPORTS" | "INSUFFICIENT_MEASUREMENTS" | "CONFLICTING_REPORTS" | "UPDATING";
export type ReportAreaPoint = { type: "Point"; coordinates: [longitude: number, latitude: number] };
export type ReportAreaLineString = { type: "LineString"; coordinates: [longitude: number, latitude: number][] };

export type ReportArea = {
  id: string;
  name: string;
  location: ReportAreaPoint;
  radiusM: number;
  sourceType: "COMMUNITY";
  depthBand: ReportAreaDepthBand;
  previousDepthBand: ReportAreaDepthBand;
  trend: ReportAreaTrend;
  reportedTrend: ReportAreaTrend;
  freshness: ReportAreaFreshness;
  summaryReason: ReportAreaSummaryReason;
  latestObservedAt: string | null;
  computedAt: string;
  windowStart: string;
  windowEnd: string;
  reportCount: number;
  sampleCount: number;
  totalReportCount: number;
  reportIds: string[];
  areaType?: "ROAD_SEGMENT" | "LEGACY_RADIUS";
  geometry?: ReportAreaLineString | null;
  roadSegmentId?: string | null;
  segmentVersion?: number;
  segmentActive?: boolean;
  startLabel?: string;
  endLabel?: string;
};

export function isRoadSegmentReportArea(area: ReportArea): area is ReportArea & { areaType: "ROAD_SEGMENT"; geometry: ReportAreaLineString; roadSegmentId: string } {
  return area.areaType === "ROAD_SEGMENT" && area.geometry?.type === "LineString" && typeof area.roadSegmentId === "string";
}

export type ReportAreaTimelineWindow = {
  windowStart: string;
  windowEnd: string;
  depthBand: ReportAreaDepthBand;
  previousDepthBand: ReportAreaDepthBand;
  trend: ReportAreaTrend;
  reportedTrend: ReportAreaTrend;
  freshness: ReportAreaFreshness;
  summaryReason: ReportAreaSummaryReason;
  latestObservedAt: string | null;
  computedAt: string;
  reportCount: number;
  sampleCount: number;
  reportIds: string[];
};

export type ReportAreaDetail = { area: ReportArea; timeline: ReportAreaTimelineWindow[] };
export type ReportAreaPage = { items: ReportArea[]; limit: number; offset: number; hasMore: boolean };
export type ReportAreaReportsPage = { items: CommunityReport[]; limit: number; offset: number; hasMore: boolean };
