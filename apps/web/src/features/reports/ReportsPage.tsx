import { EmptyState } from "../../shared/components/DataStates";

export function ReportsPage() {
  return <section className="feature-page"><div className="page-heading"><p className="eyebrow">COMMUNITY EVIDENCE</p><h1>รายงานสถานการณ์</h1><p>รายงานจากชุมชนต้องมีแหล่งที่มา เวลา และสถานะการตรวจสอบก่อนเผยแพร่</p></div>
    <EmptyState title="ยังไม่มีรายงานที่เผยแพร่">ยังไม่มี API รายงานสาธารณะหรือระบบรับรายงานในระยะนี้ โปรดอย่าส่งข้อมูลส่วนบุคคลผ่านช่องทางที่ไม่ได้จัดเตรียมไว้</EmptyState></section>;
}
