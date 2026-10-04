package report

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCreatePreservesObservationTimeAndTrendWithoutRefreshingEvidence(t *testing.T) {
	now := time.Date(2026, 10, 5, 8, 0, 0, 0, time.UTC)
	observed := now.Add(-2 * time.Hour)
	repository := &fakeRepository{}
	service, err := NewService(repository, &fakeStorage{})
	if err != nil {
		t.Fatal(err)
	}
	service.now = func() time.Time { return now }
	_, err = service.Create(context.Background(), "ddfd74fd-9561-4c5e-a304-b1cb704508d9", Input{
		Type: TypeFlood, Description: "Synthetic observation", Location: Location{Type: "Point", Coordinates: []float64{101, 14}},
		ObservedAt: &observed, WaterTrend: TrendFalling,
	})
	if err != nil {
		t.Fatal(err)
	}
	if repository.created.ObservedAt == nil || !repository.created.ObservedAt.Equal(observed) || repository.created.WaterTrend != TrendFalling {
		t.Fatalf("observation was replaced by submission time: %+v", repository.created)
	}
}

func TestCreateRejectsInvalidObservationMetadata(t *testing.T) {
	now := time.Date(2026, 10, 5, 8, 0, 0, 0, time.UTC)
	future := now.Add(time.Hour)
	zero := time.Time{}
	for _, test := range []struct {
		name  string
		time  *time.Time
		trend WaterTrend
	}{
		{name: "future", time: &future},
		{name: "zero", time: &zero},
		{name: "invalid trend", trend: "SAFE"},
	} {
		t.Run(test.name, func(t *testing.T) {
			service, err := NewService(&fakeRepository{}, &fakeStorage{})
			if err != nil {
				t.Fatal(err)
			}
			service.now = func() time.Time { return now }
			_, err = service.Create(context.Background(), "ddfd74fd-9561-4c5e-a304-b1cb704508d9", Input{
				Type: TypeFlood, Description: "Synthetic observation", Location: Location{Type: "Point", Coordinates: []float64{101, 14}},
				ObservedAt: test.time, WaterTrend: test.trend,
			})
			if !errors.Is(err, ErrInvalid) {
				t.Fatalf("error = %v, want ErrInvalid", err)
			}
		})
	}
}

func TestLegacyCreateDoesNotInventObservationTime(t *testing.T) {
	repository := &fakeRepository{}
	service, err := NewService(repository, &fakeStorage{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.Create(context.Background(), "ddfd74fd-9561-4c5e-a304-b1cb704508d9", Input{
		Type: TypeFlood, Description: "Synthetic legacy observation", Location: Location{Type: "Point", Coordinates: []float64{101, 14}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if repository.created.ObservedAt != nil || repository.created.WaterTrend != TrendUnknown {
		t.Fatalf("legacy evidence must remain unknown: %+v", repository.created)
	}
}
