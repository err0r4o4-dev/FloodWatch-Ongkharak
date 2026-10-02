export type SourceType = "OFFICIAL" | "COMMUNITY" | "AI";
export type Freshness = "FRESH" | "STALE" | "EXPIRED" | "UNKNOWN";
export type SourceRef = { id: string; code: string; name: string; sourceType: SourceType };
export type VerificationStatus = "UNVERIFIED" | "VERIFIED" | "REJECTED";
