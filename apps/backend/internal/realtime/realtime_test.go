package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestPublisherPublishesValidatedPublicEnvelope(t *testing.T) {
	stream := &fakeStream{appendID: "1728000000000-0"}
	publisher := newPublisher(stream)
	now := time.Date(2026, time.October, 2, 12, 0, 0, 0, time.UTC)
	event := Event{
		EventID:        uuid.NewString(),
		EventType:      EventWaterLevelUpdated,
		OccurredAt:     now.Add(-time.Minute),
		PublishedAt:    now,
		SourceType:     SourceOfficial,
		ResourceType:   ResourceWaterStation,
		ResourceID:     uuid.NewString(),
		PayloadVersion: PayloadVersion1,
		Payload:        json.RawMessage(`{"waterLevelM":1.25}`),
	}

	if err := publisher.Publish(context.Background(), event); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if stream.key != PublicStreamKey {
		t.Fatalf("key = %q, want %q", stream.key, PublicStreamKey)
	}
	if stream.maxLen != PublicStreamMaxLen {
		t.Fatalf("maxLen = %d, want %d", stream.maxLen, PublicStreamMaxLen)
	}
	var got Event
	if err := json.Unmarshal([]byte(stream.values["event"]), &got); err != nil {
		t.Fatalf("decode stored event: %v", err)
	}
	if !reflect.DeepEqual(got, event) {
		t.Fatalf("stored event = %#v, want %#v", got, event)
	}
}

func TestPublisherRejectsPrivateOrOversizedEvents(t *testing.T) {
	publisher := newPublisher(&fakeStream{})
	valid := Event{
		EventID:        uuid.NewString(),
		EventType:      EventReportVerified,
		OccurredAt:     time.Now().UTC(),
		PublishedAt:    time.Now().UTC(),
		SourceType:     SourceCommunity,
		ResourceType:   ResourceCommunityReport,
		ResourceID:     uuid.NewString(),
		PayloadVersion: PayloadVersion1,
		Payload:        json.RawMessage(`{"status":"VERIFIED"}`),
	}

	private := valid
	private.EventType = "report.created"
	if err := publisher.Publish(context.Background(), private); !errors.Is(err, ErrInvalidEvent) {
		t.Fatalf("private event error = %v, want ErrInvalidEvent", err)
	}
	oversized := valid
	oversized.Payload = json.RawMessage(`{"value":"` + strings.Repeat("a", MaxPayloadBytes) + `"}`)
	if err := publisher.Publish(context.Background(), oversized); !errors.Is(err, ErrInvalidEvent) {
		t.Fatalf("oversized payload error = %v, want ErrInvalidEvent", err)
	}
	wrongResource := valid
	wrongResource.ResourceType = ResourceRoad
	if err := publisher.Publish(context.Background(), wrongResource); !errors.Is(err, ErrInvalidEvent) {
		t.Fatalf("wrong resource error = %v, want ErrInvalidEvent", err)
	}
	wrongSource := valid
	wrongSource.SourceType = SourceOfficial
	if err := publisher.Publish(context.Background(), wrongSource); !errors.Is(err, ErrInvalidEvent) {
		t.Fatalf("wrong report source error = %v, want ErrInvalidEvent", err)
	}
}

func TestReportUpdatedIsCommunityOnlyCanonicalRefetchSignal(t *testing.T) {
	event := Event{EventID: uuid.NewString(), EventType: "report.updated", OccurredAt: time.Now().UTC(), PublishedAt: time.Now().UTC(), SourceType: SourceCommunity, ResourceType: ResourceCommunityReport, ResourceID: uuid.NewString(), PayloadVersion: PayloadVersion1, Payload: json.RawMessage(`{}`)}
	if err := event.Validate(); err != nil {
		t.Fatalf("report update rejected: %v", err)
	}
	event.SourceType = SourceOfficial
	if err := event.Validate(); !errors.Is(err, ErrInvalidEvent) {
		t.Fatalf("official report update = %v", err)
	}
}

func TestReportAreaUpdatedIsAnIdentifierOnlyCommunitySignal(t *testing.T) {
	event := Event{EventID: uuid.NewString(), EventType: "report-area.updated", OccurredAt: time.Now().UTC(), PublishedAt: time.Now().UTC(), SourceType: SourceCommunity, ResourceType: "REPORT_AREA", ResourceID: uuid.NewString(), PayloadVersion: PayloadVersion1, Payload: json.RawMessage(`{}`)}
	if err := event.Validate(); err != nil { t.Fatalf("area update rejected: %v", err) }
	event.Payload = json.RawMessage(`{"userId":"private"}`)
	if !errors.Is(event.Validate(), ErrInvalidEvent) { t.Fatal("area event accepted non-identifier payload") }
	event.Payload = json.RawMessage(`{}`)
	event.SourceType = SourceOfficial
	if !errors.Is(event.Validate(), ErrInvalidEvent) { t.Fatal("area update accepted official provenance") }
}

func TestIsMissingStreamErrorRecognizesRedisMissingKeyReply(t *testing.T) {
	if !isMissingStreamError(errors.New("ERR no such key")) {
		t.Fatal("missing-key reply was not recognized")
	}
	if isMissingStreamError(errors.New("connection refused")) {
		t.Fatal("unrelated Redis error was treated as a missing stream")
	}
}

func TestPublisherHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	publisher := newPublisher(&fakeStream{})
	err := publisher.Publish(ctx, Event{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("publish error = %v, want context canceled", err)
	}
}

type fakeStream struct {
	appendID string
	key      string
	maxLen   int64
	values   map[string]string
}

func (stream *fakeStream) Append(_ context.Context, key string, maxLen int64, values map[string]string) (string, error) {
	stream.key = key
	stream.maxLen = maxLen
	stream.values = values
	return stream.appendID, nil
}

func (stream *fakeStream) Read(context.Context, string, int, time.Duration) ([]Message, error) {
	return nil, nil
}

func (stream *fakeStream) HasAfter(context.Context, string) (bool, error) {
	return false, nil
}

func (stream *fakeStream) FirstID(context.Context) (string, error) {
	return "", nil
}

func (stream *fakeStream) LastID(context.Context) (string, error) {
	return "", nil
}
