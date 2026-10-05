import { beforeEach, describe, expect, it, vi } from "vitest";
import { apiClient } from "../auth/api";
import { fetchFM91News, validateFM91NewsResponse, type FM91NewsResponse } from "./api";

vi.mock("../auth/api", async () => {
  const actual = await vi.importActual<typeof import("../auth/api")>("../auth/api");
  return { ...actual, apiClient: { request: vi.fn() } };
});

const item = {
  id: "news-1",
  articleId: "123",
  title: "ฝนตกในพื้นที่องครักษ์",
  description: "ข้อความต้นฉบับจาก FM91",
  url: "https://fm91bkk.com/newsarticle/123",
  category: "ข่าวจราจร",
  publishedAt: null,
  observedAt: null,
  retrievedAt: "2026-10-05T01:00:00Z",
  updatedAt: "2026-10-05T01:00:00Z",
  historyTotal: 1,
  provenance: "MEDIA" as const,
  status: "PUBLISHED" as const,
  freshness: "UNKNOWN" as const,
  reason: "PUBLICATION_TIME_UNKNOWN",
  imageURLs: [],
  locations: ["องครักษ์"],
  roads: [],
  eventTimes: [],
  waterLevels: [],
  travelAdvice: [],
  matchedKeywords: [],
  latitude: null,
  longitude: null
};

const response: FM91NewsResponse = {
  retrievedAt: "2026-10-05T01:00:00Z",
  source: {
    code: "FM91_NEWS",
    name: "FM91",
    sourceType: "MEDIA",
    url: "https://fm91bkk.com/news",
    enabled: true,
    status: "AVAILABLE",
    freshness: "FRESH",
    lastSuccessAt: "2026-10-05T01:00:00Z",
    errorCode: null
  },
  items: [item],
  limit: 20,
  offset: 0,
  total: 1
};

beforeEach(() => vi.mocked(apiClient.request).mockReset());

describe("FM91 news API boundary", () => {
  it("accepts the media contract with nullable timestamps and coordinates", () => {
    expect(validateFM91NewsResponse(response)).toEqual(response);
  });

  it("requests only the public paginated Go API endpoint", async () => {
    vi.mocked(apiClient.request).mockResolvedValue(response);

    await expect(fetchFM91News(new AbortController().signal)).resolves.toEqual(response);
    expect(apiClient.request).toHaveBeenCalledWith(
      "/news-announcements?limit=20&offset=0",
      expect.objectContaining({ signal: expect.any(AbortSignal), cache: "no-store" }),
      { authenticate: false, retryUnauthorized: false }
    );
  });

  it("rejects unsafe article and image URLs instead of rendering them", () => {
    expect(validateFM91NewsResponse({
      ...response,
      items: [{ ...item, url: "javascript:alert(1)" }]
    })).toBeNull();
    expect(validateFM91NewsResponse({
      ...response,
      items: [{ ...item, imageURLs: ["https://evil.example/image.jpg"] }]
    })).toBeNull();
  });

  it("accepts both trusted FM91 image roots", () => {
    expect(validateFM91NewsResponse({
      ...response,
      items: [{
        ...item,
        imageURLs: [
          "https://fm91bkk.com/upload/news.jpg",
          "https://fm91bkk.com/ckfinder/userfiles/images/news.jpg"
        ]
      }]
    })).not.toBeNull();
  });

  it("rejects image path traversal before URL normalization", () => {
    for (const imageURL of [
      "https://fm91bkk.com/uploads/news.jpg",
      "https://fm91bkk.com/upload/../secret.jpg",
      "https://fm91bkk.com/upload/%2e%2e/secret.jpg",
      "https://fm91bkk.com/ckfinder/userfiles/images/%252e%252e/secret.jpg"
    ]) {
      expect(validateFM91NewsResponse({
        ...response,
        items: [{ ...item, imageURLs: [imageURL] }]
      })).toBeNull();
    }
  });

  it("enforces article identity, history, coordinate, and field bounds", () => {
    const invalidItems = [
      { ...item, articleId: "124" },
      { ...item, articleId: "12345678901234567890" },
      { ...item, historyTotal: 0 },
      { ...item, latitude: 14, longitude: null },
      { ...item, title: "t".repeat(2001) },
      { ...item, description: "d".repeat(30001) },
      { ...item, description: "ก".repeat(10001) },
      { ...item, category: "c".repeat(201) },
      { ...item, imageURLs: Array.from({ length: 21 }, (_, index) => `https://fm91bkk.com/upload/${index}.jpg`) },
      ...(["locations", "roads", "eventTimes", "waterLevels", "travelAdvice", "matchedKeywords"] as const).map((field) => ({
        ...item,
        [field]: Array.from({ length: 51 }, () => "ข้อความ")
      }))
    ];

    for (const invalidItem of invalidItems) {
      expect(validateFM91NewsResponse({ ...response, items: [invalidItem] })).toBeNull();
    }
  });

  it("accepts article descriptions up to the backend UTF-8 byte limit", () => {
    for (const description of ["d".repeat(2001), "d".repeat(30000), "ก".repeat(10000)]) {
      expect(validateFM91NewsResponse({ ...response, items: [{ ...item, description }] })).not.toBeNull();
    }
  });

  it("downgrades an item with unknown publication time without changing source freshness", () => {
    const validated = validateFM91NewsResponse({
      ...response,
      items: [{ ...item, freshness: "FRESH", publishedAt: null, reason: null }]
    });

    expect(validated?.source.freshness).toBe("FRESH");
    expect(validated?.items[0].freshness).toBe("UNKNOWN");
    expect(validated?.items[0].publishedAt).toBeNull();
    expect(validated?.items[0].reason).toBe("PUBLICATION_TIME_UNKNOWN");
  });
});
