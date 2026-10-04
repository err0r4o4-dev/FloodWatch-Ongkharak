import { useEffect, useState } from "react";
import { useTranslation } from "../../i18n";
import th from "../../i18n/locales/th/reports";
import en from "../../i18n/locales/en/reports";
import { apiClient } from "../auth/api";
import type { ReportMedia } from "./types";

export function ReportMediaImage({ media }: { media: ReportMedia }) {
  const { t } = useTranslation({ th, en });
  const [source, setSource] = useState<string | null>(null);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    let active = true;
    let objectUrl: string | null = null;
    setSource(null);
    setFailed(false);
    void apiClient.requestBlob(media.url)
      .then((blob) => {
        objectUrl = URL.createObjectURL(blob);
        if (active) setSource(objectUrl);
        else URL.revokeObjectURL(objectUrl);
      })
      .catch(() => {
        if (active) setFailed(true);
      });
    return () => {
      active = false;
      if (objectUrl) URL.revokeObjectURL(objectUrl);
    };
  }, [media.url]);

  if (failed) return <p className="report-media__error" role="status">{t("media.error")}</p>;
  if (!source) return <p className="report-media__loading" role="status">{t("media.loading")}</p>;
  return <a className="report-media__image-link" href={source} target="_blank" rel="noreferrer"><img src={source} alt={t("media.alt")} /></a>;
}
