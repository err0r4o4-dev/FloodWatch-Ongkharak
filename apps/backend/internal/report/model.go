package report

import (
	"context"
	"errors"
	"io"
	"time"
)

var (
	ErrInvalid   = errors.New("invalid community report input")
	ErrNotFound  = errors.New("community report not found")
	ErrForbidden = errors.New("community report access denied")
	ErrConflict  = errors.New("community report state conflict")
)

type Type string

const (
	TypeFlood Type = "FLOOD"
	TypeRoad  Type = "ROAD"
	TypeOther Type = "OTHER"
)

type Status string

const (
	StatusPending  Status = "PENDING"
	StatusVerified Status = "VERIFIED"
	StatusRejected Status = "REJECTED"
)

const SourceCommunity = "COMMUNITY"

type WaterTrend string

const (
	TrendUnknown WaterTrend = "UNKNOWN"
	TrendRising  WaterTrend = "RISING"
	TrendStable  WaterTrend = "STABLE"
	TrendFalling WaterTrend = "FALLING"
)

type RoadSegmentAssignmentMethod string

const (
	AssignmentUnassigned    RoadSegmentAssignmentMethod = "UNASSIGNED"
	AssignmentUserConfirmed RoadSegmentAssignmentMethod = "USER_CONFIRMED"
	AssignmentLegacyMatch   RoadSegmentAssignmentMethod = "LEGACY_MATCH"
)

type PublicationStatus string

const (
	PublicationPublished PublicationStatus = "PUBLISHED"
	PublicationRemoved   PublicationStatus = "REMOVED"
)

type ReviewEvent struct {
	ID             string    `json:"id"`
	Action         string    `json:"action"`
	PreviousStatus Status    `json:"previousStatus"`
	Status         Status    `json:"status"`
	ActorID        string    `json:"actorId"`
	Reason         string    `json:"reason"`
	CreatedAt      time.Time `json:"createdAt"`
}

func IsPublished(status Status) bool {
	return status == StatusPending || status == StatusVerified
}

func CanReview(previous, next Status) bool {
	switch next {
	case StatusVerified:
		return previous == StatusPending
	case StatusRejected:
		return IsPublished(previous)
	case StatusPending:
		return previous == StatusRejected
	default:
		return false
	}
}

func (item Report) PublicationStatus() PublicationStatus {
	if IsPublished(item.Status) {
		return PublicationPublished
	}
	return PublicationRemoved
}

type Location struct {
	Type        string    `json:"type"`
	Coordinates []float64 `json:"coordinates"`
}

type Input struct {
	Type                 Type       `json:"type"`
	Description          string     `json:"description"`
	Location             Location   `json:"location"`
	LocationName         *string    `json:"locationName"`
	LocationNote         *string    `json:"locationNote"`
	EstimatedWaterDepthM *float64   `json:"estimatedWaterDepthM"`
	ObservedAt           *time.Time `json:"observedAt"`
	WaterTrend           WaterTrend `json:"waterTrend"`
	RoadSegmentID        *string    `json:"roadSegmentId"`
}

type GuestInput struct {
	Input
	DisplayName string `json:"displayName"`
}

type GuestCreation struct {
	Report      Report
	UploadToken string
}

type Page struct {
	Limit  int
	Offset int
}

type BoundingBox struct {
	MinLng float64
	MinLat float64
	MaxLng float64
	MaxLat float64
}

type ListFilter struct {
	Page Page
	BBox *BoundingBox
}

type Report struct {
	ID                   string
	UserID               string
	DisplayName          string
	Type                 Type
	Description          string
	Location             Location
	LocationName         string
	LocationNote         string
	EstimatedWaterDepthM *float64
	ObservedAt           *time.Time
	WaterTrend           WaterTrend
	Status               Status
	VerificationStatus   Status
	SourceType           string
	CreatedAt            time.Time
	UpdatedAt            time.Time
	VerifiedAt           *time.Time
	VerifiedBy           *string
	ReviewedAt           *time.Time
	ReviewedBy           *string
	RejectionReason      *string
	RoadSegmentID        *string
	RoadSegmentAssignmentMethod RoadSegmentAssignmentMethod
	RoadSegmentAssignmentPending bool
	Media                []Media
	ReviewHistory        []ReviewEvent
}

type Media struct {
	ID            string
	ReportID      string
	ObjectKey     string
	ContentType   string
	SizeBytes     int
	CreatedAt     time.Time
	ContentSHA256 string
}

type Image struct {
	ContentType string
	Data        []byte
}

type Access struct {
	UserID  string
	IsAdmin bool
}

type MediaFile struct {
	Content io.ReadCloser
	Media   Media
}

type Repository interface {
	Create(context.Context, Report) (Report, error)
	Get(context.Context, string) (Report, error)
	GetAdmin(context.Context, string) (Report, error)
	ListPublic(context.Context, ListFilter) ([]Report, error)
	ListMine(context.Context, string, Page) ([]Report, error)
	ListAdmin(context.Context, Status, Page) ([]Report, error)
	UpdatePending(context.Context, string, Input) (Report, error)
	Review(context.Context, string, Status, string, string, time.Time) (Report, error)
	CreateMedia(context.Context, Media, string) (Media, error)
	GetMedia(context.Context, string, string) (Media, error)
}

type RoadSegmentValidator interface {
	ValidateRoadSegmentAssignment(context.Context, string, float64, float64) error
}

type RoadSegmentAssignment struct {
	ID      *string
	Method  RoadSegmentAssignmentMethod
	Pending bool
}

// AssignmentRepository is optional so existing report repositories and tests can keep their established update contract.
type AssignmentRepository interface {
	UpdatePendingWithAssignment(context.Context, string, Input, RoadSegmentAssignment) (Report, error)
}

type GuestRepository interface {
	ValidateGuestUpload(context.Context, string, []byte, time.Time) error
	CreateGuest(context.Context, Report, []byte, time.Time) (Report, error)
	CreateGuestMedia(context.Context, Media, []byte, time.Time) (Media, error)
}

type MediaStorage interface {
	Put(context.Context, string, string, []byte) error
	Open(context.Context, string) (io.ReadCloser, string, error)
	Delete(context.Context, string) error
}
