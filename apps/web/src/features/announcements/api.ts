import { apiClient } from "../auth/api";

export type PublicAlert = {
  id: string;
  ruleId: string;
  severity: "INFO" | "WATCH" | "WARNING" | "CRITICAL";
  title: string;
  message: string;
  sourceType: "OFFICIAL" | "COMMUNITY" | "AI";
  resourceType: string;
  resourceId: string;
  triggeredAt: string;
  createdAt: string;
};

function validAlert(value: unknown): value is PublicAlert {
  if (!value || typeof value !== "object") return false;
  const alert = value as Record<string, unknown>;
  return ["id", "ruleId", "title", "message", "resourceType", "resourceId", "triggeredAt", "createdAt"].every((key) => typeof alert[key] === "string") &&
    ["INFO", "WATCH", "WARNING", "CRITICAL"].includes(String(alert.severity)) &&
    ["OFFICIAL", "COMMUNITY", "AI"].includes(String(alert.sourceType));
}

export async function listPublicAlerts(signal?: AbortSignal): Promise<PublicAlert[]> {
  const value: unknown = await apiClient.request("/alerts?limit=20&offset=0", { signal }, { authenticate: false, retryUnauthorized: false });
  if (!value || typeof value !== "object" || !Array.isArray((value as { items?: unknown }).items) ||
    !(value as { items: unknown[] }).items.every(validAlert)) throw new Error("Invalid alert API response");
  return (value as { items: PublicAlert[] }).items;
}
