package newsannouncement

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestPublicNewsTimestampsAreUTCRegardlessOfDatabaseScanLocation(t *testing.T) {
	now := time.Now().UTC()
	local := now.In(time.FixedZone("Asia/Bangkok", 7*60*60))
	entry := evidence("synthetic", local.Add(-time.Minute))
	store := &memoryEvidence{data: EvidenceSnapshot{Enabled: true, LastSuccessAt: &local, Records: []Evidence{entry}}}
	service := NewService(store, true)
	service.Clock = func() time.Time { return now }
	result, err := service.List(context.Background(), 20, 0)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), "+07:00") {
		t.Fatal("public metadata retained database-local offsets instead of UTC")
	}
	if store.data.LastSuccessAt.Location() == time.UTC {
		t.Fatal("presentation mutated retained source timestamps")
	}
}
