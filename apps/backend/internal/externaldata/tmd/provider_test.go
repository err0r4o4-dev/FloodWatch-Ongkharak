package tmd

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

type syntheticFetcher struct {
	responses map[string][]byte
	err       error
}

func (fetcher syntheticFetcher) Fetch(_ context.Context, path string) ([]byte, error) {
	if fetcher.err != nil {
		return nil, fetcher.err
	}
	body, ok := fetcher.responses[path]
	if !ok {
		return nil, errors.New("unexpected synthetic path: " + path)
	}
	return body, nil
}

func TestFetchCAPKeepsOnlyMatchingSyntheticAlertGeometryAndRawEvidence(t *testing.T) {
	const rss = `<?xml version="1.0"?><rss><channel><item><title>Synthetic CAP</title><link>/uploads/CAP/CAPTMD20261003165356_2.xml</link></item></channel></rss>`
	const cap = "<?xml version=\"1.0\"?><alert xmlns=\"urn:oasis:names:tc:emergency:cap:1.2\"><identifier>TMD-SYNTH-1</identifier><sender>synthetic@example.invalid</sender><sent>2026-10-03T16:53:56+07:00</sent><status>Actual</status><msgType>Alert</msgType><info><language>en-US</language><category>Met</category><event>Synthetic flood watch</event><urgency>Expected</urgency><severity>Moderate</severity><certainty>Likely</certainty><expires>2026-10-04T18:00:00+07:00</expires><headline>Synthetic alert for Ongkharak</headline><description>Explicitly synthetic CAP content.</description><area><areaDesc>Synthetic Ongkharak area</areaDesc><geocode><valueName>FIPS6</valueName><value>TH-26</value></geocode><polygon>14.00,100.90 14.00,101.10 14.20,101.10 14.20,100.90 14.00,100.90</polygon></area></info></alert>"
	provider, err := NewWithFetcher(syntheticFetcher{responses: map[string][]byte{
		rssPath: []byte(rss),
		"/uploads/CAP/CAPTMD20261003165356_2.xml": []byte(cap),
	}})
	if err != nil {
		t.Fatal(err)
	}
	provider.now = func() time.Time { return time.Date(2026, 10, 3, 12, 0, 0, 0, time.FixedZone("Asia/Bangkok", 7*60*60)) }
	batch, err := provider.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(batch.Records) != 1 || len(batch.Raw) != 2 {
		t.Fatalf("batch = %+v", batch)
	}
	if !strings.Contains(string(batch.Records[0].Geometry), `"type":"Polygon"`) || !strings.Contains(string(batch.Records[0].Geometry), "101") {
		t.Fatalf("geometry = %s", batch.Records[0].Geometry)
	}
	if batch.Records[0].AlertIdentifier != "TMD-SYNTH-1" || batch.Records[0].Provenance != "OFFICIAL" || batch.Records[0].Kind != "OFFICIAL_ALERT" {
		t.Fatalf("record = %+v", batch.Records[0])
	}
	if batch.Raw[0].URL != tmdBase+rssPath || batch.Raw[1].URL != tmdBase+"/uploads/CAP/CAPTMD20261003165356_2.xml" {
		t.Fatalf("raw URLs = %+v", batch.Raw)
	}
}

func TestFetchCAPSkipsExpiredUnmatchedAndExerciseDocumentsButKeepsRaw(t *testing.T) {
	const rss = `<?xml version="1.0"?><rss><channel><item><link>/uploads/CAP/expired.xml</link></item><item><link>/uploads/CAP/unmatched.xml</link></item><item><link>/uploads/CAP/exercise.xml</link></item></channel></rss>`
	const base = `<alert xmlns="urn:oasis:names:tc:emergency:cap:1.2"><identifier>%s</identifier><sender>synthetic@example.invalid</sender><sent>2026-10-03T00:00:00+07:00</sent><status>%s</status><msgType>Alert</msgType><references>%s</references><info><event>Synthetic</event><expires>%s</expires><headline>Synthetic</headline><description>Synthetic</description><area><areaDesc>Area</areaDesc><geocode><valueName>FIPS6</valueName><value>%s</value></geocode></area></info></alert>`
	responses := map[string][]byte{
		rssPath:                      []byte(rss),
		"/uploads/CAP/expired.xml":   []byte(fmt.Sprintf(base, "expired", "Actual", "synthetic@example.invalid,expired,2026-10-03T00:00:00+07:00", "2026-10-03T01:00:00+07:00", "TH-26")),
		"/uploads/CAP/unmatched.xml": []byte(fmt.Sprintf(base, "unmatched", "Actual", "synthetic@example.invalid,unmatched,2026-10-03T00:00:00+07:00", "2026-10-05T01:00:00+07:00", "TH-25")),
		"/uploads/CAP/exercise.xml":  []byte(fmt.Sprintf(base, "exercise", "Test", "synthetic@example.invalid,exercise,2026-10-03T00:00:00+07:00", "2026-10-05T01:00:00+07:00", "TH-26")),
	}
	provider, err := NewWithFetcher(syntheticFetcher{responses: responses})
	if err != nil {
		t.Fatal(err)
	}
	provider.now = func() time.Time { return time.Date(2026, 10, 4, 12, 0, 0, 0, time.FixedZone("Asia/Bangkok", 7*60*60)) }
	batch, err := provider.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(batch.Records) != 0 || len(batch.Raw) != 4 {
		t.Fatalf("records/raw = %d/%d", len(batch.Records), len(batch.Raw))
	}
}

