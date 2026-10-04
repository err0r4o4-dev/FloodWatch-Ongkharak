import type { ReportArea, ReportAreaDepthBand, ReportAreaFreshness, ReportAreaTrend } from "./types";

const maxFreshAgeMs = 60 * 60 * 1000;

export function effectiveReportAreaFreshness(freshness: ReportAreaFreshness, latestObservedAt: string | null, now = Date.now()): ReportAreaFreshness {
  if (freshness !== "FRESH") return freshness;
  if (!latestObservedAt) return "UNKNOWN";
  const observedAt = Date.parse(latestObservedAt);
  const age = now - observedAt;
  return Number.isFinite(observedAt) && observedAt <= now && age >= 0 && age < maxFreshAgeMs ? "FRESH" : "UNKNOWN";
}

export function displayDepthBand(depthBand: ReportAreaDepthBand, freshness: ReportAreaFreshness, latestObservedAt: string | null = null, now = Date.now()): ReportAreaDepthBand {
  return effectiveReportAreaFreshness(freshness, latestObservedAt, now) === "FRESH" ? depthBand : "UNKNOWN";
}

export function areaDepthColor(depthBand: ReportAreaDepthBand, freshness: ReportAreaFreshness, latestObservedAt: string | null = null): string {
  const band = displayDepthBand(depthBand, freshness, latestObservedAt);
  return band === "VERY_HIGH" ? "var(--color-area-very-high)" : band === "HIGH" ? "var(--color-area-high)" :
    band === "MODERATE" ? "var(--color-area-moderate)" : band === "LOW" ? "var(--color-area-low)" : "var(--color-area-unknown)";
}

export function areaTrendSymbol(trend: ReportAreaTrend): string {
  return trend === "RISING" ? "↑" : trend === "FALLING" ? "↓" : trend === "STABLE" ? "→" : "?";
}

export function areaTrendTone(trend: ReportAreaTrend): string {
  return trend === "RISING" ? "rising" : trend === "FALLING" ? "falling" : trend === "STABLE" ? "stable" : "unknown";
}

export function displayReportAreaTrend(trend: ReportAreaTrend, freshness: ReportAreaFreshness, latestObservedAt: string | null = null, now = Date.now()): ReportAreaTrend {
  return effectiveReportAreaFreshness(freshness, latestObservedAt, now) === "FRESH" ? trend : "UNKNOWN";
}

export function areaMarkerLabel(area: ReportArea): string {
  return `${area.name} · ${displayDepthBand(area.depthBand, area.freshness, area.latestObservedAt)}`;
}

export function maskReportArea(area: ReportArea): ReportArea {
  return {
    ...area,
    name: "",
    depthBand: "UNKNOWN",
    previousDepthBand: "UNKNOWN",
    trend: "UNKNOWN",
    reportedTrend: "UNKNOWN",
    freshness: "UNKNOWN",
    summaryReason: "UPDATING",
    latestObservedAt: null,
    reportCount: 0,
    sampleCount: 0,
    totalReportCount: 0,
    reportIds: []
  };
}
