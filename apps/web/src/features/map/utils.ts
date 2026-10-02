import type { FloodFeature, MapBounds, MapGeometry, Position, RiskFeature, RoadCondition, RoadStatus } from "./types";

function validPosition(value: unknown): value is Position {
  return Array.isArray(value) && value.length === 2 &&
    typeof value[0] === "number" && Number.isFinite(value[0]) && value[0] >= -180 && value[0] <= 180 &&
    typeof value[1] === "number" && Number.isFinite(value[1]) && value[1] >= -90 && value[1] <= 90;
}

function validLine(value: unknown, minimum: number): value is Position[] {
  return Array.isArray(value) && value.length >= minimum && value.every(validPosition);
}

function validPolygon(value: unknown): value is Position[][] {
  return Array.isArray(value) && value.length > 0 && value.every((ring) =>
    validLine(ring, 4) && ring[0][0] === ring[ring.length - 1][0] && ring[0][1] === ring[ring.length - 1][1]);
}

export function validGeometry(value: unknown): value is MapGeometry {
  if (!value || typeof value !== "object" || !("type" in value) || !("coordinates" in value)) return false;
  const geometry = value as { type: unknown; coordinates: unknown };
  switch (geometry.type) {
    case "Point": return validPosition(geometry.coordinates);
    case "Polygon": return validPolygon(geometry.coordinates);
    case "MultiPolygon": return Array.isArray(geometry.coordinates) && geometry.coordinates.length > 0 && geometry.coordinates.every(validPolygon);
    case "MultiLineString": return Array.isArray(geometry.coordinates) && geometry.coordinates.length > 0 && geometry.coordinates.every((line) => validLine(line, 2));
    default: return false;
  }
}

export function validBounds(bounds: MapBounds): boolean {
  return Number.isFinite(bounds.west) && Number.isFinite(bounds.south) && Number.isFinite(bounds.east) && Number.isFinite(bounds.north) &&
    bounds.west >= -180 && bounds.east <= 180 && bounds.south >= -90 && bounds.north <= 90 &&
    bounds.west < bounds.east && bounds.south < bounds.north;
}

export function effectiveRoadStatus(condition: Pick<RoadCondition, "status" | "freshness" | "verificationStatus"> | null): RoadStatus {
  return condition?.freshness === "FRESH" && condition.verificationStatus === "VERIFIED" ? condition.status : "UNKNOWN";
}

export function effectiveFloodSeverity(feature: Pick<FloodFeature, "effectiveSeverity" | "freshness" | "verificationStatus">): string {
  return feature.freshness === "FRESH" && feature.verificationStatus === "VERIFIED" ? feature.effectiveSeverity : "UNKNOWN";
}

export function effectiveRiskLevel(feature: Pick<RiskFeature, "effectiveRiskLevel" | "freshness">): string {
  return feature.freshness === "FRESH" ? feature.effectiveRiskLevel : "UNKNOWN";
}

export function parseRoutePoint(point: { longitude: string; latitude: string }): Position | null {
  if (!point.longitude.trim() || !point.latitude.trim()) return null;
  const position: Position = [Number(point.longitude), Number(point.latitude)];
  return validPosition(position) ? position : null;
}
