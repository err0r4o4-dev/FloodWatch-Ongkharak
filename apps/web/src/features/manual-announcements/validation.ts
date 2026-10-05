import type { AnnouncementInput } from "./api";

export function isPublicReferenceUrl(value: string): boolean {
  try {
    const url = new URL(value);
    const host = url.hostname.toLowerCase().replace(/\.$/, "");
    if (!["http:", "https:"].includes(url.protocol) || url.username || url.password || value.length > 2048) return false;
    if (!host.includes(".") || host.includes(":") || /(^|\.)(localhost|local|internal|test|invalid)$/.test(host)) return false;
    if (/^\d+\.\d+\.\d+\.\d+$/.test(host)) {
      const parts = host.split(".").map(Number);
      if (parts[0] === 0 || parts[0] === 10 || parts[0] === 127 || parts[0] >= 224 || (parts[0] === 169 && parts[1] === 254) || (parts[0] === 172 && parts[1] >= 16 && parts[1] <= 31) || (parts[0] === 192 && parts[1] === 168) || (parts[0] === 100 && parts[1] >= 64 && parts[1] <= 127)) return false;
    }
    return true;
  } catch { return false; }
}

export type PublicationIssue = "title" | "summary" | "body" | "area" | "observedAt" | "expiresAt" | "references";
export function publicationIssues(input: AnnouncementInput, now = Date.now()): PublicationIssue[] {
  const issues: PublicationIssue[] = [];
  for (const [field, maximum] of [["title", 200], ["summary", 1000], ["body", 10000], ["area", 200]] as const) {
    if (!input[field].trim() || Array.from(input[field].trim()).length > maximum) issues.push(field);
  }
  const observed = input.observedAt ? Date.parse(input.observedAt) : NaN;
  const expires = input.expiresAt ? Date.parse(input.expiresAt) : NaN;
  if (!Number.isFinite(observed) || observed > now) issues.push("observedAt");
  if (!Number.isFinite(expires) || expires <= now || expires <= observed) issues.push("expiresAt");
  if (input.references.length < 1 || input.references.length > 10 || input.references.some((reference) => {
    const referenceTime = reference.observedAt ? Date.parse(reference.observedAt) : NaN;
    return !reference.name.trim() || Array.from(reference.name.trim()).length > 200 || !isPublicReferenceUrl(reference.url) || !Number.isFinite(referenceTime) || referenceTime > now || referenceTime >= expires;
  })) issues.push("references");
  return issues;
}

export function bangkokDateTimeValue(value: string | null): string {
  if (!value || !Number.isFinite(Date.parse(value))) return "";
  return new Date(Date.parse(value) + 7 * 60 * 60 * 1000).toISOString().slice(0, 16);
}

export function fromBangkokDateTime(value: string): string | null {
  if (!value) return null;
  if (!/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}$/.test(value) || Number(value.slice(0, 4)) < 1900 || Number(value.slice(0, 4)) > 2100) throw new Error("Invalid Gregorian Bangkok date.");
  const result = new Date(`${value}:00+07:00`);
  if (!Number.isFinite(result.getTime()) || bangkokDateTimeValue(result.toISOString()) !== value) throw new Error("Invalid Gregorian Bangkok date.");
  return result.toISOString();
}
