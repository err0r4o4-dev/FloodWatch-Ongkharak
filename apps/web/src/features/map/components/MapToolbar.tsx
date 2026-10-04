import type { MapMode } from "../types";
import { useTranslation } from "../../../i18n";
import th from "../../../i18n/locales/th/map";
import en from "../../../i18n/locales/en/map";

export function MapToolbar({ mode, onModeChange }: { mode: MapMode; onModeChange: (mode: MapMode) => void }) {
  const { t } = useTranslation({ th, en });
  return <div className="map-toolbar" role="group" aria-label={t("mode.label")}>
    <button type="button" aria-pressed={mode === "SITUATION"} onClick={() => onModeChange("SITUATION")}>{t("mode.situation")}</button>
    <button type="button" aria-pressed={mode === "ROUTE"} onClick={() => onModeChange("ROUTE")}>{t("mode.route")}</button>
  </div>;
}
