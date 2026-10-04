import { useTranslation, LanguageSwitcher } from "../../i18n";
import th from "../../i18n/locales/th/common";
import en from "../../i18n/locales/en/common";
const messages = { th, en };
import { Link } from "react-router";
import { LandingIcon } from "./LandingIcon";
import { LandingMapPreview } from "./LandingMapPreview";
import "./LandingPage.css";

const goals = (t: (key: keyof typeof th) => string) => [
  { icon: "building", title: t("traceableInformation"), description: t("informationFromOfficialAgenciesWithSourcesAnd"), tone: "official" },
  { icon: "people", title: t("easyToAccess"), description: t("clearInformationThatEveryoneInTheCommunity"), tone: "community" },
  { icon: "ai", title: t("aiAssistedEvidence"), description: t("aiEvidenceIsShownSeparatelyFromOfficial"), tone: "ai" },
  { icon: "bell", title: t("prepareWithInformation"), description: t("followAnnouncementsAndReportedEventsToMake"), tone: "alert" }
] as const;

function Brand() {
  const { t } = useTranslation(messages);
  return <Link to="/" className="landing-brand" aria-label={t("floodWatchOngkharakHome")}><img src="/flood-watch-mark.svg" alt="" width="44" height="44" /><span><strong>Flood Watch</strong><small>ONGKHARAK</small></span></Link>;
}

function Waves() {
  return <svg className="landing-waves" viewBox="0 0 1600 340" preserveAspectRatio="none" aria-hidden="true"><path d="M0 40C200 90 260 320 590 260S730 150 940 230s370-30 660-140v250H0Z" /><path d="M0 160c240 35 310 180 600 110s410-50 550 10 260-70 450-130v190H0Z" /><path d="M0 230c280-55 400 160 690 80s450 15 640-60 170-40 270-70v160H0Z" /></svg>;
}

export function LandingPage() {
  const { t } = useTranslation(messages);
  return <div className="landing-page">
    <a className="skip-link" href="#main-content">{t("skipToMainContent")}</a>
    <header className="landing-header"><div className="landing-container landing-header__inner"><Brand /><nav className="landing-nav" aria-label={t("homeNavigation")}><LanguageSwitcher className="landing-locale" /></nav></div></header>
    <main id="main-content" tabIndex={-1}>
      <section className="landing-hero" aria-labelledby="landing-title"><Waves /><div className="landing-container landing-hero__inner">
        <div className="landing-hero__copy"><p className="landing-eyebrow"><span aria-hidden="true" />{t("waterSituationMonitoringCentreOngkharakNakhonNayok")}</p><h1 id="landing-title">{t("followTheWaterSituation")} <span>{t("helpEveryonePrepare")}</span></h1><p className="landing-hero__description">{t("exploreOfficialInformationCommunityReportsAndSeparately")}</p><div className="landing-hero__actions"><Link to="/overview" className="landing-button landing-button--primary"><LandingIcon kind="map" />{t("openOverview")} <span aria-hidden="true">→</span></Link><Link to="/map" className="landing-button landing-button--secondary"><LandingIcon kind="map" />{t("viewMap")}</Link></div></div>
        <LandingMapPreview />
      </div></section>
      <section className="landing-goals" aria-labelledby="landing-goals-title"><div className="landing-container"><div className="landing-section-heading"><p className="landing-eyebrow"><span aria-hidden="true" />{t("ourGoals")}</p><h2 id="landing-goals-title">{t("helpTheCommunityPrepareForChangingWater")}</h2><p>{t("withClearTraceableInformationThatEveryoneCan")}</p></div><ul className="landing-goals__list">{goals(t).map((goal) => <li key={goal.title} className={`landing-goal landing-goal--${goal.tone}`}><span className="landing-goal__icon"><LandingIcon kind={goal.icon} /></span><h3>{goal.title}</h3><p>{goal.description}</p></li>)}</ul></div></section>
    </main>
    <footer className="landing-footer"><div className="landing-container landing-footer__inner"><Brand /><p className="landing-footer__description">{t("waterSituationMonitoringCentreOngkharakNakhonNayok")}<br />{t("informationToSupportDecisionsNotAGuarantee")}</p><div className="landing-footer__links"><nav aria-label={t("footerLinks")}><a href="mailto:title.thirawat.dev@gmail.com">{t("contactUs")}</a></nav><small>{t("floodWatchOngkharakForEveryoneInThe")}</small></div></div></footer>
  </div>;
}
