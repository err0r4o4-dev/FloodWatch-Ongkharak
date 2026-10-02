import { apiConfig } from "../../shared/config/api";
import type { Freshness, SourceRef } from "../../shared/types/flood";

export type Event = {
  id: string; title: string; description: string; source: SourceRef;
  severity: string; effectiveSeverity: string; status: string; effectiveStatus: string;
  observedAt: string; freshness: Freshness; verificationStatus: string;
};
export type RiskArea = {
  id: string; name: string; description: string; source: SourceRef;
  riskLevel: string; effectiveRiskLevel: string; freshness: Freshness;
  validFrom: string | null; validTo: string | null;
};
export type Road = {
  id: string; name: string; currentCondition: {
    source: SourceRef | null; status: string; reportedStatus: string;
    observedAt: string | null; freshness: Freshness; verificationStatus: string;
  };
};
export type WaterStation = {
  id: string; name: string; source: SourceRef;
  latestReading: {
    source: SourceRef; observedAt: string; waterLevelM: number | null;
    flowRateM3s: number | null; rainfallMm: number | null;
    freshness: Freshness; verificationStatus: string;
  } | null;
};

export type Collections = { events: Event[]; risks: RiskArea[]; roads: Road[]; water: WaterStation[] };
export type CollectionKey = keyof Collections;
const paths: Record<CollectionKey, string> = {
  events: "/flood-events", risks: "/risk-areas", roads: "/roads", water: "/water-stations"
};

async function listItems<T>(path: string, limit: number, signal: AbortSignal): Promise<T[]> {
  const response = await fetch(`${apiConfig.baseUrl}${path}?limit=${limit}`, { signal });
  if (!response.ok) throw new Error(`HTTP ${response.status}`);
  const body: unknown = await response.json();
  if (!body || typeof body !== "object" || !("items" in body) || !Array.isArray(body.items)) {
    throw new Error("Invalid collection response");
  }
  return body.items as T[];
}

export async function loadCollection<Key extends CollectionKey>(key: Key, signal: AbortSignal): Promise<Collections[Key]> {
  if (key === "water") {
    const stations = await listItems<Omit<WaterStation, "latestReading">>(paths.water, 5, signal);
    const water = await Promise.all(stations.map(async (station) => {
      const readings = await listItems<NonNullable<WaterStation["latestReading"]>>(`/water-stations/${encodeURIComponent(station.id)}/readings`, 1, signal);
      return { ...station, latestReading: readings[0] ?? null };
    }));
    return water as Collections[Key];
  }
  return listItems(paths[key], 20, signal) as Promise<Collections[Key]>;
}
