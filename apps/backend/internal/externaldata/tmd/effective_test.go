package tmd

import (
	"context"
	"testing"
	"time"
)

func TestCAPEffectiveTimeIsPreservedAndInvalidWindowRejected(t *testing.T) {
	for _, scenario := range []struct {
		effective string
		valid     bool
	}{{"2026-10-04T14:00:00Z", true}, {"2026-10-06T00:00:00Z", false}, {"not-a-time", false}} {
		document := `<alert xmlns="urn:oasis:names:tc:emergency:cap:1.2"><identifier>SYNTH-EFFECTIVE</identifier><sender>test@example.invalid</sender><sent>2026-10-04T00:00:00Z</sent><status>Actual</status><msgType>Alert</msgType><info><effective>` + scenario.effective + `</effective><expires>2026-10-05T00:00:00Z</expires><headline>Synthetic only</headline><area><areaDesc>Test</areaDesc><geocode><valueName>ISO</valueName><value>TH-26</value></geocode></area></info></alert>`
		provider, err := NewWithFetcher(syntheticFetcher{responses: map[string][]byte{rssPath: []byte(`<rss><channel><item><link>/uploads/CAP/effective.xml</link></item></channel></rss>`), "/uploads/CAP/effective.xml": []byte(document)}})
		if err != nil {
			t.Fatal(err)
		}
		provider.now = func() time.Time { return time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC) }
		batch, err := provider.Fetch(context.Background())
		if !scenario.valid {
			if err == nil {
				t.Fatal("invalid effective window accepted")
			}
			continue
		}
		if err != nil || len(batch.Records) != 1 || batch.Records[0].ValidFrom == nil || !batch.Records[0].ValidFrom.Equal(time.Date(2026, 10, 4, 14, 0, 0, 0, time.UTC)) {
			t.Fatalf("effective time lost: %+v err=%v", batch.Records, err)
		}
	}
}

func TestCAPMultipleInfoUsesConservativeEffectiveWindow(t *testing.T) {
	document := `<alert xmlns="urn:oasis:names:tc:emergency:cap:1.2"><identifier>SYNTH-MULTI-INFO</identifier><sender>test@example.invalid</sender><sent>2026-10-04T00:00:00Z</sent><status>Actual</status><msgType>Alert</msgType><info><effective>2026-10-04T02:00:00Z</effective><expires>2026-10-05T00:00:00Z</expires></info><info><effective>2026-10-04T04:00:00Z</effective><expires>2026-10-06T00:00:00Z</expires></info></alert>`
	alert, err := parseCAP([]byte(document), time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC))
	if err != nil || !alert.Effective.Equal(time.Date(2026, 10, 4, 4, 0, 0, 0, time.UTC)) || !alert.Expiry.Equal(time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("multi-info bounds: alert=%+v err=%v", alert, err)
	}
}

func TestCAPUnrelatedAreaWindowDoesNotSuppressLocalAnnouncement(t *testing.T) {
	document := `<alert xmlns="urn:oasis:names:tc:emergency:cap:1.2"><identifier>SYNTH-GEO-WINDOW</identifier><sender>test@example.invalid</sender><sent>2026-10-04T00:00:00Z</sent><status>Actual</status><msgType>Alert</msgType><info><effective>2026-10-04T01:00:00Z</effective><expires>2026-10-05T00:00:00Z</expires><area><areaDesc>Local test</areaDesc><geocode><valueName>ISO</valueName><value>TH-26</value></geocode></area></info><info><effective>2026-10-06T00:00:00Z</effective><expires>2026-10-07T00:00:00Z</expires><area><areaDesc>Unrelated test</areaDesc><geocode><valueName>ISO</valueName><value>TH-10</value></geocode></area></info></alert>`
	alert, err := parseCAP([]byte(document), time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC))
	if err != nil || !alert.Effective.Equal(time.Date(2026, 10, 4, 1, 0, 0, 0, time.UTC)) || !alert.Expiry.Equal(time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("unrelated area affected local bounds: alert=%+v err=%v", alert, err)
	}
}

func TestCAPDisjointSelectedWindowsRemainIndeterminateEvidence(t *testing.T) {
	document := `<alert xmlns="urn:oasis:names:tc:emergency:cap:1.2"><identifier>SYNTH-DISJOINT</identifier><sender>test@example.invalid</sender><sent>2026-10-04T00:00:00Z</sent><status>Actual</status><msgType>Alert</msgType><info><effective>2026-10-04T01:00:00Z</effective><expires>2026-10-04T12:00:00Z</expires><area><areaDesc>Local</areaDesc><geocode><valueName>ISO</valueName><value>TH-26</value></geocode></area></info><info><effective>2026-10-04T13:00:00Z</effective><expires>2026-10-04T14:00:00Z</expires><area><areaDesc>Local</areaDesc><geocode><valueName>ISO</valueName><value>TH-26</value></geocode></area></info></alert>`
	alert, err := parseCAP([]byte(document), time.Date(2026, 10, 4, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	area, _ := matchingContext(alert.Infos)
	record := makeRecord(alert, area)
	if record.ValidFrom != nil || record.ValidTo != nil || record.Reason != "CONFLICTING_TIME_WINDOWS" {
		t.Fatalf("disjoint envelope was not indeterminate: %+v", record)
	}
}
