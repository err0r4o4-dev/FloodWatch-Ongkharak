import { describe, expect, it } from "vitest";
import { validateOfficialAnnouncementsResponse } from "./api";

const validResponse = {
  retrievedAt: "2026-10-04T01:00:00Z",
  source: {
    code: "TMD_CAP",
    name: "Thai Meteorological Department",
    sourceType: "OFFICIAL",
    url: "https://www.tmd.go.th",
    enabled: true,
    status: "AVAILABLE",
    freshness: "FRESH",
    lastSuccessAt: "2026-10-04T00:59:00Z"
  },
  items: [{
    id: "123e4567-e89b-12d3-a456-426614174000",
    identifier: "tmd-cap-1",
    headline: "ประกาศทดสอบ",
    description: "รายละเอียดประกาศทดสอบ",
    status: "UPDATED",
    effectiveStatus: "UNKNOWN",
    freshness: "FRESH",
    issuedAt: "2026-10-04T00:00:00Z",
    expiresAt: null,
    areaCodes: ["TH"],
    reason: "GEOGRAPHIC_SCOPE_UNCONFIRMED",
    historyTotal: 1,
    history: [{
      evidenceId: "123e4567-e89b-12d3-a456-426614174001",
      identifier: "tmd-cap-1",
      messageType: "Update",
      issuedAt: "2026-10-04T00:00:00Z",
      retrievedAt: "2026-10-04T00:59:00Z",
      expiresAt: null,
      headline: "ประกาศทดสอบ",
      description: "รายละเอียดประกาศทดสอบ",
      areaCodes: ["TH"],
      reason: null
    }]
  }],
  limit: 20,
  offset: 0,
  total: 1
};

describe("official announcement response validation", () => {
  it("accepts bounded nullable evidence and preserves UNKNOWN effective status", () => {
    expect(validateOfficialAnnouncementsResponse(validResponse)).toEqual(validResponse);
  });

  it("rejects unbounded lists and malformed optional timestamps", () => {
    expect(validateOfficialAnnouncementsResponse({ ...validResponse, limit: 51 })).toBeNull();
    expect(validateOfficialAnnouncementsResponse({
      ...validResponse,
      items: [{ ...validResponse.items[0], expiresAt: "not-a-timestamp" }]
    })).toBeNull();
  });
});
