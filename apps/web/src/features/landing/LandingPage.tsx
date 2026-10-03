import { Link } from "react-router";
import { useOptionalAuth } from "../auth/AuthProvider";
import "./LandingPage.css";

function FeatureIcon({ kind }: { kind: "map" | "water" | "report" }) {
  return <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
    {kind === "map" && <><path d="m3 5 6-2 6 2 6-2v16l-6 2-6-2-6 2z" /><path d="M9 3v16M15 5v16" /></>}
    {kind === "water" && <><path d="M12 3C9 7 5 11 5 15a7 7 0 0 0 14 0c0-4-4-8-7-12Z" /><path d="M9 16a3 3 0 0 0 3 3" /></>}
    {kind === "report" && <><rect x="5" y="3" width="14" height="18" rx="2" /><path d="M9 8h6M9 12h6M9 16h4" /></>}
  </svg>;
}

function MapIllustration() {
  return <figure className="landing-preview">
    <div className="landing-preview__card">
      <div className="landing-preview__heading"><span className="landing-preview__mark"><FeatureIcon kind="map" /></span><div><strong>มองเห็นพื้นที่ เข้าใจสถานการณ์</strong><span>แผนที่ · ระดับน้ำ · รายงานชุมชน</span></div></div>
      <div className="landing-preview__map" aria-hidden="true">
        <svg viewBox="0 0 520 290" fill="none" className="landing-map-art">
          <rect width="520" height="290" rx="20" className="landing-map-art__ground" />
          <g className="landing-map-art__blocks"><rect x="30" y="28" width="95" height="52" rx="10" /><rect x="151" y="18" width="90" height="80" rx="12" /><rect x="37" y="123" width="110" height="60" rx="10" /><rect x="179" y="145" width="87" height="102" rx="12" /><rect x="326" y="26" width="152" height="66" rx="12" /><rect x="361" y="126" width="120" height="64" rx="12" /><rect x="364" y="225" width="91" height="45" rx="10" /><rect x="38" y="225" width="97" height="40" rx="10" /></g>
          <path d="M273-20c-46 70 68 82 27 145s-6 100 16 185" className="landing-map-art__water" />
          <g className="landing-map-art__roads"><path d="M-10 104c96-28 175 36 275-3s159 17 265-3M-10 203c96 21 173-34 257 0s194-14 283 12M150-10c-21 80 25 163 4 310M350-10c-23 78 1 122-10 166s-16 96 13 154" /></g>
          <path d="M-10 104c96-28 175 36 275-3s159 17 265-3" className="landing-map-art__road-line" />
          <g className="landing-map-art__pin" transform="translate(111 62)"><circle cy="38" r="24" className="landing-map-art__halo" /><path d="M0 39S-19 21-19 8a19 19 0 0 1 38 0C19 21 0 39 0 39Z" /><circle cy="8" r="6" className="landing-map-art__pin-center" /></g>
          <g className="landing-map-art__pin" transform="translate(393 146)"><circle cy="38" r="24" className="landing-map-art__halo" /><path d="M0 39S-19 21-19 8a19 19 0 0 1 38 0C19 21 0 39 0 39Z" /><circle cy="8" r="6" className="landing-map-art__pin-center" /></g>
          <g transform="translate(223 181)"><rect width="46" height="46" rx="14" className="landing-map-art__sensor" /><path d="M23 9c-4 6-9 11-9 16a9 9 0 0 0 18 0c0-5-5-10-9-16Z" className="landing-map-art__drop" /></g>
        </svg>
        <span className="landing-preview__map-label">สำรวจพื้นที่องครักษ์</span>
      </div>
      <div className="landing-preview__features"><span><FeatureIcon kind="water" />ระดับน้ำ</span><span><FeatureIcon kind="map" />พื้นที่เสี่ยง</span><span><FeatureIcon kind="report" />รายงานชุมชน</span></div>
      <div className="landing-preview__note"><span aria-hidden="true">↗</span> เชื่อมข้อมูล เพื่อประกอบการตัดสินใจ</div>
    </div>
    <span className="landing-preview__float landing-preview__float--water" aria-hidden="true"><FeatureIcon kind="water" /></span>
    <span className="landing-preview__float landing-preview__float--report" aria-hidden="true"><FeatureIcon kind="report" /></span>
    <figcaption>ภาพประกอบการใช้งาน ไม่ใช่ข้อมูลสถานการณ์น้ำจริง</figcaption>
  </figure>;
}

const steps = [
  { number: "01", kind: "water", title: "ตรวจข้อมูลสถานการณ์น้ำ", description: "ดูภาพรวม ระดับน้ำ และประกาศ พร้อมตรวจแหล่งข้อมูลและเวลาที่สังเกต ก่อนนำข้อมูลไปใช้", to: "/overview", action: "เปิดหน้าภาพรวม" },
  { number: "02", kind: "map", title: "ดูพื้นที่และเส้นทาง", description: "สำรวจแผนที่ พื้นที่เสี่ยง และสถานะถนนที่มีข้อมูล ใช้ประกอบการตัดสินใจ ไม่ใช่การรับรองความปลอดภัย", to: "/map", action: "สำรวจแผนที่" },
  { number: "03", kind: "report", title: "ติดตามประกาศและแจ้งเหตุ", description: "ติดตามประกาศและรายงานจากชุมชน หากพบสถานการณ์ในพื้นที่ สามารถเข้าสู่ระบบเพื่อส่งรายงานได้", to: "/reports", action: "ดูรายงานชุมชน" }
] as const;

