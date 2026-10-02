import { useEffect, useState } from "react";
import { apiClient } from "../auth/api";
import type { ReportMedia } from "./types";

export function ReportMediaImage({ media }: { media: ReportMedia }) {
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

  if (failed) return <p className="report-media__error" role="status">ไม่สามารถโหลดภาพประกอบได้</p>;
  if (!source) return <p className="report-media__loading" role="status">กำลังโหลดภาพประกอบ</p>;
  return <a className="report-media__image-link" href={source} target="_blank" rel="noreferrer"><img src={source} alt="ภาพประกอบรายงาน" /></a>;
}
