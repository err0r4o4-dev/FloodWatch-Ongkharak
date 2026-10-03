package main

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/floodwatch-ongkharak/backend/internal/flood"
	"github.com/floodwatch-ongkharak/backend/internal/ingestion"
)

type sourceListStore struct {
	ingestion.Store
	sources []flood.DataSource
	err     error
}

func (store sourceListStore) Sources(context.Context) ([]flood.DataSource, error) {
	return store.sources, store.err
}

func TestRegisteredSourceStoreFiltersSources(t *testing.T) {
	testCases := []struct {
		name       string
		code       string
		sourceType flood.SourceType
		registered bool
		wantCount  int
		wantLevel  string
	}{
		{name: "internal AI evidence is not a polling source", code: "AI_VISION_V1", sourceType: flood.SourceAI, wantLevel: "DEBUG"},
		{name: "unknown AI source still warns", code: "UNKNOWN_AI", sourceType: flood.SourceAI, wantLevel: "WARN"},
		{name: "internal code with wrong provenance still warns", code: "AI_VISION_V1", sourceType: flood.SourceOfficial, wantLevel: "WARN"},
		{name: "unregistered official source still warns", code: "UNKNOWN_OFFICIAL", sourceType: flood.SourceOfficial, wantLevel: "WARN"},
		{name: "unregistered community source still warns", code: "UNKNOWN_COMMUNITY", sourceType: flood.SourceCommunity, wantLevel: "WARN"},
		{name: "registered official source is returned", code: "REGISTERED", sourceType: flood.SourceOfficial, registered: true, wantCount: 1},
		{name: "explicitly registered AI source is returned", code: "AI_VISION_V1", sourceType: flood.SourceAI, registered: true, wantCount: 1},
		{name: "community fixture is returned", code: "SYNTH_FIXTURE", sourceType: flood.SourceCommunity, registered: true, wantCount: 1},
		{name: "official fixture is rejected", code: "SYNTH_FIXTURE", sourceType: flood.SourceOfficial, registered: true, wantLevel: "ERROR"},
		{name: "AI fixture is rejected", code: "SYNTH_FIXTURE", sourceType: flood.SourceAI, registered: true, wantLevel: "ERROR"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			source := flood.DataSource{ID: "source-id", Code: testCase.code, SourceType: testCase.sourceType, Enabled: true}
			var logs bytes.Buffer
			registeredCodes := make(map[string]struct{})
			if testCase.registered {
				registeredCodes[source.Code] = struct{}{}
			}
			store := registeredSourceStore{
				Store:             sourceListStore{sources: []flood.DataSource{source}},
				registeredCodes:   registeredCodes,
				fixtureSourceCode: "SYNTH_FIXTURE",
				logger:            slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})),
			}
			sources, err := store.Sources(context.Background())
			if err != nil || len(sources) != testCase.wantCount {
				t.Fatalf("Sources() = %v, %v; want %d sources", sources, err, testCase.wantCount)
			}
			if testCase.wantCount == 1 && sources[0] != source {
				t.Fatalf("registered source changed: %+v", sources[0])
			}
			if testCase.wantLevel == "" {
				if logs.Len() != 0 {
					t.Fatalf("unexpected log: %s", logs.String())
				}
			} else if !strings.Contains(logs.String(), `"level":"`+testCase.wantLevel+`"`) || !strings.Contains(logs.String(), `"source_code":"`+source.Code+`"`) {
				t.Fatalf("expected %s log identifying source %s; got %s", testCase.wantLevel, source.Code, logs.String())
			}
		})
	}
}

func TestRegisteredSourceStorePropagatesSourceError(t *testing.T) {
	wantErr := errors.New("source listing unavailable")
	store := registeredSourceStore{Store: sourceListStore{err: wantErr}}
	sources, err := store.Sources(context.Background())
	if sources != nil || !errors.Is(err, wantErr) {
		t.Fatalf("Sources() = %v, %v; want nil, %v", sources, err, wantErr)
	}
}

func TestRegisteredSourceStoreSkipsInternalAIWithoutLogger(t *testing.T) {
	store := registeredSourceStore{Store: sourceListStore{sources: []flood.DataSource{
		{Code: "AI_VISION_V1", SourceType: flood.SourceAI, Enabled: true},
	}}}
	sources, err := store.Sources(context.Background())
	if err != nil || len(sources) != 0 {
		t.Fatalf("Sources() = %v, %v; want no polling sources", sources, err)
	}
}
