import type { Freshness, SourceRef, VerificationStatus } from "../../shared/types/flood";

export type Position = [longitude: number, latitude: number];
export type PointGeometry = { type: "Point"; coordinates: Position };
export type PolygonGeometry = { type: "Polygon"; coordinates: Position[][] };
export type MultiPolygonGeometry = { type: "MultiPolygon"; coordinates: Position[][][] };
export type MultiLineGeometry = { type: "MultiLineString"; coordinates: Position[][] };
export type MapGeometry = PointGeometry | PolygonGeometry | MultiPolygonGeometry | MultiLineGeometry;
export type MapBounds = { west: number; south: number; east: number; north: number };
export type LayerKey = "flood" | "water" | "risk" | "roads";
export type LayerVisibility = Record<LayerKey, boolean>;
export type MapMode = "SITUATION" | "ROUTE";

export type FloodFeature = {
  id: string; title: string; description: string; geometry: PointGeometry | PolygonGeometry | MultiPolygonGeometry;
  source: SourceRef; severity: string; effectiveSeverity: string; status: string; effectiveStatus: string;
  observedAt: string; freshness: Freshness; verificationStatus: VerificationStatus; confidence: number | null;
};
export type WaterFeature = {
  id: string; name: string; location: PointGeometry; source: SourceRef; active: boolean;
};
export type RiskFeature = {
  id: string; name: string; description: string; geometry: MultiPolygonGeometry; source: SourceRef;
  riskLevel: string; effectiveRiskLevel: string; freshness: Freshness;
  validFrom: string | null; validTo: string | null;
};
export type RoadStatus = "PASSABLE" | "WATCH" | "SMALL_VEHICLE_NOT_RECOMMENDED" | "CLOSED" | "UNKNOWN";
export type RoadCondition = {
  source: SourceRef | null; status: RoadStatus; reportedStatus?: RoadStatus;
  observedAt: string | null; freshness: Freshness; verificationStatus?: VerificationStatus;
  confidence: number | null; description?: string; validUntil: string | null;
};
export type RoadFeature = {
  id: string; name: string; geometry: MultiLineGeometry; currentCondition: RoadCondition;
};
export type WaterReading = {
  source: SourceRef; observedAt: string; freshness: Freshness;
  verificationStatus: VerificationStatus; confidence: number | null;
  waterLevelM: number | null; flowRateM3s: number | null; rainfallMm: number | null;
};
export type LayerItems = {
  flood: FloodFeature[]; water: WaterFeature[]; risk: RiskFeature[]; roads: RoadFeature[];
};
export type MapSelection =
  | { layer: "flood"; feature: FloodFeature }
  | { layer: "water"; feature: WaterFeature }
  | { layer: "risk"; feature: RiskFeature }
  | { layer: "roads"; feature: RoadFeature };

export const layerKeys: LayerKey[] = ["flood", "water", "risk", "roads"];
export const layerLabels: Record<LayerKey, string> = {
  flood: "เหตุการณ์น้ำ", water: "สถานีวัดน้ำ", risk: "พื้นที่เสี่ยง", roads: "สภาพถนน"
};
