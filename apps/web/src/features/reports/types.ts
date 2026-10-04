export type ReportType = "FLOOD" | "ROAD" | "OTHER";
export type ReportStatus = "PENDING" | "VERIFIED" | "REJECTED";

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
  location: ReportLocation;
  estimatedWaterDepthM: number | null;
  status: ReportStatus;
  verificationStatus: ReportStatus;
  sourceType: "COMMUNITY";
  createdAt: string;
  updatedAt: string;
  verifiedAt: string | null;
  verifiedBy?: string | null;
  media: ReportMedia[];
};

export type ReportInput = Pick<CommunityReport, "type" | "description" | "location"> & {
  displayName?: string;
  estimatedWaterDepthM?: number;
};
