package externaldatarepo

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/announcement"
	"github.com/floodwatch-ongkharak/backend/internal/externaldata"
	"github.com/floodwatch-ongkharak/backend/internal/externaldata/tmd"
)

type announcementCAPFetcher map[string][]byte

type announcementObjects struct{ count int }

func (objects *announcementObjects) Put(_ context.Context, _, _ string, _ []byte) error {
	objects.count++
	return nil
}

func (fetcher announcementCAPFetcher) Fetch(_ context.Context, path string) ([]byte, error) {
	body, exists := fetcher[path]
	if !exists {
		return nil, fmt.Errorf("unexpected synthetic path")
	}
	return body, nil
}

func TestDisjointCAPWindowsPersistAsUnknownInsteadOfFailingPoll(t *testing.T) {
	ctx, store, pool := integrationStore(t)
	const code = "TMD_CAP"
	var existing int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM external_data_records WHERE source_code=$1`, code).Scan(&existing); err != nil || existing != 0 {
		t.Fatalf("isolated database required: records=%d err=%v", existing, err)
	}
	t.Cleanup(func() { cleanupExternalSource(t, ctx, pool, code) })
	now := time.Now().UTC().Truncate(time.Microsecond)
	format := func(offset time.Duration) string { return now.Add(offset).Format(time.RFC3339Nano) }
	area := `<area><areaDesc>Synthetic local</areaDesc><geocode><valueName>ISO</valueName><value>TH-26</value></geocode></area>`
	document := fmt.Sprintf(`<alert xmlns="urn:oasis:names:tc:emergency:cap:1.2"><identifier>SYNTH-DISJOINT</identifier><sender>test@example.invalid</sender><sent>%s</sent><status>Actual</status><msgType>Alert</msgType><info><effective>%s</effective><expires>%s</expires>%s</info><info><effective>%s</effective><expires>%s</expires>%s</info></alert>`, format(-3*time.Hour), format(-2*time.Hour), format(time.Hour), area, format(2*time.Hour), format(3*time.Hour), area)
	provider, err := tmd.NewWithFetcher(announcementCAPFetcher{"/api/xml/CAP": []byte(`<rss><channel><item><link>/uploads/CAP/synthetic.xml</link></item></channel></rss>`), "/uploads/CAP/synthetic.xml": []byte(document)})
	if err != nil {
		t.Fatal(err)
	}
	objects := &announcementObjects{}
	poller, err := externaldata.NewService(store, objects, map[string]externaldata.Provider{code: provider})
	if err != nil {
		t.Fatal(err)
	}
	if err := poller.Poll(ctx); err != nil {
		t.Fatal(err)
	}
	if objects.count != 2 {
		t.Fatalf("raw archive calls=%d", objects.count)
	}
	service := announcement.NewService(store, true)
	service.Clock = func() time.Time { return time.Now().UTC().Add(time.Second) }
	result, err := service.List(ctx, 20, 0)
	if err != nil || len(result.Items) != 1 || result.Items[0].EffectiveStatus != "UNKNOWN" || result.Items[0].Reason == nil || *result.Items[0].Reason != "CONFLICTING_TIME_WINDOWS" {
		t.Fatalf("disjoint evidence became current: %+v err=%v", result, err)
	}
}

func TestAnnouncementEvidenceReadBoundRejectsOverflow(t *testing.T) {
	ctx, store, pool := integrationStore(t)
	const code = "TMD_CAP"
	var existing int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM external_data_records WHERE source_code=$1`, code).Scan(&existing); err != nil || existing != 0 {
		t.Fatalf("isolated database required: records=%d err=%v", existing, err)
	}
	t.Cleanup(func() { cleanupExternalSource(t, ctx, pool, code) })
	if _, err := pool.Exec(ctx, `INSERT INTO external_data_sources(code,name,kind,provenance) VALUES ('TMD_CAP','Synthetic test','OFFICIAL_ALERT','OFFICIAL') ON CONFLICT (code) DO UPDATE SET enabled=true`); err != nil {
		t.Fatal(err)
	}
	record := capRecord("SYNTH-BOUND", "SYNTH-BOUND", "Alert", timePointer(time.Now().UTC().Add(-time.Hour)), nil)
	payload, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	insert := `INSERT INTO external_data_records(source_code,record_key,content_hash,kind,provenance,payload) SELECT 'TMD_CAP','SYNTH-BOUND-'||entry,repeat('a',64),'OFFICIAL_ALERT','OFFICIAL',$1::jsonb FROM generate_series($2::integer,$3::integer) entry`
	if _, err := pool.Exec(ctx, insert, payload, 1, 10000); err != nil {
		t.Fatal(err)
	}
	snapshot, err := store.Evidence(ctx)
	if err != nil || len(snapshot.Records) != 10000 {
		t.Fatalf("exact bound: count=%d err=%v", len(snapshot.Records), err)
	}
	if _, err := pool.Exec(ctx, insert, payload, 10001, 10001); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Evidence(ctx); err == nil {
		t.Fatal("overflow silently truncated retained lifecycle")
	}
}

