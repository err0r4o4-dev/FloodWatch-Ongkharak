package fm91

import (
	"context"
	"testing"
	"time"
)

func TestProviderPublishesUpcomingTrafficWithoutFutureObservation(t *testing.T) {
	fetcher := &fixtureFetcher{responses: map[string][]byte{
		mainURL:             []byte(listingFor("79953", "องครักษ์ ปิดการจราจร")),
		articleURL("79953"): []byte(articleWithBody("องครักษ์ ปิดการจราจร", "ปิดการจราจรวันที่ 8 ตุลาคม 2569 เวลา 09:00 น. บริเวณคลอง 14 โปรดเลี่ยงเส้นทาง")),
	}}
	provider := NewWithFetcher(fetcher)
	provider.Clock = func() time.Time { return time.Date(2026, 10, 5, 12, 0, 0, 0, bangkokLocation) }
	batch, err := provider.Fetch(context.Background())
	if err != nil || len(batch.Records) != 1 {
		t.Fatalf("upcoming traffic news not collected: records=%d err=%v", len(batch.Records), err)
	}
	if batch.Records[0].ObservedAt != nil {
		t.Fatal("scheduled future closure became an observation")
	}
	assertContains(t, batch.Records[0].News.EventTimes, "วันที่ 8 ตุลาคม 2569 เวลา 09:00 น.")
}
