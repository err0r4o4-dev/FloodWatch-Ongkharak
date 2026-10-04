package ai_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/ai"
	"github.com/floodwatch-ongkharak/backend/internal/report"
)

type fakeEvidenceRepository struct {
	enqueueFn  func(context.Context, int) error
	claimFn    func(context.Context, int, time.Duration, int) ([]ai.AnalysisJob, error)
	completeFn func(context.Context, ai.AnalysisJob, ai.AnalyzeMediaResponse, time.Time) error
	failFn     func(context.Context, ai.AnalysisJob, string, time.Time, int) error
}

func (repository *fakeEvidenceRepository) EnqueuePendingMedia(ctx context.Context, limit int) error {
	return repository.enqueueFn(ctx, limit)
}

func (repository *fakeEvidenceRepository) ClaimAnalysisJobs(ctx context.Context, limit int, lease time.Duration, maxAttempts int) ([]ai.AnalysisJob, error) {
	return repository.claimFn(ctx, limit, lease, maxAttempts)
}

func (repository *fakeEvidenceRepository) CompleteAnalysis(ctx context.Context, job ai.AnalysisJob, result ai.AnalyzeMediaResponse, receivedAt time.Time) error {
	return repository.completeFn(ctx, job, result, receivedAt)
}

func (repository *fakeEvidenceRepository) FailAnalysis(ctx context.Context, job ai.AnalysisJob, reason string, now time.Time, maxAttempts int) error {
	return repository.failFn(ctx, job, reason, now, maxAttempts)
}

type fakeMediaStorage struct {
	data []byte
}

func (storage fakeMediaStorage) Put(context.Context, string, string, []byte) error { return nil }
func (storage fakeMediaStorage) Delete(context.Context, string) error              { return nil }
func (storage fakeMediaStorage) Open(context.Context, string) (io.ReadCloser, string, error) {
	return io.NopCloser(strings.NewReader(string(storage.data))), "image/png", nil
}

func TestWorkerProcessesEachClaimedMediaAndDoesNotMarkFailuresAnalyzed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"flood":null,"road":{"status":"CLOSED","confidence":0.8,"bounding_box":null},"analysis_time_ms":2,"model_version":"vision-1","is_mock":false,"limitations":["no road geometry"]}`))
	}))
	defer server.Close()

	job := ai.AnalysisJob{
		ID: "33333333-3333-4333-8333-333333333333", ReportID: "11111111-1111-4111-8111-111111111111", MediaID: "22222222-2222-4222-8222-222222222222",
		ObjectKey: "reports/11111111-1111-4111-8111-111111111111/22222222-2222-4222-8222-222222222222.png", ContentType: "image/png",
		ObservedAt: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), ReportUpdatedAt: time.Date(2026, 10, 1, 1, 0, 0, 0, time.UTC), Location: [2]float64{101, 14}, Attempts: 1,
	}
	var completed bool
	repository := &fakeEvidenceRepository{
		enqueueFn: func(context.Context, int) error { return nil },
		claimFn: func(context.Context, int, time.Duration, int) ([]ai.AnalysisJob, error) { return []ai.AnalysisJob{job}, nil },
		completeFn: func(_ context.Context, got ai.AnalysisJob, result ai.AnalyzeMediaResponse, _ time.Time) error {
			completed = got.ID == job.ID && result.Road != nil && !result.IsMock
			return nil
		},
		failFn: func(context.Context, ai.AnalysisJob, string, time.Time, int) error { t.Fatal("failure path called for successful analysis"); return nil },
	}
	worker := ai.NewEvidenceWorker(ai.NewClient(server.URL, time.Second), repository, fakeMediaStorage{data: []byte("image")}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	if err := worker.ProcessPendingReports(context.Background()); err != nil {
		t.Fatalf("ProcessPendingReports failed: %v", err)
	}
	if !completed {
		t.Fatal("worker did not persist successful nonmock analysis")
	}
}

func TestWorkerRetriesAnalysisFailureWithoutCompletingJob(t *testing.T) {
	job := ai.AnalysisJob{ID: "33333333-3333-4333-8333-333333333333", MediaID: "22222222-2222-4222-8222-222222222222", ObjectKey: "reports/a/b.png", ContentType: "image/png", Attempts: 2}
	var failed bool
	repository := &fakeEvidenceRepository{
		enqueueFn: func(context.Context, int) error { return nil },
		claimFn: func(context.Context, int, time.Duration, int) ([]ai.AnalysisJob, error) { return []ai.AnalysisJob{job}, nil },
		completeFn: func(context.Context, ai.AnalysisJob, ai.AnalyzeMediaResponse, time.Time) error { t.Fatal("complete path called after analysis failure"); return nil },
		failFn: func(_ context.Context, got ai.AnalysisJob, reason string, _ time.Time, maxAttempts int) error { failed = got.ID == job.ID && reason != "" && maxAttempts > 0; return nil },
	}
	worker := ai.NewEvidenceWorker(ai.NewClient("http://127.0.0.1:1", time.Second), repository, fakeMediaStorage{data: []byte("image")}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	if err := worker.ProcessPendingReports(context.Background()); err == nil {
		t.Fatal("ProcessPendingReports reported success after analysis failure")
	}
	if !failed {
		t.Fatal("worker did not lease failure for bounded retry")
	}
}

var _ report.MediaStorage = fakeMediaStorage{}
