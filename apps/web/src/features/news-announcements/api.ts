import { apiClient } from "../auth/api";

export type FM91NewsFreshness = "FRESH" | "STALE" | "UNKNOWN";
export type FM91NewsSourceStatus = "NOT_CONFIGURED" | "AVAILABLE" | "DEGRADED" | "UNAVAILABLE";

export type FM91NewsItem = {
  id: string;
  articleId: string;
  title: string;
  description: string;
  url: string;
  category: string;
  publishedAt: string | null;
  observedAt: string | null;
  retrievedAt: string;
  updatedAt: string;
  historyTotal: number;
  provenance: "MEDIA";
  status: "PUBLISHED";
  freshness: FM91NewsFreshness;
  reason: string | null;
  imageURLs: string[];
  locations: string[];
  roads: string[];
  eventTimes: string[];
  waterLevels: string[];
  travelAdvice: string[];
  matchedKeywords: string[];
  latitude: number | null;
  longitude: number | null;
};

export type FM91NewsSource = {
  code: "FM91_NEWS";
  name: string;
  sourceType: "MEDIA";
  url: "https://fm91bkk.com/news";
  enabled: boolean;
  status: FM91NewsSourceStatus;
  freshness: FM91NewsFreshness;
  lastSuccessAt: string | null;
  errorCode: string | null;
};

export type FM91NewsResponse = {
  retrievedAt: string;
  source: FM91NewsSource;
  items: FM91NewsItem[];
  limit: number;
  offset: number;
  total: number;
};

const fm91NewsPath = "/news-announcements?limit=20&offset=0";
const freshnesses = ["FRESH", "STALE", "UNKNOWN"] as const;
const sourceStatuses = ["NOT_CONFIGURED", "AVAILABLE", "DEGRADED", "UNAVAILABLE"] as const;
const sourceUrl = "https://fm91bkk.com/news";
const articleIdPattern = /^\d{1,19}$/;
const trustedImagePathPrefixes = ["/upload/", "/ckfinder/userfiles/images/"] as const;

function isRecord(value: unknown): value is Record<string, unknown> {
  return Boolean(value) && typeof value === "object" && !Array.isArray(value);
}

function isUtcTimestamp(value: unknown): value is string {
  return typeof value === "string" && value.endsWith("Z") && !Number.isNaN(Date.parse(value));
}

function nullableTimestamp(value: unknown): value is string | null {
  return value === null || isUtcTimestamp(value);
}

function nullableText(value: unknown): value is string | null {
  return value === null || typeof value === "string";
}

function isFiniteNumber(value: unknown): value is number {
  return typeof value === "number" && Number.isFinite(value);
}

function nullableCoordinate(value: unknown, minimum: number, maximum: number): value is number | null {
  return value === null || (isFiniteNumber(value) && value >= minimum && value <= maximum);
}

function integerInRange(value: unknown, minimum: number, maximum: number): value is number {
  return typeof value === "number" && Number.isInteger(value) && value >= minimum && value <= maximum;
}

function stringArray(value: unknown, maximum: number): value is string[] {
  return Array.isArray(value) && value.length <= maximum && value.every((entry) => typeof entry === "string");
}

function boundedText(value: unknown, maximum: number): value is string {
  return typeof value === "string" && value.length <= maximum;
}

