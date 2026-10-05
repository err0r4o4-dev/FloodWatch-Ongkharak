import { useCallback, useEffect, useRef, useState } from "react";
import { formatDate, useTranslation } from "../../i18n";
import th from "../../i18n/locales/th/adminAnnouncements";
import en from "../../i18n/locales/en/adminAnnouncements";
import { AnnouncementCard } from "../manual-announcements/AnnouncementCard";
import { manualAnnouncementApi } from "../manual-announcements/api";
import type {
  AdminAnnouncementDetail,
  AnnouncementHistory,
  AnnouncementInput,
  AnnouncementStatus,
  ManualAnnouncement,
  Severity,
} from "../manual-announcements/api";
import { bangkokDateTimeValue, fromBangkokDateTime, publicationIssues } from "../manual-announcements/validation";
import "./AdminAnnouncementsPage.css";

type StatusFilter = AnnouncementStatus | "ALL";
type Action = "publish" | "update" | "withdraw" | null;
type RequestState = "idle" | "loading" | "ready" | "error";
type FormState = {
  title: string;
  summary: string;
  body: string;
  advice: string;
  area: string;
  severity: Severity;
  observedAt: string;
  expiresAt: string;
  references: Array<{ name: string; url: string; observedAt: string }>;
};

const PAGE_SIZE = 20;
const MAX_OFFSET = 10_000;
const REQUEST_TIMEOUT_MS = 10_000;
const statuses: StatusFilter[] = ["ALL", "DRAFT", "PUBLISHED", "EXPIRED", "WITHDRAWN"];
const severityValues: Severity[] = ["INFO", "WATCH", "WARNING", "CRITICAL"];

export function canAdvanceAnnouncementPage(offset: number, total: number) {
  return offset + PAGE_SIZE <= MAX_OFFSET && offset + PAGE_SIZE < total;
}

const emptyForm = (): FormState => ({
  title: "", summary: "", body: "", advice: "", area: "", severity: "INFO",
  observedAt: "", expiresAt: "", references: [],
});

function formFromAnnouncement(announcement: ManualAnnouncement): FormState {
  return {
    title: announcement.title,
    summary: announcement.summary,
    body: announcement.body,
    advice: announcement.advice,
    area: announcement.area,
    severity: announcement.severity,
    observedAt: bangkokDateTimeValue(announcement.observedAt),
    expiresAt: bangkokDateTimeValue(announcement.expiresAt),
    references: announcement.references.map((reference) => ({
      name: reference.name,
      url: reference.url,
      observedAt: bangkokDateTimeValue(reference.observedAt),
    })),
  };
}

function formToInput(form: FormState): AnnouncementInput {
  return {
    title: form.title,
    summary: form.summary,
    body: "",
    advice: form.advice,
    area: "",
    severity: form.severity,
    observedAt: form.observedAt ? fromBangkokDateTime(form.observedAt) : null,
    expiresAt: null,
    references: form.references
      .filter((reference) => reference.name || reference.url || reference.observedAt)
      .map((reference) => ({
        name: reference.name,
        url: reference.url,
        observedAt: reference.observedAt ? fromBangkokDateTime(reference.observedAt) : null,
      })),
  };
}

function formKey(form: FormState) {
  try { return JSON.stringify(formToInput(form)); }
  catch { return null; }
}

function formIsDirty(form: FormState, announcement: ManualAnnouncement) {
  return formKey(form) !== formKey(formFromAnnouncement(announcement));
}

function requestWithTimeout<T>(request: (signal: AbortSignal) => Promise<T>) {
  const controller = new AbortController();
  let timer: number | undefined;
  const timeout = new Promise<never>((_, reject) => {
    timer = window.setTimeout(() => {
      controller.abort();
      reject(new Error("REQUEST_TIMEOUT"));
    }, REQUEST_TIMEOUT_MS);
  });
  const promise = Promise.race([request(controller.signal), timeout]).finally(() => {
    if (timer !== undefined) window.clearTimeout(timer);
  });
  return { promise, cancel: () => { if (timer !== undefined) window.clearTimeout(timer); controller.abort(); } };
}

