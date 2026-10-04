package tmd

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/externaldata"
)

func TestFetchDeduplicatesCAPAcrossDifferentFeedLinks(t *testing.T) {
	const document = `<alert xmlns="urn:oasis:names:tc:emergency:cap:1.2"><identifier>SYNTH-DUPLICATE</identifier><sender>test@example.invalid</sender><sent>2026-10-04T00:00:00Z</sent><status>Actual</status><msgType>Alert</msgType><info><expires>2026-10-05T00:00:00Z</expires><headline>Synthetic only</headline><area><areaDesc>Test</areaDesc><geocode><valueName>ISO</valueName><value>TH-26</value></geocode></area></info></alert>`
	for _, scenario := range []string{"same-content", "conflicting-content"} {
		t.Run(scenario, func(t *testing.T) {
			duplicate := document
			if scenario == "conflicting-content" {
				duplicate = strings.Replace(document, "Synthetic only", "Conflicting synthetic content", 1)
			}
			provider, err := NewWithFetcher(syntheticFetcher{responses: map[string][]byte{
				rssPath:                  []byte(`<rss><channel><item><link>/uploads/CAP/first.xml</link></item><item><link>/uploads/CAP/alias.xml</link></item><item><link>/uploads/CAP/first.xml</link></item></channel></rss>`),
				"/uploads/CAP/first.xml": []byte(document),
				"/uploads/CAP/alias.xml": []byte(duplicate),
			}})
			if err != nil {
				t.Fatal(err)
			}
			now := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
			provider.now = func() time.Time { return now }
			batch, err := provider.Fetch(context.Background())
			if scenario == "conflicting-content" {
				if err == nil {
					t.Fatal("conflicting duplicate identifier accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(batch.Records) != 1 || len(batch.Raw) != 3 {
				t.Fatalf("records=%d raw=%d, want one alert with both source documents", len(batch.Records), len(batch.Raw))
			}
			if err := externaldata.ValidateBatch("TMD_CAP", batch, now); err != nil {
				t.Fatalf("deduplicated feed rejected: %v", err)
			}
		})
	}
}
