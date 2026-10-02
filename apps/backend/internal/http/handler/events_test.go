package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/realtime"
	"github.com/google/uuid"
)

func TestEventStreamUsesCursorFallbackAndRedisIDs(t *testing.T) {
	event := publicEvent(t)
	encoded, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	stream := &eventStreamFake{firstID: "100-0", reads: [][]realtime.Message{{{ID: "101-0", Values: map[string]string{"event": string(encoded)}}}}, readErrors: []error{context.Canceled}}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/events?cursor=100-0", nil)
	response := httptest.NewRecorder()

	newEventStreamHandler(stream, eventStreamOptions{heartbeat: time.Millisecond, replayLimit: 10}).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if stream.afterIDs[0] != "100-0" {
		t.Fatalf("cursor = %q, want 100-0", stream.afterIDs[0])
	}
	body := response.Body.String()
	if !strings.Contains(body, "id: 101-0\n") {
		t.Fatalf("missing Redis SSE id: %q", body)
	}
	if !strings.Contains(body, "event: water_level.updated\n") || !strings.Contains(body, event.EventID) {
		t.Fatalf("missing public envelope: %q", body)
	}
}

func TestEventStreamRejectsInvalidCursor(t *testing.T) {
	stream := &eventStreamFake{}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/events?cursor=not-a-stream-id", nil)
	response := httptest.NewRecorder()

	newEventStreamHandler(stream, eventStreamOptions{}).ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
	if len(stream.afterIDs) != 0 {
		t.Fatalf("stream was read for invalid cursor")
	}
}

func TestEventStreamResetsFutureCursor(t *testing.T) {
	stream := &eventStreamFake{firstID: "100-0", lastID: "120-0"}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/events?cursor=999-0", nil)
	response := httptest.NewRecorder()
	newEventStreamHandler(stream, eventStreamOptions{}).ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "event: replay.reset") || len(stream.afterIDs) != 0 {
		t.Fatalf("future cursor response = %d: %s", response.Code, response.Body)
	}
}

func TestEventStreamBoundsConcurrentGuestConnections(t *testing.T) {
	stream := &eventStreamFake{entered: make(chan struct{}, 1), block: make(chan struct{})}
	handler := newEventStreamHandler(stream, eventStreamOptions{maxClients: 1})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	firstDone := make(chan struct{})
	go func() {
		defer close(firstDone)
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/events/stream", nil).WithContext(ctx))
	}()
	select {
	case <-stream.entered:
	case <-time.After(time.Second):
		t.Fatal("first subscription did not start")
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/events/stream", nil))
	if response.Code != http.StatusServiceUnavailable || response.Header().Get("Retry-After") == "" {
		t.Fatalf("capacity response = %d, Retry-After %q", response.Code, response.Header().Get("Retry-After"))
	}
	cancel()
	select {
	case <-firstDone:
	case <-time.After(time.Second):
		t.Fatal("client disconnect did not release subscription")
	}
	close(stream.block)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/events/stream", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("released slot = %d", response.Code)
	}
}

func TestEventStreamPrefersLastEventIDOverCursorQuery(t *testing.T) {
	stream := &eventStreamFake{firstID: "101-0", readErrors: []error{context.Canceled}}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/events?cursor=101-0", nil)
	request.Header.Set("Last-Event-ID", "102-0")
	response := httptest.NewRecorder()

	newEventStreamHandler(stream, eventStreamOptions{heartbeat: time.Millisecond, replayLimit: 10}).ServeHTTP(response, request)

	if len(stream.afterIDs) != 1 || stream.afterIDs[0] != "102-0" {
		t.Fatalf("cursor = %#v, want Last-Event-ID 102-0", stream.afterIDs)
	}
}

func TestEventStreamDoesNotCheckHistoryForInitialConnection(t *testing.T) {
	stream := &eventStreamFake{firstErr: errors.New("stream unavailable"), readErrors: []error{context.Canceled}}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/events", nil)
	response := httptest.NewRecorder()

	newEventStreamHandler(stream, eventStreamOptions{}).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if stream.firstCalls != 0 {
		t.Fatalf("FirstID calls = %d, want 0", stream.firstCalls)
	}
}

func TestEventStreamSignalsResetForExpiredCursor(t *testing.T) {
	stream := &eventStreamFake{firstID: "100-0"}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/events?cursor=99-0", nil)
	response := httptest.NewRecorder()

	newEventStreamHandler(stream, eventStreamOptions{}).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if !strings.Contains(response.Body.String(), "event: replay.reset\n") || !strings.Contains(response.Body.String(), "cursor_expired") {
		t.Fatalf("reset signal = %q", response.Body.String())
	}
}

func TestEventStreamSignalsResetWhenHistoryIsUnavailable(t *testing.T) {
	stream := &eventStreamFake{}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/events?cursor=99-0", nil)
	response := httptest.NewRecorder()

	newEventStreamHandler(stream, eventStreamOptions{}).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if !strings.Contains(response.Body.String(), "event: replay.reset\n") || !strings.Contains(response.Body.String(), "cursor_expired") {
		t.Fatalf("reset signal = %q", response.Body.String())
	}
}

