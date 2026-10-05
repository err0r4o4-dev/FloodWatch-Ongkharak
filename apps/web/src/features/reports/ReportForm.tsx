import { useEffect, useRef, useState } from "react";
import { useTranslation } from "../../i18n";
import th from "../../i18n/locales/th/reports";
import en from "../../i18n/locales/en/reports";
import { notifyError, notifySuccess, notifyWarning } from "../../shared/alerts";
import { ApiError, reportsApi } from "./api";
import { ReportLocationPicker } from "./ReportLocationPicker";
import { useGuestIdentity } from "../guest/identity";
import { roadSegmentsApi } from "../road-segments/api";
import type { RoadSegmentCandidate } from "../road-segments/types";
import type { CommunityReport, ReportInput, ReportLocation, ReportType, WaterTrend } from "./types";

type UploadState = { name: string; state: "uploading" | "complete" | "failed" };
type FormError = { key: keyof typeof th };
const maximumDescriptionRunes = 2_000;
const maximumDepthM = 100;
const maximumFiles = 3;
const maximumFileBytes = 5 * 1024 * 1024;
const supportedMediaTypes = new Set(["image/jpeg", "image/png", "image/webp"]);
const depthPresets = ["", "0", "0.1", "0.3", "0.5", "1"];

type ObservationChoice = "now" | "custom" | "unknown";
type CandidateState = "idle" | "loading" | "ready" | "error";

function observationChoice(report: CommunityReport | null): ObservationChoice {
  return report ? report.observedAt ? "custom" : "unknown" : "now";
}

function localObservationTime(value?: string | null): string {
  if (!value) return "";
  const date = new Date(value);
  if (!Number.isFinite(date.getTime())) return "";
  date.setMinutes(date.getMinutes() - date.getTimezoneOffset());
  return date.toISOString().slice(0, 16);
}

function depthChoiceFromReport(report: CommunityReport | null) {
  const value = report?.estimatedWaterDepthM?.toString() ?? "";
  return depthPresets.includes(value) ? value : "custom";
}

function inputFromReport(report: CommunityReport | null): ReportInput {
  return report ? {
    type: report.type,
    description: report.description,
    locationName: report.locationName ?? "",
    locationNote: report.locationNote ?? "",
    location: report.location,
    ...(report.estimatedWaterDepthM === null ? {} : { estimatedWaterDepthM: report.estimatedWaterDepthM })
  } : { type: "FLOOD", description: "", locationName: "", locationNote: "", location: { type: "Point", coordinates: [0, 0] } };
}

