import { apiConfig } from "../../shared/config/api";

export function HealthPage() {
  return (
    <section className="health-panel" aria-labelledby="health-title">
      <p className="eyebrow">PHASE 0 / FOUNDATION</p>
      <h1 id="health-title">สถานะระบบสำหรับการพัฒนา</h1>
      <p className="health-panel__lead">ยังไม่มีการเผยแพร่ข้อมูลสถานการณ์น้ำ</p>
      <p className="health-panel__description">
        พื้นที่นี้เตรียมไว้สำหรับตรวจสอบการเชื่อมต่อและโครงสร้างแอปพลิเคชัน ก่อนเริ่มเผยแพร่ข้อมูลที่ผ่านการตรวจสอบ
      </p>
      <dl className="development-details">
        <div>
          <dt>ปลายทาง API</dt>
          <dd>
            <code>{apiConfig.baseUrl}</code>
          </dd>
        </div>
        <div>
          <dt>สถานะข้อมูลสาธารณะ</dt>
          <dd>ยังไม่พร้อมใช้งาน</dd>
        </div>
      </dl>
    </section>
  );
}
