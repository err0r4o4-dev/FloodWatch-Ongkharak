package realtime

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestModelProvenanceOnlyAllowedForExternalEvidence(t *testing.T) {
	now := time.Now().UTC()
	event := Event{EventID: uuid.NewString(), EventType: EventType("external_data.updated"), OccurredAt: now, PublishedAt: now, SourceType: SourceType("MODEL"), ResourceType: ResourceType("EXTERNAL_DATA"), ResourceID: uuid.NewString(), PayloadVersion: 1, Payload: json.RawMessage(`{"sourceCode":"OPENMETEO_ECMWF"}`)}
	if err := event.Validate(); err != nil {
		t.Fatalf("external MODEL event rejected: %v", err)
	}
	event.EventType = EventWaterLevelUpdated
	event.ResourceType = ResourceWaterStation
	if err := event.Validate(); err == nil {
		t.Fatal("MODEL event accepted as station water evidence")
	}
}