function errorKind(error: unknown): "conflict" | "forbidden" | "notFound" | "invalidRequest" | "serverError" {
  if (typeof error === "object" && error !== null) {
    const candidate = error as { status?: number; code?: string };
    if (candidate.status === 409 || candidate.code === "CONFLICT") return "conflict";
    if (candidate.status === 401 || candidate.status === 403) return "forbidden";
    if (candidate.status === 404) return "notFound";
    if (candidate.status === 400 || candidate.code === "INVALID_REQUEST") return "invalidRequest";
  }
  return "serverError";
}

function statusLabel(status: StatusFilter, t: (key: keyof typeof th) => string) {
  return ({ ALL: t("statusAll"), DRAFT: t("statusDraft"), PUBLISHED: t("statusPublished"), EXPIRED: t("statusExpired"), WITHDRAWN: t("statusWithdrawn") })[status];
}

function severityLabel(severity: Severity, t: (key: keyof typeof th) => string) {
  return ({ INFO: t("info"), WATCH: t("watch"), WARNING: t("warning"), CRITICAL: t("critical") })[severity];
}

function historyActionLabel(action: AnnouncementHistory["action"], t: (key: keyof typeof th) => string) {
  return ({ CREATED: t("historyCreated"), UPDATED: t("historyUpdated"), PUBLISHED: t("historyPublished"), WITHDRAWN: t("historyWithdrawn") })[action];
}

