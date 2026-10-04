import { apiConfig } from "../../shared/config/api";

export type ExternalDataKind = "OBSERVATION" | "FORECAST" | "OFFICIAL_ALERT";
export type ExternalDataProvenance = "OFFICIAL" | "MODEL";
export type ExternalDataStatus = "NOT_CONFIGURED" | "UNAVAILABLE" | "DEGRADED" | "AVAILABLE";
export type ExternalDataFreshness = "UNKNOWN" | "STALE" | "FRESH";

export type ExternalStation = {
  code: string;
  externalId: string;
  name: string;
  latitude: number;
  longitude: number;
  province: string;
  district: string;
  agency: string;
};

export type ExternalDataRecord = {
  key: string;
  kind: ExternalDataKind;
  provenance: ExternalDataProvenance;
  station: ExternalStation | null;
  observedAt: string | null;
  issuedAt: string | null;
  validFrom: string | null;
  validTo: string | null;
  model: string | null;
  member: string | null;
  metric: string | null;
  value: number | null;
  unit: string | null;
  datum: string | null;
  accumulationSeconds: number | null;
  latitude: number | null;
  longitude: number | null;
  headline: string | null;
  description: string | null;
  alertIdentifier: string | null;
  alertMessageType: string | null;
  alertReferences: string[];
  areaCodes: string[];
  geometry: unknown | null;
  reason: string | null;
  freshness: ExternalDataFreshness;
};

export type ExternalDataSource = {
  code: string;
  name: string;
  kind: ExternalDataKind;
  provenance: ExternalDataProvenance;
  url: string;
  enabled: boolean;
  status: ExternalDataStatus;
  freshness: ExternalDataFreshness;
  lastAttemptAt: string | null;
  lastSuccessAt: string | null;
  errorCode: string | null;
  recordCount: number;
  records: ExternalDataRecord[];
};

export type ExternalDataResponse = {
  retrievedAt: string;
  sources: ExternalDataSource[];
};

const externalDataPath = `${apiConfig.baseUrl}/external-data/ongkharak`;
const kinds = ["OBSERVATION", "FORECAST", "OFFICIAL_ALERT"] as const;
const provenances = ["OFFICIAL", "MODEL"] as const;
const statuses = ["NOT_CONFIGURED", "UNAVAILABLE", "DEGRADED", "AVAILABLE"] as const;
const freshnesses = ["UNKNOWN", "STALE", "FRESH"] as const;
const nullableRecordFields = ["station", "observedAt", "issuedAt", "validFrom", "validTo", "model", "member", "metric", "unit", "datum", "accumulationSeconds", "latitude", "longitude", "headline", "description", "alertIdentifier", "alertMessageType", "geometry", "reason"] as const;

function has(value: object, key: string): boolean {
  return Object.prototype.hasOwnProperty.call(value, key);
}

function isFiniteNumber(value: unknown): value is number {
  return typeof value === "number" && Number.isFinite(value);
}

function isUtcTimestamp(value: unknown): value is string {
  return typeof value === "string" && value.length > 0 && value.endsWith("Z") && !Number.isNaN(Date.parse(value));
}

function nullableTimestamp(value: unknown): value is string | null {
  return value === null || isUtcTimestamp(value);
}

function nullableText(value: unknown): value is string | null {
  return value === null || typeof value === "string";
}

function nullableNumber(value: unknown): value is number | null {
  return value === null || isFiniteNumber(value);
}

function inRange(value: number, minimum: number, maximum: number): boolean {
  return value >= minimum && value <= maximum;
}

function nullableField(value: Record<string, unknown>, key: typeof nullableRecordFields[number]): unknown {
  return has(value, key) ? value[key] : null;
}

export function isSafeProviderUrl(value: unknown): value is string {
  if (typeof value !== "string") return false;
  try {
    const url = new URL(value);
    return url.protocol === "https:" && url.port === "" && url.username === "" && url.password === "" && url.search === "" && url.hash === "" && ["www.thaiwater.net", "open-meteo.com", "www.tmd.go.th"].includes(url.hostname);
  } catch {
    return false;
  }
}

function validateStation(value: unknown): ExternalStation | null {
  if (!value || typeof value !== "object") return null;
  const station = value as Record<string, unknown>;
  if (!["code", "externalId", "name", "province", "district", "agency"].every((key) => typeof station[key] === "string")) return null;
  if (!isFiniteNumber(station.latitude) || !isFiniteNumber(station.longitude) || !inRange(station.latitude, -90, 90) || !inRange(station.longitude, -180, 180)) return null;
  return {
    code: station.code as string,
    externalId: station.externalId as string,
    name: station.name as string,
    latitude: station.latitude,
    longitude: station.longitude,
    province: station.province as string,
    district: station.district as string,
    agency: station.agency as string
  };
}

