import type { ReportAreaTimelineWindow } from "./types";

export type TimelineGap = { from: string; to: string };

export function findTimelineGaps(timeline: ReportAreaTimelineWindow[]): TimelineGap[] {
  const windows = [...timeline].sort((first, second) => new Date(first.windowStart).getTime() - new Date(second.windowStart).getTime());
  const gaps: TimelineGap[] = [];
  for (let index = 1; index < windows.length; index += 1) {
    const previous = windows[index - 1];
    const current = windows[index];
    if (new Date(current.windowStart).getTime() > new Date(previous.windowEnd).getTime()) gaps.push({ from: previous.windowEnd, to: current.windowStart });
  }
  return gaps;
}