func TestAnnouncementLifecycleSurvivesEmptyFeedWithoutSendingNotifications(t *testing.T) {
	ctx, store, pool := integrationStore(t)
	const code = "TMD_CAP"
	var existing int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM external_data_records WHERE source_code=$1`, code).Scan(&existing); err != nil {
		t.Fatal(err)
	}
	if existing != 0 {
		t.Fatal("announcement integration requires an isolated database")
	}
	t.Cleanup(func() { cleanupExternalSource(t, ctx, pool, code) })
	started := time.Now().UTC().Add(-time.Hour).Truncate(time.Microsecond)
	initial := capRecord("SYNTH-START:Alert", "SYNTH-START", "Alert", timePointer(started.Add(-time.Minute)), nil)
	initial.ValidTo = timePointer(started.Add(24 * time.Hour))
	initial.AreaCodes = []string{"TH-26"}
	initial.Headline = "Synthetic initial"
	initial.Description = "Synthetic only"
	update := initial
	update.Key = "SYNTH-UPDATE:Update"
	update.AlertIdentifier = "SYNTH-UPDATE"
	update.AlertMessageType = "Update"
	update.AlertReferences = []string{"SYNTH-START"}
	update.IssuedAt = timePointer(started.Add(10 * time.Minute))
	update.Headline = "Synthetic updated"
	cancel := capRecord("SYNTH-CANCEL:Cancel", "SYNTH-CANCEL", "Cancel", timePointer(started.Add(20*time.Minute)), []string{"SYNTH-UPDATE"})
	var notificationsBefore int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM alerts`).Scan(&notificationsBefore); err != nil {
		t.Fatal(err)
	}
	service := announcement.NewService(store, true)
	var stableID string
	for attempt, records := range [][]externaldata.Record{{initial}, {initial}, {update}, {cancel}, {}} {
		if err := completeBatch(t, ctx, store, code, started.Add(time.Duration(attempt)*10*time.Minute), records); err != nil {
			t.Fatal(err)
		}
		service.Clock = func() time.Time { return started.Add(time.Duration(attempt)*10*time.Minute + 2*time.Minute) }
		result, err := service.List(ctx, 20, 0)
		if err != nil || len(result.Items) != 1 {
			t.Fatalf("attempt%d=%+v err=%v", attempt, result, err)
		}
		if attempt == 0 {
			stableID = result.Items[0].ID
		}
		if result.Items[0].ID != stableID {
			t.Fatal("announcement identifier changed across lifecycle")
		}
		if attempt >= 3 && (result.Items[0].Status != "CANCELLED" || result.Items[0].HistoryTotal != 3 || result.Items[0].Headline != "Synthetic updated") {
			t.Fatal("empty feed lost cancellation/history")
		}
	}
	var after, createdEvents int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM alerts`).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM event_outbox WHERE event_type='alert.created' AND resource_id=$1`, stableID).Scan(&createdEvents); err != nil {
		t.Fatal(err)
	}
	if after != notificationsBefore || createdEvents != 0 {
		t.Fatal("official read model triggered app-generated alert delivery")
	}
}

func TestReappearingUnconfirmedCAPVersionDoesNotKeepConfirmedBadge(t *testing.T) {
	ctx, store, _ := integrationStore(t)
	const code = "TMD_CAP"
	t.Cleanup(func() { cleanupExternalSource(t, ctx, store.pool, code) })
	started := time.Now().UTC().Add(-time.Hour).Truncate(time.Microsecond)
	initial := capRecord("SYNTH-ROOT:Alert", "SYNTH-ROOT", "Alert", timePointer(started.Add(-time.Minute)), nil)
	initial.ValidTo = timePointer(started.Add(24 * time.Hour))
	initial.AreaCodes = []string{"TH-26"}
	unknown := capRecord("SYNTH-UPDATE:Update", "SYNTH-UPDATE", "Update", timePointer(started.Add(time.Minute)), []string{"SYNTH-ROOT"})
	unknown.ValidTo = initial.ValidTo
	unknown.Reason = "GEOGRAPHIC_SCOPE_UNCONFIRMED"
	confirmed := unknown
	confirmed.AreaCodes = []string{"TH-26"}
	confirmed.Reason = ""
	service := announcement.NewService(store, true)
	for attempt, record := range []externaldata.Record{initial, unknown, confirmed, unknown} {
		if err := completeBatch(t, ctx, store, code, started.Add(time.Duration(attempt)*10*time.Minute), []externaldata.Record{record}); err != nil {
			t.Fatal(err)
		}
		service.Clock = func() time.Time { return started.Add(time.Duration(attempt)*10*time.Minute + 2*time.Minute) }
		result, err := service.List(ctx, 20, 0)
		if err != nil || len(result.Items) != 1 {
			t.Fatalf("attempt=%d result=%+v err=%v", attempt, result, err)
		}
		if attempt == 2 && result.Items[0].EffectiveStatus != "UPDATED" {
			t.Fatal("confirmed revision did not become current")
		}
		if attempt == 3 && result.Items[0].EffectiveStatus != "UNKNOWN" {
			t.Fatal("reappearing unconfirmed revision retained current badge")
		}
	}
}
