import type { MapBounds, Position } from "../map/types";

export type RoadSegmentGeometry = { type: "LineString"; coordinates: Position[] };
export type RoadSegmentLocation = { type: "Point"; coordinates: Position };

export type RoadSegment = {
  id: string;
  roadId: string;
  name: string;
  startLabel: string;
  endLabel: string;
  geometry: RoadSegmentGeometry;
  location: RoadSegmentLocation;
  version: number;
  supersedesId: string | null;
  active: boolean;
  createdAt: string;
};

export type RoadSegmentCandidate = RoadSegment & { distanceM: number };
export type RoadSegmentAssignmentStatus = "UNASSIGNED" | "SINGLE_CANDIDATE" | "AMBIGUOUS";
export type RoadSegmentCandidateResponse = {
  items: RoadSegmentCandidate[];
  assignmentStatus: RoadSegmentAssignmentStatus;
  matchingDistanceM: 30;
};
export type RoadSegmentPage = { items: RoadSegment[]; limit: number; offset: number };
export type RoadOption = { id: string; name: string; code?: string };
export type RoadOptionPage = { items: RoadOption[]; limit: number; offset: number; hasMore: boolean };
export type RoadSegmentListBounds = MapBounds | null;