export function LandingPage() {
  const { account } = useOptionalAuth();
  return <div className="landing-page">
    <a className="skip-link" href="#main-content">ข้ามไปยังเนื้อหาหลัก</a>
    <header className="landing-header"><div className="landing-container landing-header__inner">
      <Link to="/" className="landing-brand" aria-label="Flood Watch Ongkharak หน้าหลัก"><img src="/flood-watch-mark.svg" alt="" width="42" height="42" /><span><strong>Flood Watch</strong><small>ONGKHARAK</small></span></Link>
      <nav className="landing-nav" aria-label="การนำทางหน้าแรก">
        {account ? <Link className="landing-button landing-button--primary" to={account.role === "ADMIN" ? "/admin" : "/reports"}>{account.role === "ADMIN" ? "จัดการระบบ" : "หน้ารายงาน"}</Link> : <Link className="landing-button landing-button--primary" to="/login?returnTo=%2Foverview">เข้าสู่ระบบ</Link>}
      </nav>
    </div></header>
    <main id="main-content" tabIndex={-1}>
      <section className="landing-hero" aria-labelledby="landing-title"><div className="landing-container landing-hero__inner">
        <div className="landing-hero__copy">
          <p className="landing-eyebrow"><FeatureIcon kind="water" />ติดตามสถานการณ์น้ำ อำเภอองครักษ์</p>
          <h1 id="landing-title">ติดตามสถานการณ์น้ำ <span>ก่อนออกเดินทาง</span></h1>
          <p className="landing-hero__description">เชื่อมข้อมูลระดับน้ำ พื้นที่เสี่ยง และรายงานจากชุมชนไว้ในที่เดียว เพื่อให้คุณเข้าใจสถานการณ์และวางแผนได้อย่างมีข้อมูล</p>
          <div className="landing-hero__actions"><Link to="/overview" className="landing-button landing-button--primary">เข้าสู่เมนู <span aria-hidden="true">→</span></Link><Link to="/map" className="landing-button landing-button--secondary">ดูแผนที่ <FeatureIcon kind="map" /></Link></div>
          <div className="landing-hero__support"><span>ดูข้อมูลสาธารณะได้โดยไม่ต้องเข้าสู่ระบบ</span><a href="#how-it-works">รู้จักระบบ <span aria-hidden="true">↓</span></a></div>
        </div>
        <MapIllustration />
      </div></section>
      <section id="how-it-works" className="landing-how" aria-labelledby="landing-how-title"><div className="landing-container">
        <div className="landing-section-heading"><p className="landing-kicker">เข้าใจสถานการณ์ เริ่มจากข้อมูล</p><h2 id="landing-how-title">เริ่มติดตามสถานการณ์ได้ใน 3 ขั้นตอน</h2><p>จากภาพรวมของพื้นที่ สู่ข้อมูลที่คุณต้องการใช้ในชีวิตประจำวัน</p></div>
        <ol className="landing-steps">{steps.map((step) => <li key={step.number} className="landing-step"><div className="landing-step__top"><span className="landing-step__number" aria-hidden="true">{step.number}</span><span className="landing-step__icon"><FeatureIcon kind={step.kind} /></span></div><h3>{step.title}</h3><p>{step.description}</p><Link to={step.to}>{step.action} <span aria-hidden="true">↗</span></Link></li>)}</ol>
        <aside className="landing-caution" aria-labelledby="landing-caution-title"><span className="landing-caution__icon" aria-hidden="true">i</span><div><h3 id="landing-caution-title">ข้อมูลไม่พร้อมใช้งานไม่ได้หมายถึงพื้นที่ปลอดภัย</h3><p>ตรวจแหล่งข้อมูลและเวลาอัปเดตเสมอ ข้อมูลทางการ รายงานชุมชน และข้อมูลจาก AI เป็นคนละแหล่งที่มา และไม่ใช่การรับรองความปลอดภัยในการเดินทาง</p></div><Link to="/announcements">ดูประกาศ <span aria-hidden="true">→</span></Link></aside>
      </div></section>
    </main>
    <footer className="landing-footer"><div className="landing-container landing-footer__inner"><div><strong>Flood Watch Ongkharak</strong><p>ศูนย์ติดตามสถานการณ์น้ำ อำเภอองครักษ์ จังหวัดนครนายก</p></div><Link className="landing-button landing-button--quiet" to="/overview">เข้าสู่เมนู <span aria-hidden="true">→</span></Link></div></footer>
  </div>;
}