export function AdminAnnouncementsPage() {
  const { locale, t } = useTranslation({ th, en });
  const [status, setStatus] = useState<StatusFilter>("ALL");
  const [offset, setOffset] = useState(0);
  const [listState, setListState] = useState<{ state: RequestState; items: ManualAnnouncement[]; total: number }>({ state: "loading", items: [], total: 0 });
  const [listRetry, setListRetry] = useState(0);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [detailRetry, setDetailRetry] = useState(0);
  const [detail, setDetail] = useState<AdminAnnouncementDetail | null>(null);
  const [detailState, setDetailState] = useState<RequestState>("idle");
  const [editing, setEditing] = useState(false);
  const [isNew, setIsNew] = useState(false);
  const [form, setForm] = useState<FormState>(emptyForm);
  const [previewVisible, setPreviewVisible] = useState(false);
  const [action, setAction] = useState<Action>(null);
  const [withdrawalReason, setWithdrawalReason] = useState("");
  const [fieldIssues, setFieldIssues] = useState<string[]>([]);
  const [feedback, setFeedback] = useState<{ kind: "success" | "error"; key: keyof typeof th } | null>(null);
  const [pendingAction, setPendingAction] = useState<string | null>(null);
  const pendingRef = useRef(false);

  const loadList = useCallback((nextStatus: StatusFilter, nextOffset: number, signal: AbortSignal) => {
    return manualAnnouncementApi.adminList(nextStatus, nextOffset, signal);
  }, []);

  useEffect(() => {
    let active = true;
    setListState((current) => ({ ...current, state: "loading" }));
    const request = requestWithTimeout((signal) => loadList(status, offset, signal));
    void request.promise.then((result) => {
      if (active) setListState({ state: "ready", items: result.items, total: result.total });
    }).catch(() => {
      if (active) setListState((current) => ({ ...current, state: "error" }));
    });
    return () => { active = false; request.cancel(); };
  }, [loadList, listRetry, offset, status]);

  useEffect(() => {
    if (!selectedId) return;
    let active = true;
    setDetailState("loading");
    setDetail(null);
    const request = requestWithTimeout((signal) => manualAnnouncementApi.adminGet(selectedId, signal));
    void request.promise.then((result) => {
      if (active) {
        setDetail(result);
        setForm(formFromAnnouncement(result.announcement));
        setDetailState("ready");
      }
    }).catch(() => {
      if (active) setDetailState("error");
    });
    return () => { active = false; request.cancel(); };
  }, [detailRetry, selectedId]);

  const currentAnnouncement = detail?.announcement ?? null;
  const canEdit = !currentAnnouncement || currentAnnouncement.status !== "WITHDRAWN";
  const publishedEdit = currentAnnouncement?.status === "PUBLISHED" || currentAnnouncement?.status === "EXPIRED";
  const mutationPending = pendingAction !== null;
  const setFormValue = <Key extends keyof FormState>(key: Key, value: FormState[Key]) => {
    setForm((current) => ({ ...current, [key]: value }));
    setPreviewVisible(false);
    setAction(null);
    setFeedback(null);
  };

  const openNew = () => {
    if (pendingRef.current) return;
    setSelectedId(null);
    setDetail(null);
    setDetailState("ready");
    setIsNew(true);
    setEditing(true);
    setForm(emptyForm());
    setPreviewVisible(false);
    setAction(null);
    setFeedback(null);
    setFieldIssues([]);
  };

  const openExisting = (announcement: ManualAnnouncement) => {
    if (pendingRef.current) return;
    setDetail(null);
    setDetailState("loading");
    setSelectedId(announcement.id);
    setDetailRetry((value) => value + 1);
    setIsNew(false);
    setEditing(true);
    setPreviewVisible(false);
    setAction(null);
    setFeedback(null);
    setFieldIssues([]);
  };

  const closeEditor = () => {
    if (pendingRef.current) return;
    setEditing(false);
    setPreviewVisible(false);
    setAction(null);
    setFeedback(null);
    setFieldIssues([]);
  };

  const reloadDetail = () => {
    if (!selectedId || pendingRef.current) return;
    setPreviewVisible(false);
    setAction(null);
    setFeedback(null);
    setDetail(null);
    setDetailState("loading");
    setDetailRetry((value) => value + 1);
  };

  const finishMutation = (message: keyof typeof th, announcement: ManualAnnouncement) => {
    setDetail({ announcement, history: detail?.history ?? [] });
    setForm(formFromAnnouncement(announcement));
    setEditing(false);
    setPreviewVisible(false);
    setAction(null);
    setWithdrawalReason("");
    setFeedback({ kind: "success", key: message });
    setListRetry((value) => value + 1);
  };

  const runMutation = async (kind: "create" | "update" | "publish" | "withdraw") => {
    if (pendingRef.current) return;
    pendingRef.current = true;
    setPendingAction(kind);
    setFeedback(null);
    try {
      if (kind === "create") {
        const result = await manualAnnouncementApi.create(formToInput(form));
        finishMutation("saved", result.announcement);
      } else if (kind === "update" && currentAnnouncement) {
        const result = await manualAnnouncementApi.update(currentAnnouncement.id, formToInput(form), currentAnnouncement.revision);
        finishMutation("updated", result.announcement);
      } else if (kind === "publish" && currentAnnouncement) {
        if (formIsDirty(form, currentAnnouncement)) {
          setFeedback({ kind: "error", key: "publishSaveFirst" });
          return;
        }
        const result = await manualAnnouncementApi.publish(currentAnnouncement.id, currentAnnouncement.revision);
        finishMutation("published", result.announcement);
      } else if (kind === "withdraw" && currentAnnouncement) {
        const result = await manualAnnouncementApi.withdraw(currentAnnouncement.id, currentAnnouncement.revision, withdrawalReason.trim());
        finishMutation("withdrawn", result.announcement);
      }
    } catch (error) {
      const kindKey = errorKind(error);
      setFeedback({ kind: "error", key: kindKey === "conflict" ? "conflict" : kindKey });
    } finally {
      pendingRef.current = false;
      setPendingAction(null);
    }
  };

  const handleSaveDraft = () => {
    try {
      void runMutation(isNew ? "create" : "update");
    } catch {
      setFeedback({ kind: "error", key: "invalidDate" });
    }
  };

  const requestPublish = () => {
    try {
      if (currentAnnouncement && formIsDirty(form, currentAnnouncement)) {
        setFeedback({ kind: "error", key: "publishSaveFirst" });
        setAction(null);
        return;
      }
      const issues = publicationIssues(formToInput(form));
      setFieldIssues(issues);
      if (issues.length === 0) setAction("publish");
    } catch {
      setFeedback({ kind: "error", key: "invalidDate" });
    }
  };

  const requestUpdate = () => {
    try {
      formToInput(form);
      setPreviewVisible(true);
      void runMutation("update");
    } catch {
      setFeedback({ kind: "error", key: "invalidDate" });
    }
  };

  const requestWithdraw = () => {
    setAction("withdraw");
    setFeedback(null);
  };

  const togglePreview = () => {
    setPreviewVisible((visible) => !visible);
    setAction(null);
    setFeedback(null);
  };

  const confirmAction = () => {
    if (action === "withdraw" && !withdrawalReason.trim()) {
      setFeedback({ kind: "error", key: "requiredForWithdraw" });
      return;
    }
    if (action === "publish") void runMutation("publish");
    if (action === "update") void runMutation("update");
    if (action === "withdraw") void runMutation("withdraw");
  };

  const updateReference = (index: number, key: "name" | "url" | "observedAt", value: string) => {
    setForm((current) => ({
      ...current,
      references: current.references.map((reference, referenceIndex) => referenceIndex === index ? { ...reference, [key]: value } : reference),
    }));
    setPreviewVisible(false);
    setAction(null);
  };

  const addReference = () => {
    if (form.references.length >= 10) return;
    setForm((current) => ({ ...current, references: [...current.references, { name: "", url: "", observedAt: "" }] }));
    setPreviewVisible(false);
    setAction(null);
    setFeedback(null);
  };

  const removeReference = (index: number) => {
    setForm((current) => ({ ...current, references: current.references.filter((_, referenceIndex) => referenceIndex !== index) }));
    setPreviewVisible(false);
    setAction(null);
  };

  const stateMessage = feedback ? t(feedback.key) : null;
  const pageFrom = listState.total === 0 ? 0 : offset + 1;
  const pageTo = Math.min(offset + PAGE_SIZE, offset + listState.items.length);

  return <section className="admin-announcements-page">
    <header className="admin-announcements-page__header">
      <div>
        <p className="eyebrow">{t("title")}</p>
        <h1>{t("title")}</h1>
        <p>{t("description")}</p>
      </div>
      <button type="button" className="action-button" disabled={mutationPending} onClick={openNew}>{t("create")}</button>
    </header>
    <p className="admin-announcements-page__source-note">{t("sourceNote")}</p>
    {feedback?.kind === "success" && <div className="admin-announcements-feedback admin-announcements-feedback--success" role="status">{stateMessage}</div>}

    <section className="admin-announcements-toolbar" aria-labelledby="announcement-status-heading">
      <h2 id="announcement-status-heading">{t("statusFilter")}</h2>
      <div className="admin-announcements-tabs" role="group" aria-labelledby="announcement-status-heading">
        {statuses.map((option) => <button key={option} type="button" disabled={mutationPending} aria-pressed={status === option} onClick={() => { setStatus(option); setOffset(0); }}>
          {statusLabel(option, t)}
        </button>)}
      </div>
    </section>

    {listState.state === "loading" && <div className="admin-announcements-state" role="status">{t("loading")}</div>}
    {listState.state === "error" && <div className="admin-announcements-state admin-announcements-state--error" role="alert"><p>{t("loadError")}</p><button type="button" className="action-button" onClick={() => setListRetry((value) => value + 1)}>{t("retry")}</button></div>}
    {listState.state === "ready" && listState.items.length === 0 && <div className="admin-announcements-state">{t("empty")}</div>}
    {listState.state === "ready" && listState.items.length > 0 && <section className="admin-announcements-list" aria-label={t("title")}>
      <div className="admin-announcements-list__meta">{t("showing", { from: pageFrom, to: pageTo, total: listState.total })}</div>
      <ul>{listState.items.map((announcement) => <li key={announcement.id}>
        <article className={`admin-announcement-row admin-announcement-row--${announcement.status.toLowerCase()}`}>
          <div className="admin-announcement-row__content">
            <span className="admin-announcement-status">{statusLabel(announcement.status, t)}</span>
            <h2>{announcement.title || t("formTitleCreate")}</h2>
            <p>{announcement.summary || "—"}</p>
            <span>{severityLabel(announcement.severity, t)}</span>
          </div>
          <button type="button" className="action-button" disabled={mutationPending} onClick={() => openExisting(announcement)} aria-label={t("edit", { title: announcement.title || t("formTitleCreate") })}>{announcement.status === "WITHDRAWN" ? t("readOnly") : t("edit", { title: announcement.title || t("formTitleCreate") })}</button>
        </article>
      </li>)}</ul>
      <nav className="admin-announcements-pagination" aria-label={t("pageLabel")}>
        <button type="button" className="action-button" disabled={mutationPending || offset === 0} onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}>{t("previous")}</button>
        <button type="button" className="action-button" disabled={mutationPending || !canAdvanceAnnouncementPage(offset, listState.total)} onClick={() => setOffset(offset + PAGE_SIZE)}>{t("next")}</button>
      </nav>
    </section>}

    {(editing || detailState === "loading" || detailState === "error") && <section className="admin-announcements-editor" aria-labelledby="admin-announcement-editor-heading">
      {detailState === "loading" && !isNew && <p role="status">{t("loadingDetail")}</p>}
      {detailState === "error" && !isNew && <div role="alert"><p>{t("loadDetailError")}</p><button type="button" className="action-button" disabled={mutationPending} onClick={reloadDetail}>{t("retry")}</button></div>}
      {editing && (isNew || detailState === "ready") && <AnnouncementEditor
        locale={locale}
        t={t}
        form={form}
        currentAnnouncement={currentAnnouncement}
        publishedEdit={publishedEdit}
        canEdit={canEdit}
        previewVisible={previewVisible}
        action={action}
        withdrawalReason={withdrawalReason}
        fieldIssues={fieldIssues}
        pendingAction={pendingAction}
        feedback={feedback}
        stateMessage={stateMessage}
        onClose={closeEditor}
        onReloadLatest={reloadDetail}
        onChange={setFormValue}
        onReferenceChange={updateReference}
        onAddReference={addReference}
        onRemoveReference={removeReference}
        onPreview={() => { setPreviewVisible(true); setAction(null); }}
        onTogglePreview={togglePreview}
        onRequestUpdate={requestUpdate}
        onSaveDraft={handleSaveDraft}
        onRequestPublish={requestPublish}
        onRequestWithdraw={requestWithdraw}
        onWithdrawalReasonChange={setWithdrawalReason}
        onCancelAction={() => setAction(null)}
        onConfirmAction={confirmAction}
        history={detail?.history ?? []}
      />}
    </section>}
  </section>;
}

