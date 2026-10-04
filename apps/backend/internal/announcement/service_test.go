package announcement

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/externaldata"
	"github.com/google/uuid"
)

type fakeStore struct {
	data  EvidenceSnapshot
	err   error
	calls int
}

func (store *fakeStore) Evidence(context.Context) (EvidenceSnapshot, error) {
	store.calls++
	return store.data, store.err
}

func capEvidence(identifier, messageType string, issued time.Time, references ...string) Evidence {
	expires := issued.Add(48 * time.Hour)
	return Evidence{ID: uuid.NewString(), RetrievedAt: issued.Add(time.Minute), Record: externaldata.Record{
		Key: identifier + ":" + messageType, Kind: externaldata.OfficialAlert, Provenance: "OFFICIAL",
		AlertIdentifier: identifier, AlertMessageType: messageType, AlertReferences: references,
		IssuedAt: &issued, ValidTo: &expires, Headline: "Synthetic " + identifier, Description: "Synthetic test only", AreaCodes: []string{"TH-26"},
	}}
}

func newTestService(now time.Time, records ...Evidence) (*Service, *fakeStore) {
	store := &fakeStore{data: EvidenceSnapshot{Enabled: true, LastSuccessAt: &now, Records: records}}
	service := NewService(store, true)
	service.Clock = func() time.Time { return now }
	return service, store
}

func TestCAPLifecycleKeepsStableAnnouncementAndHistory(t *testing.T) {
	now := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	initial := capEvidence("SYNTH-INITIAL", "Alert", now.Add(-3*time.Hour))
	update := capEvidence("SYNTH-UPDATE", "Update", now.Add(-2*time.Hour), "SYNTH-INITIAL")
	cancel := capEvidence("SYNTH-CANCEL", "Cancel", now.Add(-time.Hour), "SYNTH-UPDATE")
	cancel.Record.ValidTo, cancel.Record.Headline, cancel.Record.Description, cancel.Record.AreaCodes = nil, "", "", nil
	service, store := newTestService(now, initial)
	first, err := service.List(context.Background(), 20, 0)
	if err != nil || len(first.Items) != 1 || first.Items[0].Status != "ACTIVE" {
		t.Fatalf("initial=%+v err=%v", first, err)
	}
	store.data.Records = []Evidence{update, initial, update}
	second, err := service.List(context.Background(), 20, 0)
	if err != nil || len(second.Items) != 1 || second.Items[0].ID != first.Items[0].ID || second.Items[0].Status != "UPDATED" || second.Items[0].HistoryTotal != 2 {
		t.Fatalf("update=%+v err=%v", second, err)
	}
	store.data.Records = []Evidence{cancel, update, initial}
	third, err := service.List(context.Background(), 20, 0)
	if err != nil || len(third.Items) != 1 || third.Items[0].ID != first.Items[0].ID || third.Items[0].Status != "CANCELLED" || third.Items[0].Headline != update.Record.Headline || third.Items[0].HistoryTotal != 3 {
		t.Fatalf("cancel=%+v err=%v", third, err)
	}
	if third.Items[0].History[0].MessageType != "Cancel" || third.Items[0].History[2].MessageType != "Alert" {
		t.Fatal("history is not newest first")
	}
}

func TestUnconfirmedUpdateAndStaleSourceNeverBecomeCurrentWarnings(t *testing.T) {
	now := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	initial := capEvidence("SYNTH-INITIAL", "Alert", now.Add(-2*time.Hour))
	update := capEvidence("SYNTH-UPDATE", "Update", now.Add(-time.Hour), "SYNTH-INITIAL")
	update.Record.AreaCodes, update.Record.Reason = nil, "GEOGRAPHIC_SCOPE_UNCONFIRMED"
	service, store := newTestService(now, initial, update)
	result, err := service.List(context.Background(), 20, 0)
	if err != nil || result.Items[0].EffectiveStatus != "UNKNOWN" || result.Items[0].Reason == nil {
		t.Fatalf("unconfirmed=%+v err=%v", result, err)
	}
	store.data.Records = []Evidence{initial}
	old := now.Add(-2 * time.Hour)
	store.data.LastSuccessAt = &old
	result, err = service.List(context.Background(), 20, 0)
	if err != nil || result.Items[0].EffectiveStatus != "UNKNOWN" || result.Items[0].Freshness != "UNKNOWN" {
		t.Fatalf("stale source=%+v err=%v", result, err)
	}
}

func TestExpiredOrCancelledAnnouncementsAreNotReactivatedByOldEvidence(t *testing.T) {
	now := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	initial := capEvidence("SYNTH-INITIAL", "Alert", now.Add(-72*time.Hour))
	service, _ := newTestService(now, initial)
	result, err := service.List(context.Background(), 20, 0)
	if err != nil || result.Items[0].Status != "EXPIRED" || result.Items[0].EffectiveStatus != "EXPIRED" {
		t.Fatalf("expired=%+v err=%v", result, err)
	}
}

