package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	PublicStreamKey    = "floodwatch:public:v1"
	PublicStreamMaxLen = 10_000
	PayloadVersion1    = 1
	MaxPayloadBytes    = 4 * 1024
)

var ErrInvalidEvent = errors.New("invalid public realtime event")

type EventType string

const (
	EventWaterLevelUpdated   EventType = "water_level.updated"
	EventFloodEventCreated   EventType = "flood_event.created"
	EventFloodEventUpdated   EventType = "flood_event.updated"
	EventRoadStatusChanged   EventType = "road.status.changed"
	EventReportVerified      EventType = "report.verified"
	EventReportUpdated       EventType = "report.updated"
	EventReportAreaUpdated   EventType = "report-area.updated"
	EventAlertCreated        EventType = "alert.created"
	EventExternalDataUpdated EventType = "external_data.updated"
)

type SourceType string

const (
	SourceOfficial  SourceType = "OFFICIAL"
	SourceCommunity SourceType = "COMMUNITY"
	SourceAI        SourceType = "AI"
	SourceModel     SourceType = "MODEL"
	SourceMedia     SourceType = "MEDIA"
)

type ResourceType string

const (
	ResourceWaterStation    ResourceType = "WATER_STATION"
	ResourceFloodEvent      ResourceType = "FLOOD_EVENT"
	ResourceRoad            ResourceType = "ROAD"
	ResourceCommunityReport ResourceType = "COMMUNITY_REPORT"
	ResourceReportArea      ResourceType = "REPORT_AREA"
	ResourceAlert           ResourceType = "ALERT"
	ResourceExternalData    ResourceType = "EXTERNAL_DATA"
)

type Event struct {
	EventID        string          `json:"eventId"`
	EventType      EventType       `json:"eventType"`
	OccurredAt     time.Time       `json:"occurredAt"`
	PublishedAt    time.Time       `json:"publishedAt"`
	SourceType     SourceType      `json:"sourceType"`
	ResourceType   ResourceType    `json:"resourceType"`
	ResourceID     string          `json:"resourceId"`
	PayloadVersion int             `json:"payloadVersion"`
	Payload        json.RawMessage `json:"payload"`
}

func (event Event) Validate() error {
	if _, err := uuid.Parse(event.EventID); err != nil || event.OccurredAt.IsZero() || event.PublishedAt.IsZero() {
		return ErrInvalidEvent
	}
	modelEvidence := event.EventType == EventExternalDataUpdated && event.ResourceType == ResourceExternalData && (event.SourceType == SourceModel || event.SourceType == SourceMedia)
	if !validPublicPair(event.EventType, event.ResourceType) || (!publicSourceType(event.SourceType) && !modelEvidence) {
		return ErrInvalidEvent
	}
	if event.EventType == EventExternalDataUpdated && event.SourceType != SourceOfficial && event.SourceType != SourceModel && event.SourceType != SourceMedia {
		return ErrInvalidEvent
	}
	if (event.EventType == EventReportVerified || event.EventType == EventReportUpdated || event.EventType == EventReportAreaUpdated) && event.SourceType != SourceCommunity {
		return ErrInvalidEvent
	}
	if _, err := uuid.Parse(event.ResourceID); err != nil || event.PayloadVersion != PayloadVersion1 || len(event.Payload) > MaxPayloadBytes {
		return ErrInvalidEvent
	}
	var payload map[string]json.RawMessage
	if len(event.Payload) == 0 || json.Unmarshal(event.Payload, &payload) != nil || payload == nil {
		return ErrInvalidEvent
	}
	if event.EventType == EventReportAreaUpdated && len(payload) != 0 {
		return ErrInvalidEvent
	}
	return nil
}

type Message struct {
	ID     string
	Values map[string]string
}

type Stream interface {
	Append(context.Context, string, int64, map[string]string) (string, error)
	Read(context.Context, string, int, time.Duration) ([]Message, error)
	HasAfter(context.Context, string) (bool, error)
	FirstID(context.Context) (string, error)
	LastID(context.Context) (string, error)
}

type Publisher struct {
	stream Stream
}

func NewPublisher(client redis.UniversalClient) *Publisher {
	return newPublisher(NewRedisStream(client))
}

func newPublisher(stream Stream) *Publisher {
	return &Publisher{stream: stream}
}