type EditorProps = {
  locale: "th" | "en";
  t: (key: keyof typeof th, params?: Record<string, string | number>) => string;
  form: FormState;
  currentAnnouncement: ManualAnnouncement | null;
  publishedEdit: boolean;
  canEdit: boolean;
  previewVisible: boolean;
  action: Action;
  withdrawalReason: string;
  fieldIssues: string[];
  pendingAction: string | null;
  feedback: { kind: "success" | "error"; key: keyof typeof th } | null;
  stateMessage: string | null;
  history: AnnouncementHistory[];
  onClose: () => void;
  onReloadLatest: () => void;
  onTogglePreview: () => void;
  onChange: <Key extends keyof FormState>(key: Key, value: FormState[Key]) => void;
  onReferenceChange: (index: number, key: "name" | "url" | "observedAt", value: string) => void;
  onAddReference: () => void;
  onRemoveReference: (index: number) => void;
  onPreview: () => void;
  onRequestUpdate: () => void;
  onSaveDraft: () => void;
  onRequestPublish: () => void;
  onRequestWithdraw: () => void;
  onWithdrawalReasonChange: (value: string) => void;
  onCancelAction: () => void;
  onConfirmAction: () => void;
};

function AnnouncementEditor({
  locale, t, form, currentAnnouncement, publishedEdit, canEdit, previewVisible, action, withdrawalReason,
  fieldIssues, pendingAction, feedback, stateMessage, history, onClose, onChange, onReferenceChange,
  onAddReference, onRemoveReference, onPreview, onRequestUpdate, onSaveDraft, onRequestPublish,
  onRequestWithdraw, onWithdrawalReasonChange, onCancelAction, onConfirmAction, onReloadLatest, onTogglePreview,
}: EditorProps) {
  const previewAnnouncement = {
    ...(currentAnnouncement ?? {
      id: "unsaved-preview",
      sourceType: "COMMUNITY",
      publisherType: "ADMIN",
      referenceKind: "MEDIA",
      status: "DRAFT",
      effectiveStatus: form.severity,
      createdAt: "",
      updatedAt: "",
      publishedAt: null,
      withdrawnAt: null,
      withdrawalReason: "",
      revision: 1,
    }),
    ...form,
    body: "",
    area: "",
    observedAt: form.observedAt ? `${form.observedAt}:00+07:00` : null,
    expiresAt: null,
    references: form.references.map((reference) => ({ ...reference, observedAt: reference.observedAt ? `${reference.observedAt}:00+07:00` : null })),
  } as ManualAnnouncement;
  const labelForIssue = (issue: string) => ({
    title: t("validationTitle"), summary: t("validationSummary"),
    observedAt: t("validationObservedAt"), references: t("validationReferences"),
  }[issue] ?? t("invalidRequest"));

  return <div className="admin-announcements-editor__inner">
    <div className="admin-announcements-editor__heading">
      <div>
        <h2 id="admin-announcement-editor-heading">{currentAnnouncement ? t("formTitleEdit") : t("formTitleCreate")}</h2>
        <p>{t("formDescription")}</p>
      </div>
      <button type="button" className="action-button" disabled={pendingAction !== null} onClick={onClose}>{t("cancel")}</button>
    </div>
    {feedback && <div className={`admin-announcements-feedback admin-announcements-feedback--${feedback.kind}`} role={feedback.kind === "error" ? "alert" : "status"}>
      <strong>{stateMessage}</strong>
      {feedback.key === "conflict" && <><p>{t("conflictHelp")}</p><button type="button" className="action-button" disabled={pendingAction !== null} onClick={onReloadLatest}>{t("reloadLatest")}</button></>}
    </div>}
    {!canEdit && <p className="admin-announcements-readonly">{t("readOnly")}</p>}
    {fieldIssues.length > 0 && <div className="admin-announcements-feedback admin-announcements-feedback--error" role="alert">
      <ul>{fieldIssues.map((issue) => <li key={issue}>{labelForIssue(issue)}</li>)}</ul>
    </div>}

    <form className="admin-announcement-form" onSubmit={(event) => event.preventDefault()}>
      <fieldset disabled={!canEdit || pendingAction !== null}>
        <legend className="sr-only">{t("formTitleEdit")}</legend>
        <label>{t("titleLabel")}<input aria-label={t("titleLabel")} value={form.title} onChange={(event) => onChange("title", event.target.value)} /></label>
        <label>{t("summaryLabel")}<textarea aria-label={t("summaryLabel")} value={form.summary} onChange={(event) => onChange("summary", event.target.value)} /></label>
        <label>{t("severityLabel")}<select aria-label={t("severityLabel")} value={form.severity} onChange={(event) => onChange("severity", event.target.value as Severity)}>
          {severityValues.map((severity) => <option key={severity} value={severity}>{severityLabel(severity, t)}</option>)}
        </select></label>
        <div className="admin-announcement-form__date-grid">
          <label>{t("observedAtLabel")}<input type="datetime-local" aria-label={t("observedAtLabel")} value={form.observedAt} onChange={(event) => onChange("observedAt", event.target.value)} /><small>{t("bangkokTimeHint")}</small></label>
        </div>
        <fieldset className="admin-announcement-references">
          <legend>{t("referencesLegend")}</legend>
          {form.references.map((reference, index) => <div className="admin-announcement-reference" key={`reference-${index}`}>
            <label>{t("referenceName")}<input aria-label={`${t("referenceName")} ${index + 1}`} value={reference.name} onChange={(event) => onReferenceChange(index, "name", event.target.value)} /></label>
            <label>{t("referenceUrl")}<input type="url" aria-label={`${t("referenceUrl")} ${index + 1}`} value={reference.url} onChange={(event) => onReferenceChange(index, "url", event.target.value)} /></label>
            <label>{t("referenceObservedAt")}<input type="datetime-local" aria-label={`${t("referenceObservedAt")} ${index + 1}`} value={reference.observedAt} onChange={(event) => onReferenceChange(index, "observedAt", event.target.value)} /></label>
            <button type="button" className="action-button action-button--quiet" onClick={() => onRemoveReference(index)}>{t("removeReference")}</button>
          </div>)}
          <button type="button" className="action-button action-button--quiet" disabled={form.references.length >= 10} onClick={onAddReference}>{t("addReference")}</button>
          <small>{t("referenceLimit")}</small>
        </fieldset>
      </fieldset>

      {previewVisible && previewAnnouncement && <section className="admin-announcement-preview" aria-labelledby="admin-announcement-preview-heading">
        <h3 id="admin-announcement-preview-heading">{t("previewTitle")}</h3>
        <AnnouncementCard announcement={previewAnnouncement} preview />
      </section>}

      {canEdit && <div className="admin-announcement-actions">
        {!publishedEdit && <button type="button" className="action-button" disabled={pendingAction !== null} onClick={onSaveDraft}>{t("saveDraft")}</button>}
        {!publishedEdit && currentAnnouncement?.status === "DRAFT" && <button type="button" className="action-button" disabled={pendingAction !== null} onClick={onRequestPublish}>{t("publish")}</button>}
        {!publishedEdit && <button type="button" className="action-button action-button--quiet" disabled={pendingAction !== null} onClick={onTogglePreview}>{previewVisible ? t("hidePreview") : t("preview")}</button>}
        {publishedEdit && <button type="button" className="action-button" disabled={pendingAction !== null} onClick={previewVisible ? onRequestUpdate : onPreview}>{previewVisible ? t("confirmUpdate") : t("preview")}</button>}
        {currentAnnouncement && (currentAnnouncement.status === "PUBLISHED" || currentAnnouncement.status === "EXPIRED") && <button type="button" className="action-button action-button--danger" disabled={pendingAction !== null} onClick={onRequestWithdraw}>{t("withdraw")}</button>}
      </div>}
    </form>

    {action === "publish" && <ConfirmationFieldset legend={t("confirmPublishLegend")} text={t("confirmPublishText")} cancelText={t("cancel")} confirmText={t("confirmPublish")} pending={pendingAction !== null} onCancel={onCancelAction} onConfirm={onConfirmAction} />}
    {action === "withdraw" && <fieldset className="admin-announcement-confirmation" aria-label={t("confirmWithdrawLegend")}>
      <legend>{t("confirmWithdrawLegend")}</legend>
      <p>{t("confirmWithdrawText")}</p>
      <label>{t("withdrawalReasonLabel")}<textarea aria-label={t("withdrawalReasonLabel")} placeholder={t("withdrawalReasonPlaceholder")} value={withdrawalReason} onChange={(event) => onWithdrawalReasonChange(event.target.value)} /></label>
      <div className="admin-announcement-actions"><button type="button" className="action-button action-button--quiet" onClick={onCancelAction}>{t("cancel")}</button><button type="button" className="action-button action-button--danger" disabled={pendingAction !== null} onClick={onConfirmAction}>{t("confirmWithdraw")}</button></div>
    </fieldset>}

    {currentAnnouncement && <History history={history} locale={locale} t={t} />}
  </div>;
}

