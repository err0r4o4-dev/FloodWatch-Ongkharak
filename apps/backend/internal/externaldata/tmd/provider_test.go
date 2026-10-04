package tmd

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

type syntheticFetcher struct {
	responses map[string][]byte
	err      error
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
	const cap = "<?xml version=\"1.0\"?><alert xmlns=\"urn:oasis:names:tc:emergency:cap:1.2\"><identifier>TMD-SYNTH-1</identifier><sender>synthetic@example.invalid</sender><sent>2026-10-03T16:53:56+07:00</sent><status>Actual</status><msgType>Alert</msgType><references>TMD-SYNTH-1</references><info><language>en-US</language><category>Met</category><event>Synthetic flood watch</event><urgency>Expected</urgency><severity>Moderate</severity><certainty>Likely</certainty><expires>2026-10-04T18:00:00+07:00</expires><headline>Synthetic alert for Ongkharak</headline><description>Explicitly synthetic CAP content.</description><area><areaDesc>Synthetic Ongkharak area</areaDesc><geocode><valueName>FIPS6</valueName><value>TH-26</value></geocode><polygon>14.00,100.90 14.00,101.10 14.20,101.10 14.20,100.90 14.00,100.90</polygon></area></info></alert>"
	provider, err := NewWithFetcher(syntheticFetcher{responses: map[string][]byte{
		rssPath:                            []byte(rss),
		"/uploads/CAP/CAPTMD20261003165356_2.xml": []byte(cap),
	}})
	if err != nil {
		t.Fatal(err)
	}
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
		rssPath:                       []byte(rss),
		"/uploads/CAP/expired.xml":   []byte(fmt.Sprintf(base, "expired", "Actual", "expired", "2026-10-03T01:00:00+07:00", "TH-26")),
		"/uploads/CAP/unmatched.xml": []byte(fmt.Sprintf(base, "unmatched", "Actual", "unmatched", "2026-10-05T01:00:00+07:00", "TH-25")),
		"/uploads/CAP/exercise.xml":  []byte(fmt.Sprintf(base, "exercise", "Test", "exercise", "2026-10-05T01:00:00+07:00", "TH-26")),
	}
	provider, err := NewWithFetcher(syntheticFetcher{responses: responses})
	if err != nil {
		t.Fatal(err)
	}
	batch, err := provider.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(batch.Records) != 0 || len(batch.Raw) != 4 {
		t.Fatalf("records/raw = %d/%d", len(batch.Records), len(batch.Raw))
	}
}

func TestFetchCAPRejectsUnsafeLinksMalformedXMLAndTooManyItems(t *testing.T) {
	for name, rss := range map[string]string{
		"unsafe link": `<rss><channel><item><link>https://169.254.169.254/uploads/CAP/x.xml</link></item></channel></rss>`,
		"malformed xml": `<rss><channel><item><link>/uploads/CAP/x.xml</link></item></channel>`,
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
		rssPath: []byte(`<rss><channel><item><link>/uploads/CAP/x.xml</link></item></channel></rss>`),
		"/uploads/CAP/x.xml": []byte(`<!DOCTYPE alert [<!ENTITY xxe SYSTEM "file:///secret">]><alert></alert>`),
	}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Fetch(context.Background()); err == nil {
		t.Fatal("accepted DTD/entity payload")
	}
}
