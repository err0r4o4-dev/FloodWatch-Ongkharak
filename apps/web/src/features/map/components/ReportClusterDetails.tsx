import { ReportCard } from "../../../features/reports/ReportCard";
import type { CommunityReportFeature } from "../types";
import { useTranslation } from "../../../i18n";
import th from "../../../i18n/locales/th/map";
import en from "../../../i18n/locales/en/map";

function reportDisplayTime(report: CommunityReportFeature["reports"][number]): number {
  const observedAt = report.observedAt ? Date.parse(report.observedAt) : NaN;
  if (Number.isFinite(observedAt)) return observedAt;
  const createdAt = Date.parse(report.createdAt);
  return Number.isFinite(createdAt) ? createdAt : 0;
}

export function ReportClusterSummary({ cluster }: { cluster: CommunityReportFeature }) {
  const { t } = useTranslation({ th, en });
  return <div className="report-cluster-summary">
    <p className="report-cluster-summary__count">{t("cluster.count", { count: cluster.reports.length })}</p>
    <p className="report-cluster-summary__note">{t("cluster.note")}</p>
  </div>;
}

export function ReportClusterDetails({ cluster }: { cluster: CommunityReportFeature }) {
  const { t } = useTranslation({ th, en });
  const reports = [...cluster.reports].sort((first, second) => reportDisplayTime(second) - reportDisplayTime(first) || second.id.localeCompare(first.id));
  return <div className="report-cluster-details">
    <ReportClusterSummary cluster={cluster} />
    <section className="report-cluster-details__section" aria-labelledby="report-cluster-originals-title">
      <h4 id="report-cluster-originals-title">{t("cluster.originals")}</h4>
      <ul className="report-cluster-details__originals">
        {reports.map((report) => <li key={report.id}><ReportCard report={report} /></li>)}
      </ul>
    </section>
  </div>;
}
