package fm91

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/externaldata"
)

func TestLiveFM91Smoke(t *testing.T) {
	if os.Getenv("TEST_FM91_LIVE") != "1" {
		t.Skip("set TEST_FM91_LIVE=1 for the read-only FM91 smoke check")
	}

	provider, err := New()
	if err != nil {
		t.Fatalf("live provider setup failed: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	batch, err := provider.Fetch(ctx)
	if err != nil {
		t.Fatalf("live FM91 fetch failed: %v", err)
	}
	for _, record := range batch.Records {
		if !externaldata.ValidNewsRecord(record, batch.RetrievedAt) {
			t.Fatal("live FM91 record validation failed")
		}
	}
	t.Logf("FM91 live smoke: raw=%d records=%d validation=ok", len(batch.Raw), len(batch.Records))
}