function hasRawPathTraversal(value: string): boolean {
  const schemeEnd = value.indexOf("://");
  const pathStart = schemeEnd >= 0 ? value.indexOf("/", schemeEnd + 3) : -1;
  if (pathStart < 0) return false;

  const rawPath = value.slice(pathStart).split(/[?#]/, 1)[0];
  if (rawPath.includes("\\")) return true;

  let decodedPath = rawPath;
  for (let attempt = 0; attempt < 3; attempt += 1) {
    let nextPath: string;
    try {
      nextPath = decodeURIComponent(decodedPath);
    } catch {
      return true;
    }
    if (nextPath === decodedPath) break;
    decodedPath = nextPath;
  }

  return decodedPath.split(/[\\/]/).some((segment) => segment === "." || segment === "..");
}

function articleIdFromUrl(value: string): string | null {
  if (hasRawPathTraversal(value)) return null;
  try {
    const url = new URL(value);
    if (url.protocol !== "https:" || url.hostname !== "fm91bkk.com" || url.port !== "" || url.username !== "" || url.password !== "" || url.search !== "" || url.hash !== "") return null;
    const match = /^\/newsarticle\/(\d{1,19})$/.exec(url.pathname);
    return match?.[1] ?? null;
  } catch {
    return null;
  }
}

export function isSafeFM91ArticleUrl(value: unknown): value is string {
  return typeof value === "string" && articleIdFromUrl(value) !== null;
}

export function isSafeFM91ImageUrl(value: unknown): value is string {
  if (typeof value !== "string") return false;
  if (hasRawPathTraversal(value)) return false;
  try {
    const url = new URL(value);
    return url.protocol === "https:" && url.hostname === "fm91bkk.com" && url.port === "" && url.username === "" && url.password === "" && url.search === "" && url.hash === "" && trustedImagePathPrefixes.some((prefix) => url.pathname.startsWith(prefix) && url.pathname.length > prefix.length);
  } catch {
    return false;
  }
}

function validateItem(value: unknown): FM91NewsItem | null {
  if (!isRecord(value) || typeof value.id !== "string" || !value.id || typeof value.articleId !== "string" || !articleIdPattern.test(value.articleId) ||
    !boundedText(value.title, 2000) || !boundedText(value.description, 30000) || new TextEncoder().encode(value.description).length > 30000 || !isSafeFM91ArticleUrl(value.url) || typeof value.category !== "string" || value.category.length > 200 ||
    !nullableTimestamp(value.publishedAt) || !nullableTimestamp(value.observedAt) || !isUtcTimestamp(value.retrievedAt) || !isUtcTimestamp(value.updatedAt) ||
    articleIdFromUrl(value.url) !== value.articleId || !Number.isInteger(value.historyTotal) || Number(value.historyTotal) < 1 || value.provenance !== "MEDIA" || value.status !== "PUBLISHED" ||
    !freshnesses.includes(value.freshness as FM91NewsFreshness) || !nullableText(value.reason) || !stringArray(value.imageURLs, 20) ||
    !value.imageURLs.every(isSafeFM91ImageUrl) || !stringArray(value.locations, 50) || !stringArray(value.roads, 50) || !stringArray(value.eventTimes, 50) ||
    !stringArray(value.waterLevels, 50) || !stringArray(value.travelAdvice, 50) || !stringArray(value.matchedKeywords, 50) ||
    !nullableCoordinate(value.latitude, -90, 90) || !nullableCoordinate(value.longitude, -180, 180) ||
    !((value.latitude === null && value.longitude === null) || (isFiniteNumber(value.latitude) && isFiniteNumber(value.longitude)))) return null;
  const freshness = value.publishedAt === null && value.freshness === "FRESH" ? "UNKNOWN" : value.freshness;
  return {
    id: value.id,
    articleId: value.articleId,
    title: value.title,
    description: value.description,
    url: value.url,
    category: value.category,
    publishedAt: value.publishedAt,
    observedAt: value.observedAt,
    retrievedAt: value.retrievedAt,
    updatedAt: value.updatedAt,
    historyTotal: value.historyTotal as number,
    provenance: "MEDIA",
    status: "PUBLISHED",
    freshness: freshness as FM91NewsFreshness,
    reason: value.reason ?? (freshness === "UNKNOWN" && value.publishedAt === null ? "PUBLICATION_TIME_UNKNOWN" : null),
    imageURLs: value.imageURLs,
    locations: value.locations,
    roads: value.roads,
    eventTimes: value.eventTimes,
    waterLevels: value.waterLevels,
    travelAdvice: value.travelAdvice,
    matchedKeywords: value.matchedKeywords,
    latitude: value.latitude,
    longitude: value.longitude
  };
}

function validateSource(value: unknown): FM91NewsSource | null {
  if (!isRecord(value) || value.code !== "FM91_NEWS" || typeof value.name !== "string" || value.sourceType !== "MEDIA" || value.url !== sourceUrl ||
    typeof value.enabled !== "boolean" || !sourceStatuses.includes(value.status as FM91NewsSourceStatus) || !freshnesses.includes(value.freshness as FM91NewsFreshness) ||
    !nullableTimestamp(value.lastSuccessAt) || !nullableText(value.errorCode)) return null;
  return {
    code: "FM91_NEWS",
    name: value.name,
    sourceType: "MEDIA",
    url: sourceUrl,
    enabled: value.enabled,
    status: value.status as FM91NewsSourceStatus,
    freshness: value.freshness as FM91NewsFreshness,
    lastSuccessAt: value.lastSuccessAt,
    errorCode: value.errorCode
  };
}

export function validateFM91NewsResponse(value: unknown): FM91NewsResponse | null {
  if (!isRecord(value) || !isUtcTimestamp(value.retrievedAt) || !integerInRange(value.limit, 1, 50) ||
    !integerInRange(value.offset, 0, 10_000) || !integerInRange(value.total, 0, Number.MAX_SAFE_INTEGER) ||
    !Array.isArray(value.items) || value.items.length > value.limit) return null;
  const source = validateSource(value.source);
  const items = value.items.map(validateItem);
  if (!source || items.some((item) => item === null)) return null;
  return {
    retrievedAt: value.retrievedAt,
    source,
    items: items as FM91NewsItem[],
    limit: value.limit,
    offset: value.offset,
    total: value.total
  };
}

export async function fetchFM91News(signal: AbortSignal): Promise<FM91NewsResponse> {
  const body: unknown = await apiClient.request(fm91NewsPath, { signal, cache: "no-store" }, { authenticate: false, retryUnauthorized: false });
  const data = validateFM91NewsResponse(body);
  if (!data) throw new Error("Invalid FM91 news API response");
  return data;
}
