package router

import (
	"bufio"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/realtime"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

func TestGuestSSEReplaysPublicRedisStreamWithoutPrivatePayload(t *testing.T) {
	redisURL := os.Getenv("TEST_REDIS_URL")
	if redisURL == "" {
		t.Skip("set TEST_REDIS_URL for Redis-backed SSE integration")
	}
	options, err := redis.ParseURL(redisURL)
	if err != nil {
		t.Fatal(err)
	}
	client := redis.NewClient(options)
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	publisher := realtime.NewPublisher(client)
	eventIDs := []string{uuid.NewString(), uuid.NewString()}
	streamIDs := make([]string, 0, 2)
	defer func() {
		if len(streamIDs) > 0 {
			_ = client.XDel(context.Background(), realtime.PublicStreamKey, streamIDs...).Err()
		}
	}()
	for _, eventID := range eventIDs {
		message := realtime.Event{EventID: eventID, EventType: realtime.EventReportVerified,
			OccurredAt: time.Now().UTC(), PublishedAt: time.Now().UTC(), SourceType: realtime.SourceCommunity,
			ResourceType: realtime.ResourceCommunityReport, ResourceID: uuid.NewString(), PayloadVersion: 1,
			Payload: json.RawMessage(`{}`)}
		if err := publisher.Publish(ctx, message); err != nil {
			t.Fatal(err)
		}
		entries, err := client.XRevRangeN(ctx, realtime.PublicStreamKey, "+", "-", 1).Result()
		if err != nil || len(entries) != 1 {
			t.Fatalf("get appended Redis ID: %v", err)
		}
		streamIDs = append(streamIDs, entries[0].ID)
	}
	server := httptest.NewServer(New(Dependencies{Logger: slog.Default(), RedisClient: client}))
	defer server.Close()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/api/v1/events/stream", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Last-Event-ID", streamIDs[0])
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || !strings.HasPrefix(response.Header.Get("Content-Type"), "text/event-stream") {
		t.Fatalf("guest stream status = %d, content type = %s", response.StatusCode, response.Header.Get("Content-Type"))
	}
	scanner := bufio.NewScanner(response.Body)
	var eventType, body, cursor string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "event: ") {
			eventType = strings.TrimPrefix(line, "event: ")
		}
		if strings.HasPrefix(line, "id: ") {
			cursor = strings.TrimPrefix(line, "id: ")
		}
		if strings.HasPrefix(line, "data: ") {
			body = strings.TrimPrefix(line, "data: ")
		}
		if body != "" {
			break
		}
	}
	if eventType != "report.verified" || cursor != streamIDs[1] || !strings.Contains(body, eventIDs[1]) || strings.Contains(body, "userId") {
		t.Fatalf("replayed event = %q, cursor = %q, payload = %q, stream error = %v", eventType, cursor, body, scanner.Err())
	}
}