func (publisher *Publisher) Publish(ctx context.Context, event Event) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := event.Validate(); err != nil {
		return err
	}
	encoded, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("encode public realtime event: %w", err)
	}
	if _, err := publisher.stream.Append(ctx, PublicStreamKey, PublicStreamMaxLen, map[string]string{"event": string(encoded)}); err != nil {
		return fmt.Errorf("publish public realtime event: %w", err)
	}
	return nil
}

type RedisStream struct {
	client redis.UniversalClient
}

func NewRedisStream(client redis.UniversalClient) *RedisStream {
	return &RedisStream{client: client}
}

func (stream *RedisStream) Append(ctx context.Context, key string, maxLen int64, values map[string]string) (string, error) {
	return stream.client.XAdd(ctx, &redis.XAddArgs{Stream: key, MaxLen: maxLen, Approx: true, Values: values}).Result()
}

func (stream *RedisStream) Read(ctx context.Context, afterID string, count int, block time.Duration) ([]Message, error) {
	streams, err := stream.client.XRead(ctx, &redis.XReadArgs{Streams: []string{PublicStreamKey, afterID}, Count: int64(count), Block: block}).Result()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	messages := make([]Message, 0)
	for _, result := range streams {
		for _, entry := range result.Messages {
			values := make(map[string]string, len(entry.Values))
			for key, value := range entry.Values {
				stringValue, ok := value.(string)
				if !ok {
					continue
				}
				values[key] = stringValue
			}
			messages = append(messages, Message{ID: entry.ID, Values: values})
		}
	}
	return messages, nil
}

func (stream *RedisStream) HasAfter(ctx context.Context, afterID string) (bool, error) {
	messages, err := stream.client.XRangeN(ctx, PublicStreamKey, "("+afterID, "+", 1).Result()
	if isMissingStreamError(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return len(messages) > 0, nil
}

func (stream *RedisStream) FirstID(ctx context.Context) (string, error) {
	info, err := stream.client.XInfoStream(ctx, PublicStreamKey).Result()
	if isMissingStreamError(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return info.FirstEntry.ID, nil
}

func (stream *RedisStream) LastID(ctx context.Context) (string, error) {
	info, err := stream.client.XInfoStream(ctx, PublicStreamKey).Result()
	if isMissingStreamError(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return info.LastEntry.ID, nil
}

func ValidCursor(value string) bool {
	_, _, ok := parseStreamID(value)
	return ok
}

func CompareCursors(left, right string) int {
	leftMilliseconds, leftSequence, leftOK := parseStreamID(left)
	rightMilliseconds, rightSequence, rightOK := parseStreamID(right)
	if !leftOK || !rightOK {
		return 0
	}
	if leftMilliseconds < rightMilliseconds {
		return -1
	}
	if leftMilliseconds > rightMilliseconds {
		return 1
	}
	if leftSequence < rightSequence {
		return -1
	}
	if leftSequence > rightSequence {
		return 1
	}
	return 0
}

func isMissingStreamError(err error) bool {
	return errors.Is(err, redis.Nil) || (err != nil && strings.Contains(strings.ToLower(err.Error()), "no such key"))
}

func validPublicPair(eventType EventType, resourceType ResourceType) bool {
	switch eventType {
	case EventWaterLevelUpdated:
		return resourceType == ResourceWaterStation
	case EventFloodEventCreated, EventFloodEventUpdated:
		return resourceType == ResourceFloodEvent
	case EventRoadStatusChanged:
		return resourceType == ResourceRoad
	case EventReportVerified, EventReportUpdated:
		return resourceType == ResourceCommunityReport
	case EventReportAreaUpdated:
		return resourceType == ResourceReportArea
	case EventAlertCreated:
		return resourceType == ResourceAlert
	case EventExternalDataUpdated:
		return resourceType == ResourceExternalData
	default:
		return false
	}
}

func publicSourceType(value SourceType) bool {
	return value == SourceOfficial || value == SourceCommunity || value == SourceAI
}

func parseStreamID(value string) (uint64, uint64, bool) {
	parts := strings.Split(value, "-")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return 0, 0, false
	}
	milliseconds, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil {
		return 0, 0, false
	}
	sequence, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		return 0, 0, false
	}
	return milliseconds, sequence, true
}
