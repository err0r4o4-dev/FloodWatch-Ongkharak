package externaldatarepo

import (
	"strings"
	"testing"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/externaldata"
	"github.com/google/uuid"
)

func TestCAPRepeatedPollKeepsOneRecordAndRetainsChangedVersion(t *testing.T) {
	ctx, store, pool := integrationStore(t)
	code := "SYNTH_TMD_" + strings.ToUpper(uuid.NewString()[:8])
	t.Cleanup(func() { cleanupExternalSource(t, ctx, pool, code) })
	started := time.Now().UTC().Add(-30 * time.Minute).Truncate(time.Microsecond)
	issued, expiry := started.Add(-time.Hour), started.Add(24*time.Hour)
	record := externaldata.Record{
		Key: "SYNTH-TMD:Alert", Kind: externaldata.OfficialAlert, Provenance: "OFFICIAL",
		AlertIdentifier: "SYNTH-TMD", AlertMessageType: "Alert", IssuedAt: &issued, ValidTo: &expiry,
		Headline: "Synthetic original", AreaCodes: []string{"TH-26"}, Freshness: "UNKNOWN",
	}
	for attempt := 0; attempt < 3; attempt++ {
		if attempt == 2 {
			record.Headline = "Synthetic revised content"
		}
		if err := completeBatch(t, ctx, store, code, started.Add(time.Duration(attempt)*10*time.Minute), []externaldata.Record{record}); err != nil {
			t.Fatal(err)
		}
	}
	var records, batches, links, rawLinks, events int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM external_data_records WHERE source_code=$1`, code).Scan(&records); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM external_data_batches WHERE source_code=$1`, code).Scan(&batches); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM external_data_batch_records br JOIN external_data_batches b ON b.id=br.batch_id WHERE b.source_code=$1`, code).Scan(&links); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM external_data_record_raw_references rr JOIN external_data_records r ON r.id=rr.record_id WHERE r.source_code=$1`, code).Scan(&rawLinks); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM event_outbox WHERE event_type='external_data.updated' AND payload->>'sourceCode'=$1`, code).Scan(&events); err != nil {
		t.Fatal(err)
	}
	if records != 2 || batches != 3 || links != 3 || rawLinks != 3 || events != 3 {
		t.Fatalf("records=%d batches=%d links=%d rawLinks=%d invalidations=%d", records, batches, links, rawLinks, events)
	}
	snapshot, err := store.Snapshot(ctx, code, 24)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.RecordCount != 1 || len(snapshot.Records) != 1 || snapshot.Records[0].Headline != "Synthetic revised content" {
		t.Fatalf("latest snapshot=%+v", snapshot)
	}
}
