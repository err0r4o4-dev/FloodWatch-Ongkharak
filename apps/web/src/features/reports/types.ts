export type ReportType = "FLOOD" | "ROAD" | "OTHER";
export type ReportStatus = "PENDING" | "VERIFIED" | "REJECTED";
export type PublicationStatus = "PUBLISHED" | "REMOVED";
export type ReviewAction = "VERIFY" | "REMOVE" | "RESTORE";
export type WaterTrend = "UNKNOWN" | "RISING" | "STABLE" | "FALLING";

export type ReportReviewHistory = {
  id: string;
  action: ReviewAction;
  previousStatus: ReportStatus;
  status: ReportStatus;
  actorId: string;
  reason: string | null;
  createdAt: string;
};

export type ReportLocation = {
  type: "Point";
  coordinates: [longitude: number, latitude: number];
};

export type ReportMedia = {
  id: string;
  url: string;
};

export type CommunityReport = {
  id: string;
  userId?: string;
  displayName?: string;
  type: ReportType;
  description: string;
  locationName?: string;
  locationNote?: string;
  location: ReportLocation;
  roadSegmentId?: string | null;
  roadSegmentAssignmentMethod?: "UNASSIGNED" | "USER_CONFIRMED" | "LEGACY_MATCH";
  estimatedWaterDepthM: number | null;
  observedAt?: string | null;
  waterTrend?: WaterTrend;
  status: ReportStatus;
  verificationStatus: ReportStatus;
  publicationStatus?: PublicationStatus;
  sourceType: "COMMUNITY";
  createdAt: string;
  updatedAt: string;
  verifiedAt: string | null;
  verifiedBy?: string | null;
  reviewedAt?: string | null;
  reviewedBy?: string | null;
  rejectionReason?: string | null;
  reviewHistory?: ReportReviewHistory[];
  media: ReportMedia[];
};

export type ReportInput = Pick<CommunityReport, "type" | "description" | "location"> & {
  locationName?: string;
  locationNote?: string;
  displayName?: string;
  estimatedWaterDepthM?: number;
  observedAt?: string;
  waterTrend?: WaterTrend;
  roadSegmentId?: string;
};

export function derivePublicationStatus(status: ReportStatus): PublicationStatus {
  return status === "REJECTED" ? "REMOVED" : "PUBLISHED";
}
