import { EmptyState } from "../../shared/components/DataStates";

export function AnnouncementsPage() {
  return <section className="feature-page"><div className="page-heading"><p className="eyebrow">PUBLIC INFORMATION</p><h1>ประกาศ</h1><p>พื้นที่สำหรับประกาศที่เผยแพร่และตรวจสอบแหล่งข้อมูลได้</p></div>
    <EmptyState title="ยังไม่มีประกาศที่เผยแพร่">ระบบยังไม่มี API ประกาศสาธารณะ โปรดติดตามช่องทางทางการเมื่อมีเหตุฉุกเฉิน และอย่าตีความว่าการไม่มีประกาศหมายถึงปลอดภัย</EmptyState></section>;
}