func TestDisabledUnavailableAndBoundedPagination(t *testing.T) {
	store := &fakeStore{err: errors.New("private database failure")}
	service := NewService(store, false)
	result, err := service.List(context.Background(), 20, 0)
	if err != nil || store.calls != 0 || result.Source.Status != "NOT_CONFIGURED" || result.Items == nil {
		t.Fatalf("disabled=%+v err=%v", result, err)
	}
	service = NewService(store, true)
	if _, err := service.List(context.Background(), 20, 0); err == nil {
		t.Fatal("database outage became an empty successful list")
	}
	for _, page := range [][2]int{{0, 0}, {51, 0}, {20, -1}, {20, 10001}} {
		if _, err := service.List(context.Background(), page[0], page[1]); !errors.Is(err, ErrInvalid) {
			t.Fatal("unbounded pagination accepted")
		}
	}
}

func TestAnnouncementHistoryAndPaginationAreBounded(t *testing.T) {
	now := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	initial := capEvidence("SYNTH-ROOT", "Alert", now.Add(-30*time.Minute))
	evidence := []Evidence{initial}
	for index := 0; index < 24; index++ {
		revision := capEvidence("SYNTH-ROOT", "Alert", initial.Record.IssuedAt.UTC())
		revision.RetrievedAt = now.Add(time.Duration(index-24) * time.Minute)
		evidence = append(evidence, revision)
	}
	evidence = append(evidence, capEvidence("SYNTH-SECOND", "Alert", now.Add(-time.Minute)))
	service, _ := newTestService(now, evidence...)
	result, err := service.List(context.Background(), 1, 0)
	if err != nil || result.Total != 2 || len(result.Items) != 1 || result.Items[0].Identifier != "SYNTH-SECOND" {
		t.Fatalf("page=%+v err=%v", result, err)
	}
	result, err = service.List(context.Background(), 1, 1)
	if err != nil || result.Items[0].HistoryTotal != 25 || len(result.Items[0].History) != 20 || result.Items[0].ID != initial.ID {
		t.Fatalf("history=%+v err=%v", result, err)
	}
	result, err = service.List(context.Background(), 1, 2)
	if err != nil || len(result.Items) != 0 || result.Items == nil {
		t.Fatal("past end pagination is not an empty bounded page")
	}
}

func TestAnnouncementNotYetEffectiveIsUnknown(t *testing.T) {
	now := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	initial := capEvidence("SYNTH-FUTURE-EFFECTIVE", "Alert", now.Add(-time.Hour))
	effective := now.Add(time.Hour)
	initial.Record.ValidFrom = &effective
	service, _ := newTestService(now, initial)
	result, err := service.List(context.Background(), 20, 0)
	if err != nil || result.Items[0].EffectiveStatus != "UNKNOWN" || result.Items[0].Reason == nil || *result.Items[0].Reason != "NOT_YET_EFFECTIVE" {
		t.Fatal("future effective announcement displayed current")
	}
}

func TestCancellationSharedMissingReferenceSuppressesConfirmedUpdate(t *testing.T) {
	now := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	update := capEvidence("SYNTH-UPDATE", "Update", now.Add(-time.Hour), "SYNTH-MISSING")
	cancel := capEvidence("SYNTH-CANCEL", "Cancel", now.Add(-time.Minute), "SYNTH-MISSING")
	cancel.Record.AreaCodes, cancel.Record.ValidTo = nil, nil
	service, _ := newTestService(now, update, cancel)
	result, err := service.List(context.Background(), 20, 0)
	if err != nil || len(result.Items) != 1 || result.Items[0].Status != "CANCELLED" || result.Items[0].HistoryTotal != 2 {
		t.Fatalf("shared missing reference=%+v err=%v", result, err)
	}
}

func TestStandaloneUnmatchedLifecycleIsNotAnOngkharakAnnouncement(t *testing.T) {
	now := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	update := capEvidence("SYNTH-UNMATCHED", "Update", now.Add(-time.Minute), "UNKNOWN-ROOT")
	update.Record.AreaCodes = nil
	update.Record.Reason = "GEOGRAPHIC_SCOPE_UNCONFIRMED"
	service, _ := newTestService(now, update)
	result, err := service.List(context.Background(), 20, 0)
	if err != nil || len(result.Items) != 0 {
		t.Fatal("standalone unmatched update became a local announcement")
	}
}