function validateArray(value: unknown): value is string[] {
  return Array.isArray(value) && value.every((item) => typeof item === "string");
}

function hasFiniteNumbers(value: unknown): boolean {
  if (typeof value === "number") return Number.isFinite(value);
  if (Array.isArray(value)) return value.every(hasFiniteNumbers);
  if (value && typeof value === "object") return Object.values(value).every(hasFiniteNumbers);
  return true;
}

function validateRecord(value: unknown, source: Pick<ExternalDataSource, "code" | "kind" | "provenance">): ExternalDataRecord | null {
  if (!value || typeof value !== "object") return null;
  const record = value as Record<string, unknown>;
  if (typeof record.key !== "string" || record.key.length === 0 || record.kind !== source.kind || record.provenance !== source.provenance || !freshnesses.includes(record.freshness as ExternalDataFreshness) || !has(record, "value") || !nullableNumber(record.value)) return null;
  const observedAt = nullableField(record, "observedAt");
  const issuedAt = nullableField(record, "issuedAt");
  const validFrom = nullableField(record, "validFrom");
  const validTo = nullableField(record, "validTo");
  const latitude = nullableField(record, "latitude");
  const longitude = nullableField(record, "longitude");
  const accumulationSeconds = nullableField(record, "accumulationSeconds");
  if (!nullableText(nullableField(record, "model")) || !nullableText(nullableField(record, "member")) || !nullableText(nullableField(record, "metric")) || !nullableText(nullableField(record, "unit")) || !nullableText(nullableField(record, "datum")) || !nullableText(nullableField(record, "headline")) || !nullableText(nullableField(record, "description")) || !nullableText(nullableField(record, "alertIdentifier")) || !nullableText(nullableField(record, "alertMessageType")) || !nullableText(nullableField(record, "reason"))) return null;
  if (!nullableTimestamp(observedAt) || !nullableTimestamp(issuedAt) || !nullableTimestamp(validFrom) || !nullableTimestamp(validTo)) return null;
  if (!nullableNumber(latitude) || !nullableNumber(longitude) || (latitude !== null && !inRange(latitude, -90, 90)) || (longitude !== null && !inRange(longitude, -180, 180))) return null;
  if (!nullableNumber(accumulationSeconds) || (accumulationSeconds !== null && (!Number.isInteger(accumulationSeconds) || accumulationSeconds < 0 || accumulationSeconds > 7 * 86400))) return null;
  if (!validateArray(has(record, "alertReferences") ? record.alertReferences : []) || !validateArray(has(record, "areaCodes") ? record.areaCodes : [])) return null;
  if (!hasFiniteNumbers(nullableField(record, "geometry"))) return null;

  const station = nullableField(record, "station") === null ? null : validateStation(nullableField(record, "station"));
  if (source.kind === "OBSERVATION" && (!station || !isUtcTimestamp(observedAt))) return null;
  if (source.kind === "FORECAST") {
    const reason = typeof record.reason === "string" ? record.reason.toLowerCase() : "";
    const isHiiUnknown = source.code === "HII_WRF" && record.model === "HII_WRF_ROMS" && record.metric === "precipitation" && record.value === null && record.freshness === "UNKNOWN" && validFrom === null && validTo === null && nullableField(record, "unit") === null && accumulationSeconds === null && reason.includes("unverified") && reason.includes("rainfall unit") && reason.includes("timezone") && reason.includes("accumulation window");
    if (source.code === "HII_WRF" && !isHiiUnknown) return null;
    if (record.station !== undefined && record.station !== null) return null;
    if (typeof record.model !== "string" || !isFiniteNumber(record.latitude) || !isFiniteNumber(record.longitude)) return null;
    if (validFrom === null && validTo === null && !isHiiUnknown) return null;
    if ((validFrom === null) !== (validTo === null) || (validFrom !== null && validTo !== null && Date.parse(validFrom) >= Date.parse(validTo))) return null;
  }
  if (source.kind === "OFFICIAL_ALERT") {
    if (typeof record.alertIdentifier !== "string" || typeof record.issuedAt !== "string" || typeof record.alertMessageType !== "string") return null;
    const messageType = record.alertMessageType.toLowerCase();
    if (!["alert", "update", "cancel"].includes(messageType) || (messageType !== "cancel" && record.validTo === null)) return null;
  }

  const metric = record.metric as string | null;
  const unit = record.unit as string | null;
  const period = accumulationSeconds as number | null;
  if (metric === "rain_1h" && (unit !== "mm" || period !== 3600)) return null;
  if (metric === "rain_24h" && (unit !== "mm" || period !== 86400)) return null;
  if (metric === "waterlevel_msl" && (unit !== "m" || record.datum !== "MSL")) return null;
  if (["waterlevel_m", "gate_in", "gate_out"].includes(String(metric)) && unit !== "m") return null;
  if (metric === "river_discharge" && (unit !== "m³/s" || (period !== null && period !== 0))) return null;
  if (source.kind === "FORECAST" && metric === "precipitation" && record.value !== null && (unit !== "mm" || !period || period <= 0)) return null;

  return {
    key: record.key,
    kind: record.kind as ExternalDataKind,
    provenance: record.provenance as ExternalDataProvenance,
    station,
    observedAt: observedAt as string | null,
    issuedAt: issuedAt as string | null,
    validFrom: validFrom as string | null,
    validTo: validTo as string | null,
    model: nullableField(record, "model") as string | null,
    member: nullableField(record, "member") as string | null,
    metric: nullableField(record, "metric") as string | null,
    value: record.value,
    unit: nullableField(record, "unit") as string | null,
    datum: nullableField(record, "datum") as string | null,
    accumulationSeconds: period,
    latitude: latitude as number | null,
    longitude: longitude as number | null,
    headline: nullableField(record, "headline") as string | null,
    description: nullableField(record, "description") as string | null,
    alertIdentifier: nullableField(record, "alertIdentifier") as string | null,
    alertMessageType: nullableField(record, "alertMessageType") as string | null,
    alertReferences: has(record, "alertReferences") ? record.alertReferences as string[] : [],
    areaCodes: has(record, "areaCodes") ? record.areaCodes as string[] : [],
    geometry: has(record, "geometry") ? record.geometry : null,
    reason: nullableField(record, "reason") as string | null,
    freshness: record.freshness as ExternalDataFreshness
  };
}

