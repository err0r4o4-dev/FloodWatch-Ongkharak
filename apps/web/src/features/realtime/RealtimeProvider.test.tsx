import { act, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { RealtimeProvider, useRealtimeRevision, useRealtimeStatus } from "./RealtimeProvider";
import { OverviewPage } from "../overview/OverviewPage";
import { MemoryRouter } from "react-router";

vi.mock("../overview/OverviewMapPreview", () => ({ OverviewMapPreview: () => <div aria-label="แผนที่ภาพรวม" /> }));

class FakeEventSource {
  static connections: FakeEventSource[] = [];
  readonly listeners = new Map<string, (event: Event) => void>();
  onopen: (() => void) | null = null;
  onerror: (() => void) | null = null;
  closed = false;

  constructor(readonly url: string) {
    FakeEventSource.connections.push(this);
  }

  addEventListener(type: string, listener: EventListenerOrEventListenerObject) {
    this.listeners.set(type, listener as (event: Event) => void);
  }

  close() { this.closed = true; }

  send(type: string, data: unknown, cursor = "1740000000000-0") {
    this.listeners.get(type)?.(new MessageEvent(type, { data: JSON.stringify(data), lastEventId: cursor }));
  }
}

function Consumer() {
  const status = useRealtimeStatus();
  const revision = useRealtimeRevision(["report.verified"]);
  return <div><span>{status}</span><span>revision {revision}</span></div>;
}

afterEach(() => {
  vi.unstubAllGlobals();
  vi.useRealTimers();
  FakeEventSource.connections = [];
});

describe("public realtime subscription", () => {
  it("connects once, ignores malformed/duplicate events, and refetches by revision", () => {
    vi.stubGlobal("EventSource", FakeEventSource);
    const view = render(<RealtimeProvider><Consumer /></RealtimeProvider>);
    expect(FakeEventSource.connections).toHaveLength(1);
    const connection = FakeEventSource.connections[0];
    act(() => connection.onopen?.());
    expect(screen.getByText("CONNECTED")).toBeInTheDocument();
    expect(screen.getByText("revision 1")).toBeInTheDocument();
    act(() => connection.send("report.verified", { eventType: "report.verified", eventId: "id-1", resourceId: "report-1", payloadVersion: 1, sourceType: "COMMUNITY" }));
    expect(screen.getByText("revision 2")).toBeInTheDocument();
    act(() => connection.send("report.verified", { eventType: "report.verified", eventId: "id-1", resourceId: "report-1", payloadVersion: 1, sourceType: "COMMUNITY" }));
    act(() => connection.send("report.verified", { eventType: "report.verified", eventId: "id-2", resourceId: "report-2", payloadVersion: 2, sourceType: "COMMUNITY" }));
    expect(screen.getByText("revision 2")).toBeInTheDocument();
    view.rerender(<RealtimeProvider><Consumer /></RealtimeProvider>);
    expect(FakeEventSource.connections).toHaveLength(1);
    view.unmount();
    expect(connection.closed).toBe(true);
  });

  it("backs off reconnects, resumes at the cursor, and keeps REST state independent", () => {
    vi.useFakeTimers();
    vi.stubGlobal("EventSource", FakeEventSource);
    const view = render(<RealtimeProvider><Consumer /></RealtimeProvider>);
    const first = FakeEventSource.connections[0];
    act(() => first.onopen?.());
    act(() => first.send("report.verified", { eventType: "report.verified", eventId: "id-1", resourceId: "report-1", payloadVersion: 1, sourceType: "COMMUNITY" }));
    act(() => first.onerror?.());
    expect(screen.getByText("RECONNECTING")).toBeInTheDocument();
    expect(first.closed).toBe(true);
    act(() => vi.advanceTimersByTime(1000));
    expect(FakeEventSource.connections).toHaveLength(2);
    expect(FakeEventSource.connections[1].url).toContain("cursor=1740000000000-0");
    act(() => FakeEventSource.connections[1].onopen?.());
    expect(screen.getByText("revision 3")).toBeInTheDocument();
    act(() => FakeEventSource.connections[1].listeners.get("replay.reset")?.(new Event("replay.reset")));
    expect(screen.getByText("revision 4")).toBeInTheDocument();
    view.unmount();
    act(() => vi.advanceTimersByTime(30_000));
    expect(FakeEventSource.connections).toHaveLength(2);
  });

  it("keeps exponential backoff across transient opens and resets after a stable connection", () => {
    vi.useFakeTimers();
    vi.stubGlobal("EventSource", FakeEventSource);
    const view = render(<RealtimeProvider><Consumer /></RealtimeProvider>);
    const first = FakeEventSource.connections[0];
    act(() => first.onopen?.());
    act(() => first.onerror?.());

    act(() => vi.advanceTimersByTime(1000));
    const second = FakeEventSource.connections[1];
    act(() => second.onopen?.());
    act(() => second.onerror?.());
    act(() => vi.advanceTimersByTime(1999));
    expect(FakeEventSource.connections).toHaveLength(2);
    act(() => vi.advanceTimersByTime(1));

    const third = FakeEventSource.connections[2];
    act(() => third.onopen?.());
    act(() => vi.advanceTimersByTime(15_000));
    act(() => third.onerror?.());
    act(() => vi.advanceTimersByTime(999));
    expect(FakeEventSource.connections).toHaveLength(3);
    act(() => vi.advanceTimersByTime(1));
    expect(FakeEventSource.connections).toHaveLength(4);
    view.unmount();
  });

  it("surfaces disconnected after repeated transient connections", () => {
    vi.useFakeTimers();
    vi.stubGlobal("EventSource", FakeEventSource);
    const view = render(<RealtimeProvider><Consumer /></RealtimeProvider>);

    for (const delay of [1000, 2000, 4000, 8000, 16_000]) {
      const connection = FakeEventSource.connections.at(-1)!;
      act(() => connection.onopen?.());
      act(() => connection.onerror?.());
      act(() => vi.advanceTimersByTime(delay));
    }
    const sixth = FakeEventSource.connections.at(-1)!;
    act(() => sixth.onopen?.());
    act(() => sixth.onerror?.());

    expect(screen.getByText("DISCONNECTED")).toBeInTheDocument();
    view.unmount();
  });

  it("refetches the canonical overview after a relevant public event", async () => {
    vi.stubGlobal("EventSource", FakeEventSource);
    const request = vi.fn().mockResolvedValue({ ok: true, json: async () => ({ items: [], limit: 20, offset: 0 }) });
    vi.stubGlobal("fetch", request);
    const view = render(<MemoryRouter><RealtimeProvider><OverviewPage /></RealtimeProvider></MemoryRouter>);
    expect(await screen.findByText("ยังไม่มีสถานีในรายการที่เรียกดู")).toBeInTheDocument();
    expect(request).toHaveBeenCalledTimes(6);
    act(() => FakeEventSource.connections[0].onopen?.());
    await waitFor(() => expect(request).toHaveBeenCalledTimes(12));
    act(() => FakeEventSource.connections[0].send("water_level.updated", {
      eventType: "water_level.updated", eventId: "event-1", resourceId: "station-1", payloadVersion: 1, sourceType: "OFFICIAL"
    }));
    await waitFor(() => expect(request).toHaveBeenCalledTimes(18));
    act(() => FakeEventSource.connections[0].send("report.verified", {
      eventType: "report.verified", eventId: "report-event-1", resourceId: "report-1", payloadVersion: 1, sourceType: "COMMUNITY"
    }));
    await waitFor(() => expect(request).toHaveBeenCalledTimes(24));
    act(() => FakeEventSource.connections[0].send("alert.created", {
      eventType: "alert.created", eventId: "alert-event-1", resourceId: "alert-1", payloadVersion: 1, sourceType: "OFFICIAL"
    }));
    await waitFor(() => expect(request).toHaveBeenCalledTimes(30));
    expect(screen.queryByText("ปลอดภัย")).not.toBeInTheDocument();
    view.unmount();
  });
});
