import { Link } from "react-router";
import { useTranslation } from "../../i18n";
import th from "../../i18n/locales/th/reports";
import en from "../../i18n/locales/en/reports";
import { ReportForm } from "./ReportForm";
import "./CreateReportPage.css";

export function CreateReportPage() {
  const { t } = useTranslation({ th, en });
  return <section className="feature-page create-report-page">
    <header className="page-header">
      <Link className="create-report-back" to="/reports"><span aria-hidden="true">←</span> {t("createPage.back")}</Link>
      <p className="eyebrow">{t("eyebrow")}</p>
      <h1>{t("createPage.title")}</h1>
      <p>{t("createPage.description")}</p>
    </header>
    <ReportForm editing={null} onComplete={() => undefined} onCancel={() => undefined} />
  </section>;
}
