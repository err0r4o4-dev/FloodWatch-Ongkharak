import { useEffect, useId, useState } from "react";
import { apiConfig } from "../../shared/config/api";
import { googleAuthApi, safeReturnTo } from "./google";
import type { GoogleAuthApi } from "./google";

export function GoogleSignInButton({ returnTo, api = googleAuthApi, navigate = (url: string) => window.location.assign(url) }: { returnTo: string; api?: GoogleAuthApi; navigate?: (url: string) => void }) {
  const [state, setState] = useState<"loading" | "enabled" | "disabled" | "error" | "redirecting">("loading");
  const descriptionID = useId();
  useEffect(() => {
    let active = true;
    void api.available().then((enabled) => { if (active) setState(enabled ? "enabled" : "disabled"); }).catch(() => { if (active) setState("error"); });
    return () => { active = false; };
  }, [api]);
  const description = state === "disabled" ? "Google Login ยังไม่ได้ตั้งค่า" : state === "error" ? "ตรวจสอบ Google Login ไม่สำเร็จ ใช้อีเมลและรหัสผ่านได้ตามปกติ" : state === "loading" ? "กำลังตรวจสอบ Google Login..." : state === "redirecting" ? "กำลังไปยัง Google..." : "เลือกบัญชี Google เพื่อเข้าสู่ระบบ";
  const start = () => {
    setState("redirecting");
    try { navigate(`${apiConfig.baseUrl}/auth/google/start?returnTo=${encodeURIComponent(safeReturnTo(returnTo))}`); } catch { setState("error"); }
  };
  return <div className="flex flex-col gap-2">
    <button type="button" className="auth-google" disabled={state !== "enabled"} aria-describedby={descriptionID} onClick={start}>Continue with Google</button>
    <p id={descriptionID} role="status" className="text-center text-sm text-[var(--color-ink-muted)]">{description}</p>
  </div>;
}
