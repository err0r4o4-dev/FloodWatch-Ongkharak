package ai

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/report"
)

const (
	analysisBatchSize = 10
	analysisLease     = 5 * time.Minute
	analysisMaxTries  = 3
)

type EvidenceWorker struct {
	client       Analyzer
	repo         JobRepository
	mediaStorage report.MediaStorage
	logger       *slog.Logger
}

func NewEvidenceWorker(client Analyzer, repo JobRepository, mediaStorage report.MediaStorage, logger *slog.Logger) *EvidenceWorker {
	if logger == nil { logger = slog.Default() }
	return &EvidenceWorker{client: client, repo: repo, mediaStorage: mediaStorage, logger: logger}
}

func (w *EvidenceWorker) ProcessPendingReports(ctx context.Context) error {
	if err := w.repo.EnqueuePendingMedia(ctx, analysisBatchSize); err != nil { return fmt.Errorf("enqueue AI media: %w", err) }
	jobs, err := w.repo.ClaimAnalysisJobs(ctx, analysisBatchSize, analysisLease, analysisMaxTries)
	if err != nil { return fmt.Errorf("claim AI jobs: %w", err) }
	var firstJobError error
	for _, job := range jobs {
		jobCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		err := w.processJob(jobCtx, job)
		cancel()
		if err != nil {
			if firstJobError == nil { firstJobError = err }
			w.logger.Error("AI media analysis failed", slog.String("job_id", job.ID), slog.String("error", err.Error()))
			if failErr := w.repo.FailAnalysis(ctx, job, err.Error(), time.Now().UTC(), analysisMaxTries); failErr != nil { return fmt.Errorf("record AI job failure: %w", failErr) }
		}
	}
	if firstJobError != nil { return fmt.Errorf("one or more AI analysis jobs failed: %w", firstJobError) }
	return nil
}

func (w *EvidenceWorker) processJob(ctx context.Context, job AnalysisJob) error {
	file, contentType, err := w.mediaStorage.Open(ctx, job.ObjectKey)
	if err != nil { return fmt.Errorf("open private media: %w", err) }
	defer file.Close()
	if contentType != "" { job.ContentType = contentType }
	data, err := io.ReadAll(io.LimitReader(file, maxMediaBytes+1))
	if err != nil { return fmt.Errorf("read private media: %w", err) }
	if len(data) == 0 || len(data) > maxMediaBytes { return ErrInvalid }
	result, err := w.client.AnalyzeMedia(ctx, data, job.ContentType, job.ReportID, job.MediaID)
	if err != nil { return err }
	if result == nil || result.IsMock { return ErrInvalid }
	return w.repo.CompleteAnalysis(ctx, job, *result, time.Now().UTC())
}
