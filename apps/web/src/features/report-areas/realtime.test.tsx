import { act, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { useReportAreaUpdatedRevision } from "./realtime";

class FakeEventSource {
  static connections: FakeEventSource[] = [];
  listeners = new Map<string, (event: MessageEvent) => void>();
  close = vi.fn();
  constructor(public url: string) { FakeEventSource.connections.push(this); }
  addEventListener(type: string, listener: EventListenerOrEventListenerObject) { this.listeners.set(type, listener as (event: MessageEvent) => void); }
  send(type: string, data: unknown) { this.listeners.get(type)?.({ data: JSON.stringify(data) } as MessageEvent); }
}

function Probe() { return <output>{useReportAreaUpdatedRevision()}</output>; }

afterEach(() => { FakeEventSource.connections = []; vi.unstubAllGlobals(); });

describe("report area realtime invalidation", () => {
  it("only invalidates for a validated community report-area event", () => {
    vi.stubGlobal("EventSource", FakeEventSource);
    render(<Probe />);
    const source = FakeEventSource.connections[0];
    expect(screen.getByText("0")).toBeInTheDocument();
    act(() => source.send("report-area.updated", { eventType: "report-area.updated", eventId: "bad", resourceId: "area-1", payloadVersion: 2, sourceType: "COMMUNITY" }));
    expect(screen.getByText("0")).toBeInTheDocument();
    act(() => source.send("report-area.updated", { eventType: "report-area.updated", eventId: "event-1", resourceId: "area-1", payloadVersion: 1, sourceType: "COMMUNITY" }));
    expect(screen.getByText("1")).toBeInTheDocument();
  });
});
