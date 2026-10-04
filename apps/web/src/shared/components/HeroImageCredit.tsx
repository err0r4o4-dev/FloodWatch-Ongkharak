import { useTranslation } from "../../i18n";
import th from "../../i18n/locales/th/common";
import en from "../../i18n/locales/en/common";
const messages = { th, en };
export function HeroImageCredit() {
  const { t } = useTranslation(messages);
  return <p className="page-hero__credit">{t("decorativeLandscapeImageNotALiveView")} <a href="/image-credits.txt" target="_blank" rel="noreferrer">{t("imageCredits")}</a></p>;
}
