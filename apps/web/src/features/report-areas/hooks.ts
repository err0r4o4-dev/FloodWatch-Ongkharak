import { useCallback, useEffect, useRef, useState } from "react";
import { fetchReportAreaDetail, fetchReportAreaReports } from "./api";
import type { ReportArea, ReportAreaReportsPage, ReportAreaTimelineWindow } from "./types";

export type ReportAreaDetailState = { state: "loading" | "ready" | "error"; area: ReportArea | null; timeline: ReportAreaTimelineWindow[] };
export type ReportAreaReportsState = { state: "loading" | "ready" | "error"; items: import("../reports/types").CommunityReport[]; hasMore: boolean };

export function useReportAreaDetail(areaId: string | null, revision: number): ReportAreaDetailState {
  const [result, setResult] = useState<ReportAreaDetailState>({ state: "loading", area: null, timeline: [] });
  const [pollRevision, setPollRevision] = useState(0);
  useEffect(() => {
    if (!areaId) return;
    const timer = window.setInterval(() => setPollRevision((current) => current + 1), 60_000);
    return () => window.clearInterval(timer);
  }, [areaId]);
  useEffect(() => {
    if (!areaId) return;
    const controller = new AbortController();
    setResult({ state: "loading", area: null, timeline: [] });
    void fetchReportAreaDetail(areaId, controller.signal).then((detail) => {
      if (!controller.signal.aborted) setResult({ state: "ready", area: detail.area, timeline: detail.timeline });
    }).catch(() => { if (!controller.signal.aborted) setResult({ state: "error", area: null, timeline: [] }); });
    return () => controller.abort();
  }, [areaId, revision, pollRevision]);
  return result;
}

const initialReports: ReportAreaReportsState = { state: "loading", items: [], hasMore: false };

export function useReportAreaReports(areaId: string | null, revision: number) {
  const [result, setResult] = useState<ReportAreaReportsState>(initialReports);
  const [pollRevision, setPollRevision] = useState(0);
  const requestVersion = useRef(0);
  const controllerRef = useRef<AbortController | null>(null);
  const offsetRef = useRef(0);
  const load = useCallback((offset: number, append: boolean) => {
    if (!areaId) return;
    controllerRef.current?.abort();
    const controller = new AbortController();
    controllerRef.current = controller;
    const version = ++requestVersion.current;
    setResult((previous) => ({ state: "loading", items: append ? previous.items : [], hasMore: previous.hasMore }));
    void fetchReportAreaReports(areaId, 20, offset, controller.signal).then((page: ReportAreaReportsPage) => {
      if (controller.signal.aborted || requestVersion.current !== version) return;
      offsetRef.current = page.offset + page.items.length;
      setResult((previous) => ({ state: "ready", items: append ? [...previous.items, ...page.items] : page.items, hasMore: page.hasMore }));
    }).catch(() => {
      if (!controller.signal.aborted && requestVersion.current === version) setResult((previous) => ({ ...previous, state: "error" }));
    });
  }, [areaId]);
  useEffect(() => {
    if (!areaId) return;
    const timer = window.setInterval(() => setPollRevision((current) => current + 1), 60_000);
    return () => window.clearInterval(timer);
  }, [areaId]);
  useEffect(() => {
    offsetRef.current = 0;
    setResult(initialReports);
    load(0, false);
    return () => { requestVersion.current += 1; controllerRef.current?.abort(); };
  }, [load, revision, pollRevision]);
  const loadMore = useCallback(() => load(offsetRef.current, true), [load]);
  return { ...result, loadMore };
}