func TestLatestRetrievalSelectsVariantWithoutChangingIdentityOrRevivingCancellation(t *testing.T) {
	now := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	initial := capEvidence("SYNTH-ROOT", "Alert", now.Add(-3*time.Hour))
	unconfirmed := capEvidence("SYNTH-UPDATE", "Update", now.Add(-2*time.Hour), "SYNTH-ROOT")
	unconfirmed.Record.Reason = "GEOGRAPHIC_SCOPE_UNCONFIRMED"
	unconfirmed.Record.AreaCodes = nil
	confirmed := capEvidence("SYNTH-UPDATE", "Update", *unconfirmed.Record.IssuedAt, "SYNTH-ROOT")
	confirmed.RetrievedAt = unconfirmed.RetrievedAt.Add(time.Minute)
	unconfirmed.LastRetrievedAt = now.Add(-time.Minute)
	service, store := newTestService(now, initial, unconfirmed, confirmed)
	result, err := service.List(context.Background(), 20, 0)
	if err != nil || len(result.Items) != 1 || result.Items[0].ID != initial.ID || result.Items[0].EffectiveStatus != "UNKNOWN" || result.Items[0].HistoryTotal != 3 {
		t.Fatalf("reappearing variant=%+v err=%v", result, err)
	}
	cancel := capEvidence("SYNTH-CANCEL", "Cancel", now.Add(-time.Hour), "SYNTH-ROOT")
	initial.LastRetrievedAt = now
	store.data.Records = append(store.data.Records, cancel, initial)
	result, err = service.List(context.Background(), 20, 0)
	if err != nil || result.Items[0].Status != "CANCELLED" || result.Items[0].ID != initial.ID {
		t.Fatalf("old retrieval revived cancellation=%+v err=%v", result, err)
	}
}

func TestRecentProviderFailureMakesAnnouncementUnknown(t *testing.T) {
	now := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	service, store := newTestService(now, capEvidence("SYNTH-ROOT", "Alert", now.Add(-time.Hour)))
	code := "UPSTREAM_UNAVAILABLE"
	store.data.ErrorCode = &code
	result, err := service.List(context.Background(), 20, 0)
	if err != nil || result.Source.Status != "DEGRADED" || result.Items[0].EffectiveStatus != "UNKNOWN" || result.Items[0].Reason == nil || *result.Items[0].Reason != "SOURCE_UNAVAILABLE" {
		t.Fatalf("provider failure=%+v err=%v", result, err)
	}
}

func TestEqualIssuedCancellationCannotBeRevivedByUpdateRepoll(t *testing.T) {
	now := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	update := capEvidence("SYNTH-UPDATE", "Update", now.Add(-time.Hour), "SYNTH-ROOT")
	cancel := capEvidence("SYNTH-CANCEL", "Cancel", *update.Record.IssuedAt, "SYNTH-UPDATE")
	cancel.RetrievedAt = update.RetrievedAt.Add(time.Minute)
	update.LastRetrievedAt = now
	service, _ := newTestService(now, update, cancel)
	result, err := service.List(context.Background(), 20, 0)
	if err != nil || len(result.Items) != 1 || result.Items[0].Status != "CANCELLED" {
		t.Fatalf("equal-issued cancellation revived: %+v err=%v", result, err)
	}
}

func TestEqualIssuedSupersedingUpdateWinsDespiteReverseArrival(t *testing.T) {
	now := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	prior := capEvidence("SYNTH-PRIOR", "Update", now.Add(-time.Hour), "SYNTH-ROOT")
	latest := capEvidence("SYNTH-LATEST", "Update", *prior.Record.IssuedAt, "SYNTH-PRIOR")
	latest.RetrievedAt = prior.RetrievedAt.Add(-time.Minute)
	latest.Record.Reason = "GEOGRAPHIC_SCOPE_UNCONFIRMED"
	latest.Record.AreaCodes = nil
	service, _ := newTestService(now, latest, prior)
	result, err := service.List(context.Background(), 20, 0)
	if err != nil || len(result.Items) != 1 || result.Items[0].Identifier != "SYNTH-LATEST" || result.Items[0].EffectiveStatus != "UNKNOWN" {
		t.Fatalf("ancestor superseded equal-time update: %+v err=%v", result, err)
	}
}

func TestEqualIssuedCompetingUpdatesFailClosed(t *testing.T) {
	now := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	first := capEvidence("SYNTH-FIRST", "Update", now.Add(-time.Hour), "SYNTH-ROOT")
	second := capEvidence("SYNTH-SECOND", "Update", *first.Record.IssuedAt, "SYNTH-ROOT")
	service, _ := newTestService(now, first, second)
	result, err := service.List(context.Background(), 20, 0)
	if err != nil || len(result.Items) != 1 || result.Items[0].EffectiveStatus != "UNKNOWN" || result.Items[0].Reason == nil || *result.Items[0].Reason != "CONFLICTING_LIFECYCLE" {
		t.Fatalf("ambiguous equal-time updates became current: %+v err=%v", result, err)
	}
}

func TestAmbiguousExpiredUpdatesRemainUnknown(t *testing.T) {
	now := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	first := capEvidence("SYNTH-FIRST", "Update", now.Add(-2*time.Hour), "SYNTH-ROOT")
	second := capEvidence("SYNTH-SECOND", "Update", *first.Record.IssuedAt, "SYNTH-ROOT")
	first.ID, second.ID = "00000000-0000-0000-0000-000000000001", "ffffffff-ffff-ffff-ffff-ffffffffffff"
	expired := now.Add(-time.Hour)
	second.Record.ValidTo = &expired
	service, _ := newTestService(now, first, second)
	result, err := service.List(context.Background(), 20, 0)
	if err != nil || len(result.Items) != 1 || result.Items[0].EffectiveStatus != "UNKNOWN" || result.Items[0].Freshness != "UNKNOWN" {
		t.Fatalf("ambiguous expiry=%+v err=%v", result, err)
	}
}