func TestFetchCAPPreservesSyntheticUpdateAndCancelReferencesWithoutArea(t *testing.T) {
	const rss = `<rss><channel><item><link>/uploads/CAP/base.xml</link></item><item><link>/uploads/CAP/update.xml</link></item><item><link>/uploads/CAP/cancel.xml</link></item></channel></rss>`
	const base = `<alert xmlns="urn:oasis:names:tc:emergency:cap:1.2"><identifier>%s</identifier><sender>synthetic@example.invalid</sender><sent>2026-10-03T00:00:00+07:00</sent><status>Actual</status><msgType>%s</msgType><references>%s</references><info><expires>2026-10-05T00:00:00+07:00</expires><headline>Synthetic %s</headline><description>Synthetic lifecycle message</description>%s</info></alert>`
	const cancel = `<alert xmlns="urn:oasis:names:tc:emergency:cap:1.2"><identifier>TMD-CANCEL-1</identifier><sender>synthetic@example.invalid</sender><sent>2026-10-03T00:00:00+07:00</sent><status>Actual</status><msgType>Cancel</msgType><references>synthetic@example.invalid,TMD-UPDATE-1,2026-10-03T00:00:00+07:00</references></alert>`
	area := `<area><areaDesc>Synthetic Ongkharak</areaDesc><geocode><valueName>FIPS6</valueName><value>TH-26</value></geocode><polygon>14.00,100.90 14.00,101.10 14.20,101.10 14.20,100.90 14.00,100.90</polygon></area>`
	responses := map[string][]byte{
		rssPath:                   []byte(rss),
		"/uploads/CAP/base.xml":   []byte(fmt.Sprintf(base, "TMD-BASE-1", "Alert", "synthetic@example.invalid,TMD-BASE-1,2026-10-03T00:00:00+07:00", "base", area)),
		"/uploads/CAP/update.xml": []byte(fmt.Sprintf(base, "TMD-UPDATE-1", "Update", "synthetic@example.invalid,TMD-BASE-1,2026-10-03T00:00:00+07:00", "update", "")),
		"/uploads/CAP/cancel.xml": []byte(cancel),
	}
	provider, err := NewWithFetcher(syntheticFetcher{responses: responses})
	if err != nil {
		t.Fatal(err)
	}
	provider.now = func() time.Time { return time.Date(2026, 10, 4, 12, 0, 0, 0, time.FixedZone("Asia/Bangkok", 7*60*60)) }
	batch, err := provider.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(batch.Records) != 3 || len(batch.Raw) != 4 {
		t.Fatalf("records/raw = %d/%d", len(batch.Records), len(batch.Raw))
	}
	for _, record := range batch.Records {
		if len(record.Geometry) == 0 || len(record.AlertReferences) == 0 || record.IssuedAt == nil {
			t.Fatalf("lifecycle record = %+v", record)
		}
	}
	if batch.Records[1].AlertMessageType != "Update" || batch.Records[2].AlertMessageType != "Cancel" || batch.Records[2].AlertReferences[0] != "TMD-UPDATE-1" {
		t.Fatalf("lifecycle records = %+v", batch.Records)
	}
	if batch.Records[2].ValidTo != nil {
		t.Fatalf("cancel ValidTo = %v, want nil when expiry is omitted", batch.Records[2].ValidTo)
	}
}

func TestFetchCAPKeepsStandaloneSyntheticCancelWithoutGeography(t *testing.T) {
	const rss = `<rss><channel><item><link>/uploads/CAP/standalone-cancel.xml</link></item></channel></rss>`
	const cancel = `<alert xmlns="urn:oasis:names:tc:emergency:cap:1.2"><identifier>TMD-STANDALONE-CANCEL</identifier><sender>synthetic@example.invalid</sender><sent>2026-10-03T00:00:00+07:00</sent><status>Actual</status><msgType>Cancel</msgType><references>synthetic@example.invalid,TMD-RETAINED-1,2026-10-02T00:00:00+07:00</references></alert>`
	provider, err := NewWithFetcher(syntheticFetcher{responses: map[string][]byte{
		rssPath:                              []byte(rss),
		"/uploads/CAP/standalone-cancel.xml": []byte(cancel),
	}})
	if err != nil {
		t.Fatal(err)
	}
	provider.now = func() time.Time { return time.Date(2026, 10, 4, 12, 0, 0, 0, time.FixedZone("Asia/Bangkok", 7*60*60)) }
	batch, err := provider.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(batch.Records) != 1 {
		t.Fatalf("records = %+v, want standalone cancel", batch.Records)
	}
	record := batch.Records[0]
	if record.AlertMessageType != "Cancel" || record.AlertIdentifier != "TMD-STANDALONE-CANCEL" || record.IssuedAt == nil || len(record.AlertReferences) != 1 || record.Geometry != nil || len(record.AreaCodes) != 0 {
		t.Fatalf("standalone cancel = %+v", record)
	}
}