function validateSource(value: unknown): ExternalDataSource | null {
  if (!value || typeof value !== "object") return null;
  const source = value as Record<string, unknown>;
  if (typeof source.code !== "string" || typeof source.name !== "string" || !kinds.includes(source.kind as ExternalDataKind) || !provenances.includes(source.provenance as ExternalDataProvenance) || !isSafeProviderUrl(source.url) || typeof source.enabled !== "boolean" || !statuses.includes(source.status as ExternalDataStatus) || !freshnesses.includes(source.freshness as ExternalDataFreshness) || !has(source, "lastAttemptAt") || !has(source, "lastSuccessAt") || !nullableTimestamp(source.lastAttemptAt) || !nullableTimestamp(source.lastSuccessAt) || !has(source, "errorCode") || !nullableText(source.errorCode) || !isFiniteNumber(source.recordCount) || !Number.isInteger(source.recordCount) || source.recordCount < 0 || !Array.isArray(source.records) || source.records.length > 24) return null;
  const kind = source.kind as ExternalDataKind;
  const provenance = source.provenance as ExternalDataProvenance;
  const definition = { code: source.code, kind, provenance } as Pick<ExternalDataSource, "code" | "kind" | "provenance">;
  const records = source.records.map((record) => validateRecord(record, definition));
  if (records.some((record) => record === null)) return null;
  return {
    code: source.code,
    name: source.name,
    kind,
    provenance,
    url: source.url,
    enabled: source.enabled,
    status: source.status as ExternalDataStatus,
    freshness: source.freshness as ExternalDataFreshness,
    lastAttemptAt: source.lastAttemptAt,
    lastSuccessAt: source.lastSuccessAt,
    errorCode: source.errorCode,
    recordCount: source.recordCount as number,
    records: records as ExternalDataRecord[]
  };
}

export function validateExternalDataResponse(value: unknown): ExternalDataResponse | null {
  if (!value || typeof value !== "object") return null;
  const response = value as Record<string, unknown>;
  if (!isUtcTimestamp(response.retrievedAt) || !Array.isArray(response.sources) || response.sources.length > 10) return null;
  const sources = response.sources.map(validateSource);
  if (sources.some((source) => source === null)) return null;
  return { retrievedAt: response.retrievedAt as string, sources: sources as ExternalDataSource[] };
}

export async function fetchExternalData(signal: AbortSignal): Promise<ExternalDataResponse> {
  const response = await fetch(externalDataPath, { signal });
  let body: unknown;
  try {
    body = await response.json();
  } catch {
    throw new Error("Invalid external data API response");
  }
  const data = validateExternalDataResponse(body);
  if (!response.ok || !data) throw new Error(`External data API HTTP ${response.status}`);
  return data;
}
