import { useEffect, useRef, useState } from "react";
import { useTranslation } from "../../i18n";
import th from "../../i18n/locales/th/reports";
import en from "../../i18n/locales/en/reports";
import { notifyError, notifySuccess, notifyWarning } from "../../shared/alerts";
import { ApiError, reportsApi } from "./api";
import { ReportLocationPicker } from "./ReportLocationPicker";
import { useGuestIdentity } from "../guest/identity";
import type { CommunityReport, ReportInput, ReportLocation, ReportType } from "./types";

type UploadState = { name: string; state: "uploading" | "complete" | "failed" };
type FormError = { key: keyof typeof th };
const maximumDescriptionRunes = 2_000;
const maximumDepthM = 100;
const maximumFiles = 3;
const maximumFileBytes = 5 * 1024 * 1024;
const supportedMediaTypes = new Set(["image/jpeg", "image/png", "image/webp"]);

function inputFromReport(report: CommunityReport | null): ReportInput {
  return report ? {
    type: report.type,
    description: report.description,
    location: report.location,
    ...(report.estimatedWaterDepthM === null ? {} : { estimatedWaterDepthM: report.estimatedWaterDepthM })
  } : { type: "FLOOD", description: "", location: { type: "Point", coordinates: [0, 0] } };
}

export function ReportForm({ editing, onComplete, onCancel }: {
  editing: CommunityReport | null;
  onComplete: (preserveEditing?: boolean) => void;
  onCancel: () => void;
}) {
  const { t } = useTranslation({ th, en });
  const { displayName, setDisplayName } = useGuestIdentity();
  const [nameDraft, setNameDraft] = useState(displayName);
  const submitPending = useRef(false);
  const [submitted, setSubmitted] = useState(false);
  const [input, setInput] = useState(() => inputFromReport(editing));
  const [location, setLocation] = useState<ReportLocation | null>(editing?.location ?? null);
  const [waterDepth, setWaterDepth] = useState(editing?.estimatedWaterDepthM?.toString() ?? "");
  const [files, setFiles] = useState<File[]>([]);
  const [locationResetKey, setLocationResetKey] = useState(0);
  const [uploadStates, setUploadStates] = useState<UploadState[]>([]);
  const [retryReportId, setRetryReportId] = useState<string | null>(null);
  const [uploadToken, setUploadToken] = useState<string | undefined>();
  const [error, setError] = useState<FormError | null>(null);
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => { setNameDraft(displayName); }, [displayName]);

  useEffect(() => {
    setInput(inputFromReport(editing));
    setLocation(editing?.location ?? null);
    setWaterDepth(editing?.estimatedWaterDepthM?.toString() ?? "");
    setFiles([]);
    setUploadStates([]);
    setRetryReportId(null);
    setUploadToken(undefined);
    setError(null);
    setLocationResetKey((key) => key + 1);
  }, [editing]);

  const submit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (submitPending.current) return;
    const submittedName = nameDraft.trim() || "Guest";
    const description = input.description.trim();
    const depth = waterDepth === "" ? undefined : Number(waterDepth);
    if (!retryReportId) {
      if (Array.from(submittedName).length > 80) return setError({ key: "form.error.displayName" });
      if (!description) return setError({ key: "form.error.required" });
      if (Array.from(description).length > maximumDescriptionRunes) return setError({ key: "form.error.descriptionLength" });
      if (!location) return setError({ key: "form.error.location" });
      if (depth !== undefined && (!Number.isFinite(depth) || depth < 0 || depth > maximumDepthM)) return setError({ key: "form.error.depth" });
    }

    setError(null);
    submitPending.current = true;
    setSubmitting(true);
    try {
      let reportId = retryReportId;
      let capability = uploadToken;
      if (!reportId && location) {
        const payload: ReportInput = { type: input.type, description, location, ...(!editing ? { displayName: submittedName } : {}), ...(depth === undefined ? {} : { estimatedWaterDepthM: depth }) };
        const report = editing ? await reportsApi.update(editing.id, payload) : await reportsApi.create(payload);
        reportId = report.id;
        if (!editing) {
          setDisplayName(submittedName);
          setSubmitted(true);
        }
        if ("uploadToken" in report && typeof report.uploadToken === "string") {
          capability = report.uploadToken;
          setUploadToken(capability);
        }
      }
      if (!reportId) throw new Error("Missing report ID");
      setUploadStates(files.map((file) => ({ name: file.name, state: "uploading" })));
      const failedFiles: File[] = [];
      for (const [index, file] of files.entries()) {
        try {
          if (capability) await reportsApi.uploadMedia(reportId, file, capability);
          else await reportsApi.uploadMedia(reportId, file);
          setUploadStates((states) => states.map((state, position) => position === index ? { ...state, state: "complete" } : state));
        } catch {
          failedFiles.push(file);
          setUploadStates((states) => states.map((state, position) => position === index ? { ...state, state: "failed" } : state));
        }
      }
      if (failedFiles.length > 0) {
        setRetryReportId(reportId);
        setFiles(failedFiles);
        setError({ key: "form.error.uploadPartial" });
        void notifyWarning(t("form.warning.partialTitle"), t("form.warning.partialText"));
        onComplete(true);
      } else {
        setRetryReportId(null);
        setUploadToken(undefined);
        setFiles([]);
        if (!editing) {
          setInput(inputFromReport(null));
          setLocation(null);
          setWaterDepth("");
          setLocationResetKey((key) => key + 1);
        }
        void notifySuccess(editing ? t("form.success.updated") : t("form.success.created"), t("form.success.text"));
        onComplete();
      }
    } catch (cause) {
      const message = cause instanceof ApiError && cause.status === 409 ? { key: "review.conflict" as const } : { key: "form.error.save" as const };
      setError(message);
      void notifyError(t("form.error.saveTitle"), t(message.key));
    } finally {
      submitPending.current = false;
      setSubmitting(false);
    }
  };

  return <section className="data-section report-form-section" aria-labelledby="report-form-heading">
    <div className="data-card__heading"><div><h2 id="report-form-heading">{editing ? t("form.editTitle") : t("form.createTitle")}</h2><p className="section-caption">{t("form.description")}</p></div>{editing && !retryReportId && <button type="button" className="action-button" onClick={onCancel}>{t("form.cancelEdit")}</button>}</div>
    <form className="report-form" onSubmit={submit} noValidate>
      {submitted && <p className="form-success" role="status">{t("form.success.text")}</p>}
      {error && <p className="form-error" role="alert">{t(error.key)}</p>}
      {retryReportId ? <p>{t("form.retryNotice")}</p> : <>
      {!editing && <label>{t("form.displayName")}<input value={nameDraft} maxLength={160} onChange={(event) => setNameDraft(event.target.value)} /></label>}
      <label>{t("form.type")}<select value={input.type} onChange={(event) => setInput((current) => ({ ...current, type: event.target.value as ReportType }))}>
        <option value="FLOOD">{t("type.flood")}</option><option value="ROAD">{t("type.road")}</option><option value="OTHER">{t("type.other")}</option>
      </select></label>
      <label>{t("form.descriptionLabel")}<textarea value={input.description} onChange={(event) => setInput((current) => ({ ...current, description: event.target.value }))} maxLength={maximumDescriptionRunes} /></label>
      <label>{t("form.depth")}<input type="number" inputMode="decimal" min="0" max={maximumDepthM} step="0.01" value={waterDepth} onChange={(event) => setWaterDepth(event.target.value)} /></label>
      <ReportLocationPicker location={location} onChange={setLocation} resetKey={locationResetKey} />
      <label>{t("form.media")}<input key={locationResetKey} type="file" accept="image/jpeg,image/png,image/webp" multiple onChange={(event) => {
        const selected = Array.from(event.target.files ?? []);
        if (selected.length > maximumFiles) {
          setFiles([]);
          setError({ key: "form.error.tooManyFiles" });
          return;
        }
        if (selected.some((file) => !supportedMediaTypes.has(file.type) || file.size > maximumFileBytes)) {
          setFiles([]);
          setError({ key: "form.error.fileType" });
          return;
        }
        setFiles(selected);
        setError(null);
      }} /></label></>}
      {uploadStates.length > 0 && <ul className="upload-status" aria-live="polite">{uploadStates.map((upload, index) => <li key={index}>{upload.name}: {upload.state === "uploading" ? t("form.uploading") : upload.state === "complete" ? t("form.uploaded") : t("form.uploadFailed")}</li>)}</ul>}
      <button type="submit" className="action-button" disabled={submitting}>{submitting ? t("form.saving") : retryReportId ? t("form.retryUpload") : editing ? t("form.saveEdit") : t("form.submit")}</button>
      {retryReportId && <button type="button" className="action-button" onClick={() => {
        setRetryReportId(null);
        setUploadToken(undefined);
        setFiles([]);
        setUploadStates([]);
        setError(null);
        if (editing) onCancel();
        else {
          setInput(inputFromReport(null));
          setLocation(null);
          setWaterDepth("");
          setLocationResetKey((key) => key + 1);
        }
      }}>{t("form.skipFailed")}</button>}
    </form>
  </section>;
}