export function ReportForm({ editing, initialLocation = null, initialLocationName = "", initialRoadSegmentId = "", onComplete, onCancel }: {
  editing: CommunityReport | null;
  initialLocation?: ReportLocation | null;
  initialLocationName?: string;
  initialRoadSegmentId?: string;
  onComplete: (preserveEditing?: boolean) => void;
  onCancel: () => void;
}) {
  const { t } = useTranslation({ th, en });
  const { displayName, setDisplayName } = useGuestIdentity();
  const [nameDraft, setNameDraft] = useState(displayName);
  const submitPending = useRef(false);
  const [submitted, setSubmitted] = useState(false);
  const [input, setInput] = useState(() => ({ ...inputFromReport(editing), ...(!editing && initialLocationName ? { locationName: initialLocationName } : {}) }));
  const [location, setLocation] = useState<ReportLocation | null>(editing?.location ?? initialLocation);
  const [roadSegmentId, setRoadSegmentId] = useState(editing?.roadSegmentId ?? "");
  const [candidates, setCandidates] = useState<RoadSegmentCandidate[]>([]);
  const [candidateState, setCandidateState] = useState<CandidateState>("idle");
  const initialSegmentId = useRef<string | null>(editing?.roadSegmentId ?? null);
  const [observedChoice, setObservedChoice] = useState<ObservationChoice>(() => observationChoice(editing));
  const [observedTime, setObservedTime] = useState(() => localObservationTime(editing?.observedAt));
  const [waterTrend, setWaterTrend] = useState<WaterTrend>(editing?.waterTrend ?? "UNKNOWN");
  const [waterDepth, setWaterDepth] = useState(editing?.estimatedWaterDepthM?.toString() ?? "");
  const [depthChoice, setDepthChoice] = useState(() => depthChoiceFromReport(editing));
  const [customDepthCm, setCustomDepthCm] = useState(editing?.estimatedWaterDepthM === null || !editing ? "" : (editing.estimatedWaterDepthM * 100).toString());
  const [files, setFiles] = useState<File[]>([]);
  const [locationResetKey, setLocationResetKey] = useState(0);
  const [uploadStates, setUploadStates] = useState<UploadState[]>([]);
  const [retryReportId, setRetryReportId] = useState<string | null>(null);
  const [uploadToken, setUploadToken] = useState<string | undefined>();
  const [error, setError] = useState<FormError | null>(null);
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => { setNameDraft(displayName); }, [displayName]);

  useEffect(() => {
    setInput({ ...inputFromReport(editing), ...(!editing && initialLocationName ? { locationName: initialLocationName } : {}) });
    setLocation(editing?.location ?? initialLocation);
    setRoadSegmentId(editing?.roadSegmentId ?? "");
    initialSegmentId.current = editing?.roadSegmentId ?? null;
    setCandidates([]);
    setCandidateState("idle");
    setObservedChoice(observationChoice(editing));
    setObservedTime(localObservationTime(editing?.observedAt));
    setWaterTrend(editing?.waterTrend ?? "UNKNOWN");
    setWaterDepth(editing?.estimatedWaterDepthM?.toString() ?? "");
    setDepthChoice(depthChoiceFromReport(editing));
    setCustomDepthCm(editing?.estimatedWaterDepthM === null || !editing ? "" : (editing.estimatedWaterDepthM * 100).toString());
    setFiles([]);
    setUploadStates([]);
    setRetryReportId(null);
    setUploadToken(undefined);
    setError(null);
    setLocationResetKey((key) => key + 1);
  }, [editing, initialLocation, initialLocationName]);

  useEffect(() => {
    if (!location) {
      setCandidates([]);
      setCandidateState("idle");
      return;
    }
    const controller = new AbortController();
    setCandidates([]);
    setCandidateState("loading");
    void roadSegmentsApi.candidates(location.coordinates[0], location.coordinates[1], controller.signal)
      .then((result) => {
        if (controller.signal.aborted) return;
        setCandidates(result.items);
        setCandidateState("ready");
        if (initialSegmentId.current && result.items.some((candidate) => candidate.id === initialSegmentId.current)) return;
      })
      .catch(() => {
        if (!controller.signal.aborted) setCandidateState("error");
      });
    return () => controller.abort();
  }, [location?.coordinates[0], location?.coordinates[1]]);

  const selectLocation = (next: ReportLocation | null) => {
    const changed = location?.coordinates[0] !== next?.coordinates[0] || location?.coordinates[1] !== next?.coordinates[1];
    if (changed) {
      setRoadSegmentId("");
      initialSegmentId.current = null;
    }
    setLocation(next);
  };

  const submit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (submitPending.current) return;
    const submittedName = nameDraft.trim() || "Guest";
    const description = input.description.trim();
    const locationName = input.locationName?.trim() ?? "";
    const locationNote = input.locationNote?.trim() ?? "";
    const depth = waterDepth === "" ? undefined : Number(waterDepth);
    const observedDate = observedChoice === "now" ? new Date() : observedChoice === "custom" ? new Date(observedTime) : null;
    if (!retryReportId) {
      if (Array.from(submittedName).length > 80) return setError({ key: "form.error.displayName" });
      if (!description) return setError({ key: "form.error.required" });
      if (Array.from(description).length > maximumDescriptionRunes) return setError({ key: "form.error.descriptionLength" });
      if (!location) return setError({ key: "form.error.location" });
      if ((!editing && !locationName) || Array.from(locationName).length > 200) return setError({ key: "form.error.locationName" });
      if (Array.from(locationNote).length > 500) return setError({ key: "form.error.locationNote" });
      if (depthChoice === "custom" && !customDepthCm.trim()) return setError({ key: "form.error.depth" });
      if (depth !== undefined && (!Number.isFinite(depth) || depth < 0 || depth > maximumDepthM)) return setError({ key: "form.error.depth" });
      if (observedDate && (!Number.isFinite(observedDate.getTime()) || observedDate.getFullYear() < 2000 || observedDate.getTime() > Date.now() + 30_000)) return setError({ key: "form.error.observedAt" });
    }

    setError(null);
    submitPending.current = true;
    setSubmitting(true);
    try {
      let reportId = retryReportId;
      let capability = uploadToken;
      if (!reportId && location) {
        const payload: ReportInput = { type: input.type, description, location, locationName, locationNote, ...(editing?.roadSegmentId === roadSegmentId && editing.location.coordinates[0] === location.coordinates[0] && editing.location.coordinates[1] === location.coordinates[1] ? {} : { roadSegmentId }), waterTrend, ...(observedDate ? { observedAt: editing?.observedAt && observedTime === localObservationTime(editing.observedAt) && observedChoice === "custom" ? editing.observedAt : observedDate.toISOString() } : {}), ...(!editing ? { displayName: submittedName } : {}), ...(depth === undefined ? {} : { estimatedWaterDepthM: depth }) };
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
          setRoadSegmentId("");
          initialSegmentId.current = null;
          setWaterDepth("");
          setDepthChoice("");
          setCustomDepthCm("");
          setObservedChoice("now");
          setObservedTime("");
          setWaterTrend("UNKNOWN");
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
      {retryReportId ? <p>{t("form.retryNotice")}</p> : <fieldset className="report-form-fields" disabled={submitting}>
      {!editing && <div className="report-field"><label>{t("form.displayName")}<input value={nameDraft} maxLength={160} aria-describedby="report-name-hint" onChange={(event) => setNameDraft(event.target.value)} /></label><p id="report-name-hint" className="report-field-hint">{t("form.displayNameHint")}</p></div>}
      <label>{t("form.type")}<select value={input.type} onChange={(event) => setInput((current) => ({ ...current, type: event.target.value as ReportType }))}>
        <option value="FLOOD">{t("type.flood")}</option><option value="ROAD">{t("type.road")}</option><option value="OTHER">{t("type.other")}</option>
      </select></label>
      <ReportLocationPicker location={location} onChange={selectLocation} resetKey={locationResetKey} />
      <fieldset className="report-segment-picker" aria-describedby="report-segment-hint">
        <legend>{t("form.roadSegment")}</legend>
        <p id="report-segment-hint" className="report-field-hint">{t("form.roadSegmentHint")}</p>
        {candidateState === "loading" && <p role="status">{t("form.roadSegmentLoading")}</p>}
        {candidateState === "error" && <p role="alert" className="form-error">{t("form.roadSegmentError")}</p>}
        {candidateState === "ready" && candidates.length === 0 && <p className="report-field-hint">{t("form.roadSegmentNone")}</p>}
        <label className="report-segment-option"><input type="radio" name="report-road-segment" value="" checked={roadSegmentId === ""} onChange={() => { setRoadSegmentId(""); initialSegmentId.current = null; }} /><span>{t("form.roadSegmentUnassigned")}</span></label>
        {candidates.map((candidate) => <label className="report-segment-option" key={candidate.id}><input type="radio" name="report-road-segment" value={candidate.id} checked={roadSegmentId === candidate.id} onChange={() => { setRoadSegmentId(candidate.id); initialSegmentId.current = null; }} /><span><strong>{candidate.name}</strong> · {candidate.startLabel}–{candidate.endLabel}<small>{t("form.roadSegmentDistance", { meters: Math.round(candidate.distanceM) })}</small></span></label>)}
        {roadSegmentId && !candidates.some((candidate) => candidate.id === roadSegmentId) && <div className="report-segment-retired" role="status"><p>{t("form.roadSegmentNotListed")}</p><button type="button" className="action-button" onClick={() => { setRoadSegmentId(""); initialSegmentId.current = null; }}>{t("form.roadSegmentClear")}</button></div>}
        {initialRoadSegmentId && !roadSegmentId && <p className="report-field-hint">{t("form.roadSegmentSuggested")}</p>}
      </fieldset>
      <div className="report-place-fields">
        <label>{t("form.locationName")}<input required={!editing} value={input.locationName ?? ""} maxLength={400} placeholder={t("form.locationNamePlaceholder")} aria-describedby="report-place-hint" onChange={(event) => setInput((current) => ({ ...current, locationName: event.target.value }))} /></label>
        <label>{t("form.locationNote")}<input value={input.locationNote ?? ""} maxLength={1000} placeholder={t("form.locationNotePlaceholder")} onChange={(event) => setInput((current) => ({ ...current, locationNote: event.target.value }))} /></label>
        <p id="report-place-hint" className="report-field-hint">{t("form.locationHint")}</p>
      </div>
      <div className="report-place-fields">
        <div className="report-field"><label htmlFor="report-observed-choice">{t("form.observedAt")}</label><select id="report-observed-choice" value={observedChoice} aria-describedby="report-observation-hint" onChange={(event) => setObservedChoice(event.target.value as ObservationChoice)}>
          <option value="now">{t("form.observedNow")}</option>
          <option value="custom">{t("form.observedCustom")}</option>
          {!editing?.observedAt && <option value="unknown">{t("form.observedUnknown")}</option>}
        </select></div>
        {observedChoice === "custom" && <label>{t("form.observedCustomLabel")}<input type="datetime-local" required value={observedTime} onChange={(event) => setObservedTime(event.target.value)} /></label>}
        <p id="report-observation-hint" className="report-field-hint">{t("form.observedHint")}</p>
        <div className="report-field"><label htmlFor="report-water-trend">{t("form.waterTrend")}</label><select id="report-water-trend" value={waterTrend} aria-describedby="report-trend-hint" onChange={(event) => setWaterTrend(event.target.value as WaterTrend)}>
          {["UNKNOWN", "RISING", "STABLE", "FALLING"].map((trend) => <option key={trend} value={trend}>{t(`trend.${trend as WaterTrend}`)}</option>)}
        </select></div>
        <p id="report-trend-hint" className="report-field-hint">{t("form.waterTrendHint")}</p>
      </div>
      <fieldset className="report-depth-picker" aria-describedby="report-depth-hint">
        <legend>{t("form.depth")}</legend>
        <p id="report-depth-hint" className="report-field-hint">{t("form.depthHint")}</p>
        <div className="report-depth-options">
          {[...depthPresets, "custom"].map((value) => <label className="report-depth-option" key={value}>
            <input type="radio" name="report-water-depth" value={value} checked={depthChoice === value} onChange={() => {
              setDepthChoice(value);
              setWaterDepth(value === "custom" ? customDepthCm.trim() === "" ? "" : (Number(customDepthCm) / 100).toString() : value);
            }} />
            <span><span className="report-depth-check" aria-hidden="true">✓</span>{value === "" ? t("form.depthUnspecified") : value === "0" ? t("form.depthNotObserved") : value === "custom" ? t("form.depthCustom") : value === "1" ? t("form.depthOneMeter") : t("form.depthCentimeters", { value: Number(value) * 100 })}</span>
          </label>)}
        </div>
        {depthChoice === "custom" && <label>{t("form.depthCustomLabel")}<input type="number" inputMode="decimal" min="0" max={maximumDepthM * 100} step="any" value={customDepthCm} onChange={(event) => {
          setCustomDepthCm(event.target.value);
          setWaterDepth(event.target.value.trim() === "" ? "" : (Number(event.target.value) / 100).toString());
        }} /></label>}
      </fieldset>
      <label>{t("form.descriptionLabel")}<textarea required value={input.description} onChange={(event) => setInput((current) => ({ ...current, description: event.target.value }))} maxLength={maximumDescriptionRunes} /></label>
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
      }} /></label></fieldset>}
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
          setRoadSegmentId("");
          initialSegmentId.current = null;
          setWaterDepth("");
          setDepthChoice("");
          setCustomDepthCm("");
          setObservedChoice("now");
          setObservedTime("");
          setWaterTrend("UNKNOWN");
          setLocationResetKey((key) => key + 1);
        }
      }}>{t("form.skipFailed")}</button>}
    </form>
  </section>;
}
