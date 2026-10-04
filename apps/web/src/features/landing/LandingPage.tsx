import { Link } from "react-router";
import { useOptionalAuth } from "../auth/AuthProvider";
import { LandingIcon } from "./LandingIcon";
import { LandingMapPreview } from "./LandingMapPreview";
import "./LandingPage.css";

const goals = [
  { icon: "building", title: "เชื่อถือได้", description: "รวบรวมข้อมูลจากหน่วยงานทางการ พร้อมแสดงแหล่งที่มาและเวลาสังเกต", tone: "official" },
  { icon: "people", title: "เข้าถึงง่าย", description: "ให้ข้อมูลที่เข้าใจง่าย เข้าถึงได้สำหรับทุกคนในชุมชน", tone: "community" },
  { icon: "ai", title: "ใช้เทคโนโลยี AI", description: "ใช้ AI เป็นหลักฐานประกอบการประเมิน แยกจากข้อมูลทางการและรายงานชุมชน", tone: "ai" },
  { icon: "bell", title: "ลดความเสี่ยง", description: "ติดตามประกาศและเหตุการณ์สำคัญ เพื่อช่วยเตรียมรับมืออย่างมีข้อมูล", tone: "alert" }
] as const;

function Brand() {
  return <Link to="/" className="landing-brand" aria-label="Flood Watch Ongkharak หน้าหลัก"><img src="/flood-watch-mark.svg" alt="" width="44" height="44" /><span><strong>Flood Watch</strong><small>ONGKHARAK</small></span></Link>;
}

function Waves() {
  return <svg className="landing-waves" viewBox="0 0 1600 340" preserveAspectRatio="none" aria-hidden="true"><path d="M0 40C200 90 260 320 590 260S730 150 940 230s370-30 660-140v250H0Z" /><path d="M0 160c240 35 310 180 600 110s410-50 550 10 260-70 450-130v190H0Z" /><path d="M0 230c280-55 400 160 690 80s450 15 640-60 170-40 270-70v160H0Z" /></svg>;
}

export function LandingPage() {
  const { account } = useOptionalAuth();
  const entry = account ? "/overview" : "/login?returnTo=%2Foverview";
  const entryLabel = "เข้าสู่เมนู";
  return <div className="landing-page">
    <a className="skip-link" href="#main-content">ข้ามไปยังเนื้อหาหลัก</a>
    <header className="landing-header"><div className="landing-container landing-header__inner"><Brand /><nav className="landing-nav" aria-label="การนำทางหน้าแรก"><div className="landing-locale" aria-label="ภาษา"><span aria-current="true">TH</span><button type="button" disabled aria-describedby="landing-english-note">EN</button><span id="landing-english-note" className="sr-only">ภาษาอังกฤษยังไม่พร้อมใช้งาน</span></div><Link className="landing-button landing-button--primary landing-button--header" to={entry}>เข้าสู่ระบบ</Link></nav></div></header>
    <main id="main-content" tabIndex={-1}>
      <section className="landing-hero" aria-labelledby="landing-title"><Waves /><div className="landing-container landing-hero__inner">
        <div className="landing-hero__copy"><p className="landing-eyebrow"><span aria-hidden="true" />ศูนย์ติดตามสถานการณ์น้ำ อำเภอองครักษ์ จังหวัดนครนายก</p><h1 id="landing-title">ติดตามสถานการณ์น้ำ <span>เพื่อความปลอดภัยของทุกคน</span></h1><p className="landing-hero__description">รวบรวมข้อมูลจากหน่วยงานทางการและรายงานชุมชน พร้อมแยกหลักฐานจาก AI เพื่อให้คุณเข้าถึงข้อมูลสถานการณ์น้ำได้อย่างเข้าใจง่าย สำหรับอำเภอองครักษ์ จังหวัดนครนายก</p><div className="landing-hero__actions"><Link to="/overview" className="landing-button landing-button--primary"><LandingIcon kind="map" />{entryLabel} <span aria-hidden="true">→</span></Link><Link to="/map" className="landing-button landing-button--secondary"><LandingIcon kind="map" />ดูแผนที่</Link></div>{!account && <Link className="landing-guest-link" to="/overview">ดูภาพรวมโดยไม่เข้าสู่ระบบ</Link>}</div>
        <LandingMapPreview />
      </div></section>
      <section className="landing-goals" aria-labelledby="landing-goals-title"><div className="landing-container"><div className="landing-section-heading"><p className="landing-eyebrow"><span aria-hidden="true" />เป้าหมายของเรา</p><h2 id="landing-goals-title">สร้างชุมชนที่ปลอดภัย พร้อมรับมือสถานการณ์น้ำ</h2><p>ด้วยข้อมูลที่มีแหล่งที่มา เข้าใจง่าย และเข้าถึงได้สำหรับทุกคน</p></div><ul className="landing-goals__list">{goals.map((goal) => <li key={goal.title} className={`landing-goal landing-goal--${goal.tone}`}><span className="landing-goal__icon"><LandingIcon kind={goal.icon} /></span><h3>{goal.title}</h3><p>{goal.description}</p></li>)}</ul></div></section>
    </main>
    <footer className="landing-footer"><div className="landing-container landing-footer__inner"><Brand /><p className="landing-footer__description">ศูนย์ติดตามสถานการณ์น้ำ อำเภอองครักษ์ จังหวัดนครนายก<br />ข้อมูลเพื่อประกอบการตัดสินใจ ไม่ใช่การรับรองความปลอดภัย</p><div className="landing-footer__links"><nav aria-label="ลิงก์ท้ายหน้า"><Link to="/overview">ภาพรวม</Link><Link to="/announcements">ประกาศ</Link><Link to="/reports">รายงานชุมชน</Link></nav><small>Flood Watch Ongkharak · เพื่อทุกคนในชุมชน</small></div></div></footer>
  </div>;
}
