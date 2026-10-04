import { apiConfig } from "../../shared/config/api";

export type WeatherFreshness = "FRESH" | "STALE" | "UNKNOWN";

export type WeatherCurrent = {
  observedAt: string;
  intervalSeconds: number;
  temperatureC: number;
  apparentTemperatureC: number;
  humidityPercent: number;
  precipitationMm: number;
  weatherCode: number;
  windSpeedKmh: number;
  windDirectionDegrees: number;
  isDay: boolean;
};

export type WeatherResponse = {
  location: { name: "Ongkharak"; latitude: number; longitude: number };
  source: { name: string; url: string; kind: "MODEL" };
  freshness: WeatherFreshness;
  retrievedAt: string | null;
  reason: string | null;
  current: WeatherCurrent | null;
};

const weatherPath = `${apiConfig.baseUrl}/weather/ongkharak`;

function has(value: object, key: string): boolean {
  return Object.prototype.hasOwnProperty.call(value, key);
}

function isFiniteNumber(value: unknown): value is number {
  return typeof value === "number" && Number.isFinite(value);
}

function isUtcTimestamp(value: unknown): value is string {
  return typeof value === "string" && value.length > 0 && value.endsWith("Z") && !Number.isNaN(Date.parse(value));
}

function isNullableTimestamp(value: unknown): value is string | null {
  return value === null || isUtcTimestamp(value);
}

function within(value: number, minimum: number, maximum: number): boolean {
  return value >= minimum && value <= maximum;
}

export function isSafeProviderUrl(value: unknown): value is string {
  if (typeof value !== "string") return false;
  try {
    const url = new URL(value);
    return url.protocol === "https:" && url.hostname === "open-meteo.com" && url.port === "" && url.pathname === "/" && url.search === "" && url.hash === "" && url.username === "" && url.password === "";
  } catch {
    return false;
  }
}

function validateCurrent(value: unknown): WeatherCurrent | null {
  if (!value || typeof value !== "object") return null;
  const current = value as Record<string, unknown>;
  const required = ["observedAt", "intervalSeconds", "temperatureC", "apparentTemperatureC", "humidityPercent", "precipitationMm", "weatherCode", "windSpeedKmh", "windDirectionDegrees", "isDay"];
  if (!required.every((key) => has(current, key))) return null;
  const observedAt = current.observedAt;
  const intervalSeconds = current.intervalSeconds;
  const temperatureC = current.temperatureC;
  const apparentTemperatureC = current.apparentTemperatureC;
  const humidityPercent = current.humidityPercent;
  const precipitationMm = current.precipitationMm;
  const weatherCode = current.weatherCode;
  const windSpeedKmh = current.windSpeedKmh;
  const windDirectionDegrees = current.windDirectionDegrees;
  const isDay = current.isDay;
  if (!isUtcTimestamp(observedAt) || Date.parse(observedAt) > Date.now() + 60_000 ||
    !isFiniteNumber(intervalSeconds) || !Number.isInteger(intervalSeconds) || !within(intervalSeconds, 60, 3600) ||
    !isFiniteNumber(temperatureC) || !within(temperatureC, -100, 70) ||
    !isFiniteNumber(apparentTemperatureC) || !within(apparentTemperatureC, -100, 100) ||
    !isFiniteNumber(humidityPercent) || humidityPercent < 0 || humidityPercent > 100 ||
    !isFiniteNumber(precipitationMm) || !within(precipitationMm, 0, 1000) ||
    !isFiniteNumber(weatherCode) || !Number.isInteger(weatherCode) || !within(weatherCode, 0, 99) ||
    !isFiniteNumber(windSpeedKmh) || !within(windSpeedKmh, 0, 500) ||
    !isFiniteNumber(windDirectionDegrees) || windDirectionDegrees < 0 || windDirectionDegrees > 360 ||
    typeof isDay !== "boolean") return null;
  return {
    observedAt,
    intervalSeconds,
    temperatureC,
    apparentTemperatureC,
    humidityPercent,
    precipitationMm,
    weatherCode,
    windSpeedKmh,
    windDirectionDegrees,
    isDay
  };
}

export function validateWeatherResponse(value: unknown): WeatherResponse | null {
  if (!value || typeof value !== "object") return null;
  const body = value as Record<string, unknown>;
  if (!["location", "source", "freshness", "retrievedAt", "reason", "current"].every((key) => has(body, key))) return null;
  if (!body.location || typeof body.location !== "object" || !body.source || typeof body.source !== "object") return null;
  const location = body.location as Record<string, unknown>;
  const source = body.source as Record<string, unknown>;
  if (location.name !== "Ongkharak" || location.latitude !== 14.1 || location.longitude !== 101 ||
    source.name !== "Open-Meteo" || source.kind !== "MODEL" || !isSafeProviderUrl(source.url) ||
    !["FRESH", "STALE", "UNKNOWN"].includes(String(body.freshness)) || !isNullableTimestamp(body.retrievedAt) ||
    !(body.reason === null || (typeof body.reason === "string" && body.reason.length > 0))) return null;
  const current = body.current === null ? null : validateCurrent(body.current);
  if (body.current !== null && current === null) return null;
  const freshness = body.freshness as WeatherFreshness;
  const retrievedAt = body.retrievedAt;
  if ((freshness === "UNKNOWN") !== (current === null)) return null;
  if (current === null && retrievedAt !== null) return null;
  if (current && (retrievedAt === null || Date.parse(retrievedAt) + 60_000 < Date.parse(current.observedAt))) return null;
  return {
    location: { name: "Ongkharak", latitude: location.latitude, longitude: location.longitude },
    source: { name: source.name, url: source.url, kind: "MODEL" },
    freshness,
    retrievedAt,
    reason: body.reason,
    current
  };
}

export async function fetchWeather(signal: AbortSignal): Promise<WeatherResponse> {
  const response = await fetch(weatherPath, { signal });
  let body: unknown;
  try {
    body = await response.json();
  } catch {
    throw new Error("Invalid weather API response");
  }
  const weather = validateWeatherResponse(body);
  if (!weather || (!response.ok && response.status !== 503)) throw new Error(`Weather API HTTP ${response.status}`);
  return weather;
}