func TestFetchCAPRejectsUnsafeLinksMalformedXMLAndTooManyItems(t *testing.T) {
	for name, rss := range map[string]string{
		"unsafe link":    `<rss><channel><item><link>https://169.254.169.254/uploads/CAP/x.xml</link></item></channel></rss>`,
		"malformed xml":  `<rss><channel><item><link>/uploads/CAP/x.xml</link></item></channel>`,
		"too many items": `<rss><channel>` + strings.Repeat(`<item><link>/uploads/CAP/x.xml</link></item>`, 11) + `</channel></rss>`,
	} {
		provider, err := NewWithFetcher(syntheticFetcher{responses: map[string][]byte{rssPath: []byte(rss)}})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := provider.Fetch(context.Background()); err == nil {
			t.Fatalf("accepted %s", name)
		}
	}

	provider, err := NewWithFetcher(syntheticFetcher{responses: map[string][]byte{
		rssPath:              []byte(`<rss><channel><item><link>/uploads/CAP/x.xml</link></item></channel></rss>`),
		"/uploads/CAP/x.xml": []byte(`<!DOCTYPE alert [<!ENTITY xxe SYSTEM "file:///secret">]><alert></alert>`),
	}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Fetch(context.Background()); err == nil {
		t.Fatal("accepted DTD/entity payload")
	}

	provider, err = NewWithFetcher(syntheticFetcher{responses: map[string][]byte{
		rssPath:              []byte(`<rss><channel><item><link>/uploads/CAP/x.xml</link></item></channel></rss>`),
		"/uploads/CAP/x.xml": []byte(`<alert><identifier>wrong-namespace</identifier></alert>`),
	}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Fetch(context.Background()); err == nil {
		t.Fatal("accepted CAP without the CAP 1.2 namespace")
	}
}

func TestFetchCAPReturnsFetcherError(t *testing.T) {
	provider, err := NewWithFetcher(syntheticFetcher{err: errors.New("synthetic upstream failure")})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Fetch(context.Background()); err == nil {
		t.Fatal("fetcher error was hidden")
	}
}

func TestCAPNewestFirstAndStandaloneUpdatePreserveLifecycle(t *testing.T) {
	const initial = `<alert xmlns="urn:oasis:names:tc:emergency:cap:1.2"><identifier>SYNTH-OLD</identifier><sender>test@example.invalid</sender><sent>2026-10-03T00:00:00Z</sent><status>Actual</status><msgType>Alert</msgType><info><expires>2026-10-05T00:00:00Z</expires><headline>Test only</headline><area><areaDesc>Test</areaDesc><geocode><valueName>ISO</valueName><value>TH-26</value></geocode></area></info></alert>`
	const update = `<alert xmlns="urn:oasis:names:tc:emergency:cap:1.2"><identifier>SYNTH-UPDATE</identifier><sender>test@example.invalid</sender><sent>2026-10-04T00:00:00Z</sent><status>Actual</status><msgType>Update</msgType><references>test@example.invalid,SYNTH-OLD,2026-10-03T00:00:00Z</references><info><expires>2026-10-05T00:00:00Z</expires><headline>Test update only</headline></info></alert>`
	for _, scenario := range []string{"newest-first", "standalone", "outside-target"} {
		includeInitial := scenario != "standalone"
		updateDocument := update
		if scenario == "outside-target" {
			updateDocument = strings.Replace(update, "</info>", `<area><areaDesc>Other province</areaDesc><geocode><valueName>ISO</valueName><value>TH-25</value></geocode></area></info>`, 1)
		}
		rss := `<rss><channel><item><link>/uploads/CAP/update.xml</link></item>`
		if includeInitial {
			rss += `<item><link>/uploads/CAP/initial.xml</link></item>`
		}
		rss += `</channel></rss>`
		provider, err := NewWithFetcher(syntheticFetcher{responses: map[string][]byte{rssPath: []byte(rss), "/uploads/CAP/update.xml": []byte(updateDocument), "/uploads/CAP/initial.xml": []byte(initial)}})
		if err != nil {
			t.Fatal(err)
		}
		provider.now = func() time.Time { return time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC) }
		batch, err := provider.Fetch(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if len(batch.Records) == 0 || batch.Records[0].AlertMessageType != "Update" || batch.Records[0].AlertReferences[0] != "SYNTH-OLD" {
			t.Fatalf("update lost: %+v", batch.Records)
		}
		if scenario == "newest-first" && len(batch.Records[0].AreaCodes) == 0 {
			t.Fatal("newest-first update lost inherited geography")
		}
		if scenario != "newest-first" && (batch.Records[0].Reason != "GEOGRAPHIC_SCOPE_UNCONFIRMED" || batch.Records[0].Freshness != "UNKNOWN" || len(batch.Records[0].AreaCodes) != 0) {
			t.Fatal("standalone update invented applicability")
		}
	}
}
