package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/realtime"
	"github.com/redis/go-redis/v9"
)

const (
	eventHeartbeat    = 15 * time.Second
	eventReadBlock    = time.Second
	eventReplayLimit  = 100
	eventWriteTimeout = 5 * time.Second
	eventMaxClients   = 128
)

type eventStreamOptions struct {
	heartbeat   time.Duration
	replayLimit int
	maxClients  int
}

func NewEventStreamHandler(client redis.UniversalClient) http.Handler {
	return newEventStreamHandler(realtime.NewRedisStream(client), eventStreamOptions{heartbeat: eventHeartbeat, replayLimit: eventReplayLimit})
}

func newEventStreamHandler(stream realtime.Stream, options eventStreamOptions) http.Handler {
	if options.heartbeat <= 0 {
		options.heartbeat = eventHeartbeat
	}
	if options.replayLimit <= 0 {
		options.replayLimit = eventReplayLimit
	}
	if options.maxClients <= 0 {
		options.maxClients = eventMaxClients
	}
	slots := make(chan struct{}, options.maxClients)
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		select {
		case slots <- struct{}{}:
			defer func() { <-slots }()
		default:
			response.Header().Set("Retry-After", "5")
			http.Error(response, "event stream capacity reached", http.StatusServiceUnavailable)
			return
		}
		cursor, hasCursor, err := eventCursor(request)
		if err != nil {
			http.Error(response, "invalid event cursor", http.StatusBadRequest)
			return
		}
		if hasCursor {
			firstID, firstErr := stream.FirstID(request.Context())
			lastID, lastErr := stream.LastID(request.Context())
			if firstErr != nil || lastErr != nil {
				http.Error(response, "event stream is unavailable", http.StatusServiceUnavailable)
				return
			}
			if firstID == "" || lastID == "" || realtime.CompareCursors(cursor, firstID) < 0 || realtime.CompareCursors(cursor, lastID) > 0 {
				writeEventReset(response)
				return
			}
		} else {
			cursor = "$"
		}

		prepareEventStream(response)
		if err := writeComment(response, "connected"); err != nil {
			return
		}
		replayRemaining := options.replayLimit
		replaying := hasCursor
		lastHeartbeat := time.Now()
		for {
			if err := request.Context().Err(); err != nil {
				return
			}
			count := eventReplayLimit
			if replaying && replayRemaining < count {
				count = replayRemaining
			}
			block := options.heartbeat
			if block > eventReadBlock {
				block = eventReadBlock
			}
			messages, readErr := stream.Read(request.Context(), cursor, count, block)
			if readErr != nil {
				if errors.Is(readErr, context.Canceled) || errors.Is(readErr, context.DeadlineExceeded) {
					return
				}
				writeEventReset(response)
				return
			}
			if len(messages) == 0 {
				replaying = false
				if time.Since(lastHeartbeat) >= options.heartbeat {
					if err := writeComment(response, "heartbeat"); err != nil {
						return
					}
					lastHeartbeat = time.Now()
				}
				continue
			}
			for _, message := range messages {
				cursor = message.ID
				if replaying {
					replayRemaining--
				}
				event, err := decodePublicEvent(message)
				if err != nil {
					continue
				}
				encoded, err := json.Marshal(event)
				if err != nil || writeSSE(response, message.ID, string(event.EventType), string(encoded)) != nil {
					return
				}
			}
			if replaying {
				if len(messages) < count {
					replaying = false
					continue
				}
				if replayRemaining == 0 {
					more, moreErr := stream.HasAfter(request.Context(), cursor)
					if moreErr != nil {
						writeEventReset(response)
						return
					}
					if more {
						_ = writeSSE(response, "", "replay.reset", `{"reason":"replay_limit","action":"refetch"}`)
						return
					}
					replaying = false
				}
			}
		}
	})
}

func eventCursor(request *http.Request) (string, bool, error) {
	if header := request.Header.Get("Last-Event-ID"); header != "" {
		if !realtime.ValidCursor(header) {
			return "", false, errors.New("invalid Last-Event-ID")
		}
		return header, true, nil
	}
	values, present := request.URL.Query()["cursor"]
	if !present {
		return "", false, nil
	}
	if len(values) != 1 || !realtime.ValidCursor(values[0]) {
		return "", false, errors.New("invalid cursor")
	}
	return values[0], true, nil
}

func decodePublicEvent(message realtime.Message) (realtime.Event, error) {
	encoded, ok := message.Values["event"]
	if !ok {
		return realtime.Event{}, errors.New("missing event")
	}
	var event realtime.Event
	if err := json.Unmarshal([]byte(encoded), &event); err != nil {
		return realtime.Event{}, fmt.Errorf("decode event: %w", err)
	}
	if err := event.Validate(); err != nil {
		return realtime.Event{}, err
	}
	return event, nil
}

func prepareEventStream(response http.ResponseWriter) {
	response.Header().Set("Cache-Control", "no-cache")
	response.Header().Set("Connection", "keep-alive")
	response.Header().Set("Content-Type", "text/event-stream")
	response.Header().Set("X-Accel-Buffering", "no")
}

func writeEventReset(response http.ResponseWriter) {
	if response.Header().Get("Content-Type") == "" {
		prepareEventStream(response)
	}
	_ = writeSSE(response, "", "replay.reset", `{"reason":"cursor_expired","action":"refetch"}`)
}

func writeComment(response http.ResponseWriter, value string) error {
	return writeEventBytes(response, []byte(": "+value+"\n\n"))
}

func writeSSE(response http.ResponseWriter, id, eventType, data string) error {
	prefix := ""
	if id != "" {
		prefix = "id: " + id + "\n"
	}
	return writeEventBytes(response, []byte(prefix+"event: "+eventType+"\ndata: "+data+"\n\n"))
}

func writeEventBytes(response http.ResponseWriter, value []byte) error {
	controller := http.NewResponseController(response)
	_ = controller.SetWriteDeadline(time.Now().Add(eventWriteTimeout))
	defer controller.SetWriteDeadline(time.Time{})
	if _, err := response.Write(value); err != nil {
		return err
	}
	return controller.Flush()
}
