import { describe, expect, it } from "vitest";
import type { AnnouncementInput } from "./api";
import { bangkokDateTimeValue, fromBangkokDateTime, isPublicReferenceUrl, publicationIssues } from "./validation";

const input: AnnouncementInput = {
  title: "ประกาศทดสอบ", summary: "สรุปจากแหล่งอ้างอิง", body: "รายละเอียดจากแหล่งอ้างอิง", advice: "", area: "พื้นที่ทดสอบ", severity: "WARNING",
  observedAt: "2026-10-04T14:00:00Z", expiresAt: "2026-10-05T14:00:00Z",
  references: [{ name: "สำนักข่าวทดสอบ", url: "https://news.example.com/article", observedAt: "2026-10-04T13:00:00Z" }]
};
const now = Date.parse("2026-10-04T15:00:00Z");
describe("manual announcement publication requirements", () => {
  it("requires source links and source-specific observation times", () => {
    expect(publicationIssues(input, now)).toEqual([]);
    expect(publicationIssues({ ...input, references: [] }, now)).toContain("references");
    expect(publicationIssues({ ...input, references: [{ ...input.references[0], observedAt: null }] }, now)).toContain("references");
  });
  it("rejects missing, future or expired time windows", () => {
    expect(publicationIssues({ ...input, observedAt: null }, now)).toContain("observedAt");
    expect(publicationIssues({ ...input, observedAt: "2026-10-06T00:00:00Z" }, now)).toContain("observedAt");
    expect(publicationIssues({ ...input, expiresAt: "2026-10-04T14:30:00Z" }, now)).toContain("expiresAt");
    expect(publicationIssues({ ...input, references: [{ ...input.references[0], observedAt: "2026-10-06T00:00:00Z" }] }, now)).toContain("references");
  });
  it("rejects credential-bearing and non-public links", () => {
    for (const value of ["javascript:alert(1)", "data:text/html,test", "https://user:password@news.example.com", "http://localhost/news", "http://127.0.0.1", "http://192.168.1.1", "http://[::1]", "http://internal", "https://news.local/news"]) expect(isPublicReferenceUrl(value), value).toBe(false);
    expect(isPublicReferenceUrl("https://news.example.com/article")).toBe(true);
  });
  it("converts Gregorian Bangkok fields independently of browser timezone", () => {
    expect(fromBangkokDateTime("2026-10-04T21:00")).toBe("2026-10-04T14:00:00.000Z");
    expect(bangkokDateTimeValue("2026-10-04T14:00:00Z")).toBe("2026-10-04T21:00");
    expect(fromBangkokDateTime("")).toBeNull();
    expect(bangkokDateTimeValue(null)).toBe("");
    expect(() => fromBangkokDateTime("2026-02-30T21:00")).toThrow();
    expect(() => fromBangkokDateTime("2569-10-04T21:00")).toThrow();
  });
});