function ConfirmationFieldset({ legend, text, confirmText, cancelText, pending, onCancel, onConfirm }: { legend: string; text: string; confirmText: string; cancelText: string; pending: boolean; onCancel: () => void; onConfirm: () => void }) {
  return <fieldset className="admin-announcement-confirmation" aria-label={legend}>
    <legend>{legend}</legend><p>{text}</p>
    <div className="admin-announcement-actions"><button type="button" className="action-button action-button--quiet" onClick={onCancel}>{cancelText}</button><button type="button" className="action-button" disabled={pending} onClick={onConfirm}>{confirmText}</button></div>
  </fieldset>;
}

function History({ history, locale, t }: { history: AnnouncementHistory[]; locale: "th" | "en"; t: (key: keyof typeof th, params?: Record<string, string | number>) => string }) {
  return <section className="admin-announcement-history" aria-labelledby="admin-announcement-history-heading">
    <h3 id="admin-announcement-history-heading">{t("historyTitle")}</h3>
    {history.length === 0 ? <p>{t("noReason")}</p> : <ol>{history.map((entry) => <li key={`${entry.revision}-${entry.action}`}>
      <details>
        <summary>{historyActionLabel(entry.action, t)} · {t("historySnapshot", { revision: entry.revision })}</summary>
        <div className="admin-announcement-history__snapshot">
          <strong>{entry.announcement.title || t("formTitleCreate")}</strong>
          <p>{entry.announcement.summary || "—"}</p>
          <p>{severityLabel(entry.announcement.severity, t)}</p>
          <time dateTime={entry.recordedAt}>{formatDate(entry.recordedAt, locale)}</time>
          <p>{entry.reason ? t("historyReason", { reason: entry.reason }) : t("noReason")}</p>
        </div>
      </details>
    </li>)}</ol>}
  </section>;
}
