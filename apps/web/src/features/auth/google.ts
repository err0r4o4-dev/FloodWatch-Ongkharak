import { apiConfig } from "../../shared/config/api";
import { ApiError } from "./api";

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

export function googleErrorMessage(code: string | null): string | null {
  if (!code) return null;
  switch (code) {
    case "GOOGLE_CANCELED": return "คุณยกเลิกการเข้าสู่ระบบด้วย Google สามารถลองใหม่ได้";
    case "GOOGLE_STATE_INVALID": return "คำขอ Google Login หมดอายุหรือไม่ถูกต้อง กรุณาเริ่มใหม่";
    case "GOOGLE_PROVIDER_UNAVAILABLE": return "เชื่อมต่อ Google ไม่สำเร็จ กรุณาลองใหม่ภายหลัง";
    case "GOOGLE_LINK_REQUIRED": return "รหัสผ่านไม่ถูกต้อง หรือคำขอหมดอายุ กรุณาลองใหม่หรือเริ่มเข้าสู่ระบบอีกครั้ง";
    case "GOOGLE_LINK_LIMIT": return "ยืนยันรหัสผ่านครบจำนวนครั้งแล้ว กรุณาเริ่มเข้าสู่ระบบด้วย Google ใหม่";
    case "GOOGLE_IDENTITY_CONFLICT": return "ไม่สามารถเชื่อม Google กับบัญชีนี้ได้ กรุณาใช้ช่องทางเข้าสู่ระบบเดิม";
    case "GOOGLE_ACCOUNT_UNAVAILABLE": return "บัญชีนี้ไม่พร้อมใช้งาน กรุณาติดต่อผู้ดูแลระบบ";
    default: return "เข้าสู่ระบบด้วย Google ไม่สำเร็จ กรุณาลองใหม่หรือใช้อีเมลและรหัสผ่าน";
  }
}
