import type { ReactNode } from "react";
import type { Freshness, SourceType } from "../../features/overview/api";

const sourceLabels: Record<SourceType, string> = {
  OFFICIAL: "แหล่งทางการ", COMMUNITY: "รายงานชุมชน", AI: "หลักฐาน AI"
};
const freshnessLabels: Record<Freshness, string> = {
  FRESH: "ข้อมูลล่าสุดตามเกณฑ์", STALE: "ข้อมูลเก่า", EXPIRED: "ข้อมูลหมดอายุ", UNKNOWN: "ไม่ทราบความใหม่"
};
const statusLabels: Record<string, string> = {
  INFO: "ข้อมูล", WATCH: "เฝ้าระวัง", WARNING: "เตือนภัย", CRITICAL: "วิกฤต",
  ACTIVE: "ยังมีเหตุการณ์", RESOLVED: "คลี่คลายตามรายงาน", LOW: "ความเสี่ยงต่ำ",
  MODERATE: "ความเสี่ยงปานกลาง", HIGH: "ความเสี่ยงสูง", VERY_HIGH: "ความเสี่ยงสูงมาก",
  PASSABLE: "ผ่านได้ตามรายงาน", SMALL_VEHICLE_NOT_RECOMMENDED: "ไม่แนะนำรถเล็ก",
  CLOSED: "ปิดการสัญจร", UNKNOWN: "ไม่ทราบสถานะ"
};

export function SourceBadge({ type }: { type: SourceType }) {
  return <span className={`badge badge--source-${type.toLowerCase()}`}>{sourceLabels[type] ?? "ไม่ทราบแหล่งข้อมูล"}</span>;
}

export function FreshnessBadge({ freshness }: { freshness: Freshness }) {
  return <span className={`badge badge--freshness-${freshness.toLowerCase()}`}>{freshnessLabels[freshness] ?? freshnessLabels.UNKNOWN}</span>;
}

export function StatusBadge({ status }: { status: string }) {
  return <span className={`badge badge--status-${status.toLowerCase().replace(/[^a-z_]/g, "")}`}>{statusLabels[status] ?? statusLabels.UNKNOWN}</span>;
}

export function DataTimestamp({ value, label = "เวลาสังเกต" }: { value: string | null; label?: string }) {
  const date = value ? new Date(value) : null;
  const valid = date && !Number.isNaN(date.getTime());
  return <span className="data-timestamp">{label}: {valid ? <time dateTime={value!}>{new Intl.DateTimeFormat("th-TH", { dateStyle: "medium", timeStyle: "short", timeZone: "Asia/Bangkok" }).format(date)}</time> : "ไม่ระบุ"}</span>;
}

export function EmptyState({ title, children }: { title: string; children: ReactNode }) {
  return <div className="state-panel"><span className="state-panel__symbol" aria-hidden="true">○</span><h2>{title}</h2><p>{children}</p></div>;
}

export function ErrorState({ onRetry }: { onRetry: () => void }) {
  return <div className="state-panel" role="alert"><h2>ไม่สามารถโหลดข้อมูลสถานการณ์ได้</h2><p>โปรดลองอีกครั้ง ข้อมูลที่ไม่พร้อมใช้งานไม่ได้หมายถึงพื้นที่ปลอดภัย</p><button type="button" className="action-button" onClick={onRetry}>ลองอีกครั้ง</button></div>;
}

export function LoadingState() {
  return <div className="state-panel" role="status" aria-label="กำลังโหลดข้อมูล"><h2>กำลังโหลดข้อมูล</h2><p>กำลังตรวจสอบข้อมูลจาก API สาธารณะ</p></div>;
}
