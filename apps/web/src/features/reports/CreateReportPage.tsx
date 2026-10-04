import { useMemo } from "react";
import { Link, useSearchParams } from "react-router";
import { useTranslation } from "../../i18n";
import th from "../../i18n/locales/th/reports";
import en from "../../i18n/locales/en/reports";
import { ReportForm } from "./ReportForm";
import type { ReportLocation } from "./types";
import "./CreateReportPage.css";

export function CreateReportPage() {
  const { t } = useTranslation({ th, en });
  const [searchParams] = useSearchParams();
  const prefill = useMemo(() => {
    const longitudeText = searchParams.get("lng");
    const latitudeText = searchParams.get("lat");
    if (!longitudeText?.trim() || !latitudeText?.trim()) return null;
    const longitude = Number(longitudeText);
    const latitude = Number(latitudeText);
    if (!Number.isFinite(longitude) || !Number.isFinite(latitude) || Math.abs(longitude) > 180 || Math.abs(latitude) > 90) return null;
    const location: ReportLocation = { type: "Point", coordinates: [longitude, latitude] };
    const name = Array.from(searchParams.get("locationName")?.trim() ?? "").slice(0, 200).join("");
    return { location, name };
  }, [searchParams]);
  return <section className="feature-page create-report-page">
    <header className="page-header">
      <Link className="create-report-back" to="/reports"><span aria-hidden="true">←</span> {t("createPage.back")}</Link>
      <p className="eyebrow">{t("eyebrow")}</p>
      <h1>{t("createPage.title")}</h1>
      <p>{t("createPage.description")}</p>
    </header>
    {prefill && <p className="report-field-hint">{t("createPage.areaUpdateHint")}</p>}
    <ReportForm editing={null} initialLocation={prefill?.location} initialLocationName={prefill?.name} onComplete={() => undefined} onCancel={() => undefined} />
  </section>;
}
