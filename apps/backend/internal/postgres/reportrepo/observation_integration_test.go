package reportrepo

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/report"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestObservationMetadataPersistsAndOmittedEditsPreserveEvidenceTime(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL for report observation integration")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	repository := New(pool)
	now := time.Now().UTC().Truncate(time.Microsecond)
	observed := now.Add(-90 * time.Minute)
	depth := 0.75
	input := report.Report{ID: uuid.NewString(), DisplayName: "Synthetic observation", Type: report.TypeFlood, Description: "Synthetic observation only", Location: report.Location{Type: "Point", Coordinates: []float64{101.01, 14.03}}, LocationName: "Synthetic area", ObservedAt: &observed, WaterTrend: report.TrendFalling, EstimatedWaterDepthM: &depth, Status: report.StatusPending, CreatedAt: now, UpdatedAt: now}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM community_reports WHERE id=$1`, input.ID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM event_outbox WHERE resource_type='COMMUNITY_REPORT' AND resource_id=$1`, input.ID)
	})
	tokenHash := bytes.Repeat([]byte{0x3a}, 32)
	created, err := repository.CreateGuest(ctx, input, tokenHash, now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if created.ObservedAt == nil || !created.ObservedAt.Equal(observed) || created.WaterTrend != report.TrendFalling {
		t.Fatalf("observation metadata lost: %+v", created)
	}
	updated, err := repository.UpdatePending(ctx, input.ID, report.Input{Type: report.TypeFlood, Description: "Synthetic edited text", Location: input.Location, EstimatedWaterDepthM: &depth})
	if err != nil {
		t.Fatal(err)
	}
	if updated.ObservedAt == nil || !updated.ObservedAt.Equal(observed) || updated.WaterTrend != report.TrendFalling || !updated.UpdatedAt.After(created.UpdatedAt) {
		t.Fatalf("text edit refreshed or lost evidence metadata: %+v", updated)
	}
	newTime := now.Add(-30 * time.Minute)
	updated, err = repository.UpdatePending(ctx, input.ID, report.Input{Type: report.TypeFlood, Description: "Synthetic corrected observation", Location: input.Location, ObservedAt: &newTime, WaterTrend: report.TrendStable})
	if err != nil {
		t.Fatal(err)
	}
	if updated.ObservedAt == nil || !updated.ObservedAt.Equal(newTime) || updated.WaterTrend != report.TrendStable {
		t.Fatalf("explicit observation correction lost: %+v", updated)
	}
	mediaID := uuid.NewString()
	media := report.Media{ID: mediaID, ReportID: input.ID, ObjectKey: "reports/" + input.ID + "/" + mediaID + ".png", ContentType: "image/png", SizeBytes: 1, ContentSHA256: strings.Repeat("a", 64)}
	if _, err := repository.CreateGuestMedia(ctx, media, tokenHash, now); err != nil {
		t.Fatal(err)
	}
	stored, err := repository.GetMedia(ctx, input.ID, mediaID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.ContentSHA256 != media.ContentSHA256 {
		t.Fatal("media content hash not preserved")
	}
	current, err := repository.Get(ctx, input.ID)
	if err != nil {
		t.Fatal(err)
	}
	if current.ObservedAt == nil || !current.ObservedAt.Equal(newTime) || len(current.Media) != 1 || current.Media[0].ContentSHA256 != media.ContentSHA256 {
		t.Fatalf("media update refreshed observation or lost hash: %+v", current)
	}
}
