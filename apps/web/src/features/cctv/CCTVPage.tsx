import { EmptyState } from "../../shared/components/DataStates";

export function CCTVPage() {
  return <section className="feature-page"><div className="page-heading"><p className="eyebrow">CAMERA NETWORK</p><h1>กล้อง CCTV</h1><p>ภาพจากกล้องจะปรากฏเมื่อมีแหล่งข้อมูลและสิทธิ์เผยแพร่ที่พร้อมใช้งาน</p></div>
    <EmptyState title="ยังไม่มีกล้องที่เผยแพร่">ยังไม่มี API หรือภาพกล้องสาธารณะในระยะนี้ ไม่มีภาพสดหรือสถานะการทำงานที่สามารถยืนยันได้</EmptyState></section>;
}
