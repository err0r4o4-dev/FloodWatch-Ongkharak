package ai

import (
	"context"
	"errors"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/flood"
)

var (
	ErrInvalid   = errors.New("invalid AI evidence request")
	ErrNotFound  = errors.New("AI evidence not found")
	ErrConflict  = errors.New("AI evidence state conflict")
	ErrForbidden = errors.New("AI evidence review forbidden")
)

const (
	EvidenceUnverified = string(flood.Unverified)
	EvidenceVerified   = string(flood.Verified)
	EvidenceRejected   = string(flood.Rejected)
)

type ReviewActor struct {
	ID    string
	Email string
	Role  string
}

type EvidenceFilter struct {
	Status string
	Limit  int
	Offset int
}

type Evidence struct {
	ID                string         `json:"id"`
	ReportID          string         `json:"reportId"`
	MediaID           string         `json:"mediaId"`
	SourceType        string         `json:"sourceType"`
	ModelVersion      string         `json:"modelVersion"`
	Flood             *FloodEstimate `json:"flood"`
	Road              *RoadEstimate  `json:"road"`
	ObservedAt        time.Time      `json:"observedAt"`
	ReceivedAt        time.Time      `json:"receivedAt"`
	Limitations       []string       `json:"limitations"`
	Status            string         `json:"status"`
	ReportUpdatedAt   time.Time      `json:"reportUpdatedAt"`
	ReviewedAt        *time.Time     `json:"reviewedAt,omitempty"`
	ReviewedBy        *string        `json:"reviewedBy,omitempty"`
}

type AnalysisJob struct {
	ID              string
	ReportID        string
	MediaID         string
	ObjectKey       string
	ContentType     string
	LeaseToken      string
	ObservedAt      time.Time
	ReportUpdatedAt time.Time
	Location        [2]float64
	Attempts        int
}

type JobRepository interface {
	EnqueuePendingMedia(context.Context, int) error
	ClaimAnalysisJobs(context.Context, int, time.Duration, int) ([]AnalysisJob, error)
	CompleteAnalysis(context.Context, AnalysisJob, AnalyzeMediaResponse, time.Time) error
	FailAnalysis(context.Context, AnalysisJob, string, time.Time, int) error
}

type EvidenceRepository interface {
	ListEvidence(context.Context, EvidenceFilter) ([]Evidence, int, error)
	GetEvidence(context.Context, string) (Evidence, error)
	VerifyEvidence(context.Context, string, ReviewActor, string) error
	RejectEvidence(context.Context, string, ReviewActor, string) error
}

type EvidenceService struct{ repository EvidenceRepository }

func NewEvidenceService(repository EvidenceRepository) *EvidenceService {
	return &EvidenceService{repository: repository}
}

func (s *EvidenceService) ListEvidence(ctx context.Context, filter EvidenceFilter) ([]Evidence, int, error) {
	if filter.Status == "" {
		filter.Status = EvidenceUnverified
	}
	if filter.Status != EvidenceUnverified && filter.Status != EvidenceVerified && filter.Status != EvidenceRejected {
		return nil, 0, ErrInvalid
	}
	if filter.Limit <= 0 {
		filter.Limit = 50
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}
	if filter.Offset < 0 {
		return nil, 0, ErrInvalid
	}
	return s.repository.ListEvidence(ctx, filter)
}

func (s *EvidenceService) GetEvidence(ctx context.Context, id string) (Evidence, error) {
	return s.repository.GetEvidence(ctx, id)
}

func (s *EvidenceService) VerifyEvidence(ctx context.Context, id string, actor ReviewActor, roadID string) error {
	return s.repository.VerifyEvidence(ctx, id, actor, roadID)
}

func (s *EvidenceService) RejectEvidence(ctx context.Context, id string, actor ReviewActor, reason string) error {
	return s.repository.RejectEvidence(ctx, id, actor, reason)
}
