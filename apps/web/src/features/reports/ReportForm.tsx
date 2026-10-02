import { useEffect, useState } from "react";
import { ApiError, reportsApi } from "./api";
import { ReportLocationPicker } from "./ReportLocationPicker";
import type { CommunityReport, ReportInput, ReportLocation, ReportType } from "./types";

type UploadState = { name: string; state: "uploading" | "complete" | "failed" };
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
  const [input, setInput] = useState(() => inputFromReport(editing));
  const [location, setLocation] = useState<ReportLocation | null>(editing?.location ?? null);
  const [waterDepth, setWaterDepth] = useState(editing?.estimatedWaterDepthM?.toString() ?? "");
  const [files, setFiles] = useState<File[]>([]);
  const [locationResetKey, setLocationResetKey] = useState(0);
  const [uploadStates, setUploadStates] = useState<UploadState[]>([]);
  const [retryReportId, setRetryReportId] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    setInput(inputFromReport(editing));
    setLocation(editing?.location ?? null);
    setWaterDepth(editing?.estimatedWaterDepthM?.toString() ?? "");
    setFiles([]);
    setUploadStates([]);
    setRetryReportId(null);
    setError(null);
    setLocationResetKey((key) => key + 1);
  }, [editing]);

  const submit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const description = input.description.trim();
    const depth = waterDepth === "" ? undefined : Number(waterDepth);
    if (!retryReportId) {
      if (!description) return setError("กรอกรายละเอียดรายงาน");
      if (Array.from(description).length > maximumDescriptionRunes) return setError("รายละเอียดต้องไม่เกิน 2,000 ตัวอักษร");
      if (!location) return setError("เลือกตำแหน่งบนแผนที่ หรือกรอกพิกัดที่ถูกต้อง");
      if (depth !== undefined && (!Number.isFinite(depth) || depth < 0 || depth > maximumDepthM)) return setError("ระดับน้ำต้องเป็นตัวเลขตั้งแต่ 0 ถึง 100 เมตร");
    }

    setError(null);
    setSubmitting(true);
    try {
      let reportId = retryReportId;
      if (!reportId && location) {
        const payload: ReportInput = { type: input.type, description, location, ...(depth === undefined ? {} : { estimatedWaterDepthM: depth }) };
        const report = editing ? await reportsApi.update(editing.id, payload) : await reportsApi.create(payload);
        reportId = report.id;
      }
      if (!reportId) throw new Error("Missing report ID");
      setUploadStates(files.map((file) => ({ name: file.name, state: "uploading" })));
      const failedFiles: File[] = [];
      for (const [index, file] of files.entries()) {
        try {
          await reportsApi.uploadMedia(reportId, file);
          setUploadStates((states) => states.map((state, position) => position === index ? { ...state, state: "complete" } : state));
        } catch {
          failedFiles.push(file);
          setUploadStates((states) => states.map((state, position) => position === index ? { ...state, state: "failed" } : state));
        }
      }
      if (failedFiles.length > 0) {
        setRetryReportId(reportId);
        setFiles(failedFiles);
        setError("บันทึกรายงานแล้ว แต่บางไฟล์อัปโหลดไม่สำเร็จ ลองอัปโหลดเฉพาะไฟล์ที่เหลืออีกครั้ง");
        onComplete(true);
      } else {
        setRetryReportId(null);
        setFiles([]);
        if (!editing) {
          setInput(inputFromReport(null));
          setLocation(null);
          setWaterDepth("");
          setLocationResetKey((key) => key + 1);
        }
        onComplete();
      }
    } catch (cause) {
      setError(cause instanceof ApiError ? cause.message : "ไม่สามารถบันทึกรายงานได้");
    } finally {
      setSubmitting(false);
    }
  };

  return <section className="data-section report-form-section" aria-labelledby="report-form-heading">
    <div className="data-card__heading"><div><h2 id="report-form-heading">{editing ? "แก้ไขรายงานที่รอตรวจสอบ" : "ส่งรายงานชุมชน"}</h2><p className="section-caption">รายงานจะยังไม่เผยแพร่จนกว่าจะผ่านการตรวจสอบ</p></div>{editing && !retryReportId && <button type="button" className="action-button" onClick={onCancel}>ยกเลิกการแก้ไข</button>}</div>
    <form className="report-form" onSubmit={submit} noValidate>
      {error && <p className="form-error" role="alert">{error}</p>}
      {retryReportId ? <p>รายงานถูกบันทึกแล้ว การลองอีกครั้งจะไม่สร้างรายงานใหม่</p> : <>
      <label>ประเภทเหตุการณ์<select value={input.type} onChange={(event) => setInput((current) => ({ ...current, type: event.target.value as ReportType }))}>
        <option value="FLOOD">น้ำท่วม</option><option value="ROAD">สภาพถนน</option><option value="OTHER">อื่น ๆ</option>
      </select></label>
      <label>รายละเอียด<textarea value={input.description} onChange={(event) => setInput((current) => ({ ...current, description: event.target.value }))} maxLength={maximumDescriptionRunes} /></label>
      <label>ระดับน้ำโดยประมาณ (เมตร, ไม่บังคับ)<input type="number" inputMode="decimal" min="0" max={maximumDepthM} step="0.01" value={waterDepth} onChange={(event) => setWaterDepth(event.target.value)} /></label>
      <ReportLocationPicker location={location} onChange={setLocation} resetKey={locationResetKey} />
      <label>ภาพประกอบ (JPEG, PNG หรือ WebP; ไม่บังคับ)<input key={locationResetKey} type="file" accept="image/jpeg,image/png,image/webp" multiple onChange={(event) => {
        const selected = Array.from(event.target.files ?? []);
        if (selected.length > maximumFiles) {
          setFiles([]);
          setError("เลือกภาพได้ไม่เกิน 3 ไฟล์");
          return;
        }
        if (selected.some((file) => !supportedMediaTypes.has(file.type) || file.size > maximumFileBytes)) {
          setFiles([]);
          setError("ภาพต้องเป็น JPEG, PNG หรือ WebP และมีขนาดไม่เกิน 5 MiB ต่อไฟล์");
          return;
        }
        setFiles(selected);
        setError(null);
      }} /></label></>}
      {uploadStates.length > 0 && <ul className="upload-status" aria-live="polite">{uploadStates.map((upload, index) => <li key={index}>{upload.name}: {upload.state === "uploading" ? "กำลังอัปโหลด" : upload.state === "complete" ? "อัปโหลดแล้ว" : "อัปโหลดไม่สำเร็จ"}</li>)}</ul>}
      <button type="submit" className="action-button" disabled={submitting}>{submitting ? "กำลังบันทึก..." : retryReportId ? "อัปโหลดไฟล์ที่เหลืออีกครั้ง" : editing ? "บันทึกการแก้ไข" : "ส่งรายงาน"}</button>
      {retryReportId && <button type="button" className="action-button" onClick={() => {
        setRetryReportId(null);
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
      }}>ข้ามไฟล์ที่ไม่สำเร็จ</button>}
    </form>
  </section>;
}
