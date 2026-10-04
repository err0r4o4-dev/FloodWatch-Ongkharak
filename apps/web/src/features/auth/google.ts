import { apiConfig } from "../../shared/config/api";
import { ApiError } from "./api";
import { authMessage, translateAuthMessage } from "./messages";
import type { AuthMessage } from "./messages";

export type GoogleAuthApi = {
  available(): Promise<boolean>;
  link(password: string): Promise<{ returnTo: string }>;
};

export function safeReturnTo(value: string | null): string {
  if (!value || value.length > 2048 || !value.startsWith("/") || value.startsWith("//")) return "/reports";
  try {
    const decoded = decodeURIComponent(value);
    if (/[\\\u0000-\u001f\u007f-\u009f]/u.test(decoded)) return "/reports";
    const path = value.split(/[?#]/u)[0];
    const decodedPath = decodeURIComponent(path);
    if (decodedPath.includes("%") || decodedPath !== path || decodedPath.startsWith("//") || path.includes("//") || path.split("/").some((part) => part === "." || part === "..")) return "/reports";
    if (["/login", "/register", "/forgot-password", "/auth", "/api"].some((blocked) => path === blocked || path.startsWith(`${blocked}/`))) return "/reports";
    return value;
  } catch {
    return "/reports";
  }
}

async function googleRequest(path: string, init: RequestInit = {}): Promise<Record<string, unknown>> {
  const response = await fetch(`${apiConfig.baseUrl}/auth/google/${path}`, { ...init, credentials: "same-origin", redirect: "error" });
  const body: unknown = await response.json();
  if (!body || typeof body !== "object" || Array.isArray(body)) throw new Error("Invalid Google login response");
  const payload = body as Record<string, unknown>;
  if (!response.ok) {
    const error = payload.error;
    const code = error && typeof error === "object" && "code" in error && typeof error.code === "string" ? error.code : undefined;
    throw new ApiError("Google login could not be completed", response.status, code);
  }
  return payload;
}

export const googleAuthApi: GoogleAuthApi = {
  async available() {
    const body = await googleRequest("config");
    if (typeof body.enabled !== "boolean") throw new Error("Invalid Google login configuration");
    return body.enabled;
  },
  async link(password) {
    const body = await googleRequest("link", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ password }) });
    if (typeof body.returnTo !== "string") throw new Error("Invalid Google login destination");
    return { returnTo: safeReturnTo(body.returnTo) };
  }
};

export function googleErrorKey(code: string | null): AuthMessage | null {
  if (!code) return null;
  switch (code) {
    case "GOOGLE_CANCELED": return authMessage("googleCanceled");
    case "GOOGLE_STATE_INVALID": return authMessage("googleStateInvalid");
    case "GOOGLE_PROVIDER_UNAVAILABLE": return authMessage("googleProviderUnavailable");
    case "GOOGLE_LINK_REQUIRED": return authMessage("googleLinkRequired");
    case "GOOGLE_LINK_LIMIT": return authMessage("googleLinkLimit");
    case "GOOGLE_IDENTITY_CONFLICT": return authMessage("googleIdentityConflict");
    case "GOOGLE_ACCOUNT_UNAVAILABLE": return authMessage("googleAccountUnavailable");
    default: return authMessage("googleGenericError");
  }
}

export function googleErrorMessage(code: string | null): string | null {
  const message = googleErrorKey(code);
  return message ? translateAuthMessage(message) : null;
}
