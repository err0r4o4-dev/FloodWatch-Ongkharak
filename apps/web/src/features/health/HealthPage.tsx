import { useTranslation } from "../../i18n";
import th from "../../i18n/locales/th/common";
import en from "../../i18n/locales/en/common";
const messages = { th, en };
import { apiConfig } from "../../shared/config/api";

export function HealthPage() {
  const { t } = useTranslation(messages);
  return (
    <section className="health-panel" aria-labelledby="health-title">
      <p className="eyebrow">PHASE 0 / FOUNDATION</p>
      <h1 id="health-title">{t("developmentSystemStatus")}</h1>
      <p className="health-panel__lead">{t("thisDevelopmentPageDoesNotPublishWater")}</p>
      <p className="health-panel__description">
        {t("thisPageIsForCheckingTheApplication")}
      </p>
      <dl className="development-details">
        <div>
          <dt>{t("serviceAddress")}</dt>
          <dd>
            <code>{apiConfig.baseUrl}</code>
          </dd>
        </div>
        <div>
          <dt>{t("publicDataStatus")}</dt>
          <dd>{t("notAvailable")}</dd>
        </div>
      </dl>
    </section>
  );
}