func TestEventStreamStopsOnClientBackpressure(t *testing.T) {
	event := publicEvent(t)
	encoded, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	stream := &eventStreamFake{reads: [][]realtime.Message{{{ID: "101-0", Values: map[string]string{"event": string(encoded)}}}}}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/events", nil)
	response := &failingEventWriter{header: make(http.Header)}

	newEventStreamHandler(stream, eventStreamOptions{heartbeat: time.Millisecond, replayLimit: 10}).ServeHTTP(response, request)

	if len(stream.afterIDs) != 1 {
		t.Fatalf("stream reads = %d, want 1", len(stream.afterIDs))
	}
}

func TestEventStreamResetsOnlyWhenReplayLimitLeavesBacklog(t *testing.T) {
	event := publicEvent(t)
	encoded, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	stream := &eventStreamFake{firstID: "100-0", reads: [][]realtime.Message{{{ID: "101-0", Values: map[string]string{"event": string(encoded)}}}}, hasAfter: true}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/events?cursor=100-0", nil)
	response := httptest.NewRecorder()

	newEventStreamHandler(stream, eventStreamOptions{replayLimit: 1}).ServeHTTP(response, request)

	if !strings.Contains(response.Body.String(), "event: replay.reset\n") || !strings.Contains(response.Body.String(), "replay_limit") {
		t.Fatalf("reset signal = %q", response.Body.String())
	}
}

func TestEventStreamDoesNotResetWhenReplayLimitReachesEnd(t *testing.T) {
	event := publicEvent(t)
	encoded, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	stream := &eventStreamFake{firstID: "100-0", reads: [][]realtime.Message{{{ID: "101-0", Values: map[string]string{"event": string(encoded)}}}}, readErrors: []error{context.Canceled}}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/events?cursor=100-0", nil)
	response := httptest.NewRecorder()

	newEventStreamHandler(stream, eventStreamOptions{replayLimit: 1}).ServeHTTP(response, request)

	if strings.Contains(response.Body.String(), "event: replay.reset\n") {
		t.Fatalf("unexpected reset signal = %q", response.Body.String())
	}
}

func publicEvent(t *testing.T) realtime.Event {
	t.Helper()
	return realtime.Event{
		EventID:        uuid.NewString(),
		EventType:      realtime.EventWaterLevelUpdated,
		OccurredAt:     time.Date(2026, time.October, 2, 12, 0, 0, 0, time.UTC),
		PublishedAt:    time.Date(2026, time.October, 2, 12, 1, 0, 0, time.UTC),
		SourceType:     realtime.SourceOfficial,
		ResourceType:   realtime.ResourceWaterStation,
		ResourceID:     uuid.NewString(),
		PayloadVersion: realtime.PayloadVersion1,
		Payload:        json.RawMessage(`{"waterLevelM":1.25}`),
	}
}

type eventStreamFake struct {
	firstID    string
	lastID     string
	firstErr   error
	firstCalls int
	reads      [][]realtime.Message
	readErrors []error
	afterIDs   []string
	hasAfter   bool
	entered    chan struct{}
	block      chan struct{}
}

func (stream *eventStreamFake) Append(context.Context, string, int64, map[string]string) (string, error) {
	return "", errors.New("not implemented")
}

func (stream *eventStreamFake) FirstID(context.Context) (string, error) {
	stream.firstCalls++
	return stream.firstID, stream.firstErr
}

func (stream *eventStreamFake) LastID(context.Context) (string, error) {
	if stream.lastID != "" {
		return stream.lastID, nil
	}
	return "9999999999999-0", nil
}

func (stream *eventStreamFake) HasAfter(context.Context, string) (bool, error) {
	return stream.hasAfter, nil
}

func (stream *eventStreamFake) Read(ctx context.Context, afterID string, _ int, _ time.Duration) ([]realtime.Message, error) {
	if stream.block != nil {
		if stream.entered != nil {
			select {
			case stream.entered <- struct{}{}:
			default:
			}
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-stream.block:
			return nil, context.Canceled
		}
	}
	stream.afterIDs = append(stream.afterIDs, afterID)
	if len(stream.reads) > 0 {
		messages := stream.reads[0]
		stream.reads = stream.reads[1:]
		return messages, nil
	}
	if len(stream.readErrors) > 0 {
		err := stream.readErrors[0]
		stream.readErrors = stream.readErrors[1:]
		return nil, err
	}
	return nil, context.Canceled
}

type failingEventWriter struct {
	header     http.Header
	writeCount int
}

func (writer *failingEventWriter) Header() http.Header { return writer.header }
func (writer *failingEventWriter) WriteHeader(int)     {}
func (writer *failingEventWriter) Write([]byte) (int, error) {
	writer.writeCount++
	if writer.writeCount == 1 {
		return 1, nil
	}
	return 0, errors.New("slow client")
}
func (writer *failingEventWriter) Flush() {}
