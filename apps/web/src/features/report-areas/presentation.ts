import type { ReportArea, ReportAreaDepthBand, ReportAreaFreshness, ReportAreaTrend } from "./types";

export function displayDepthBand(depthBand: ReportAreaDepthBand, freshness: ReportAreaFreshness): ReportAreaDepthBand {
  return freshness === "FRESH" ? depthBand : "UNKNOWN";
}

export function areaDepthColor(depthBand: ReportAreaDepthBand, freshness: ReportAreaFreshness): string {
  const band = displayDepthBand(depthBand, freshness);
  return band === "VERY_HIGH" ? "var(--color-area-very-high)" : band === "HIGH" ? "var(--color-area-high)" :
    band === "MODERATE" ? "var(--color-area-moderate)" : band === "LOW" ? "var(--color-area-low)" : "var(--color-area-unknown)";
}

export function areaTrendSymbol(trend: ReportAreaTrend): string {
  return trend === "RISING" ? "↑" : trend === "FALLING" ? "↓" : trend === "STABLE" ? "→" : "?";
}

export function areaTrendTone(trend: ReportAreaTrend): string {
  return trend === "RISING" ? "rising" : trend === "FALLING" ? "falling" : trend === "STABLE" ? "stable" : "unknown";
}

export function areaMarkerLabel(area: ReportArea): string {
  return `${area.name} · ${displayDepthBand(area.depthBand, area.freshness)}`;
}
