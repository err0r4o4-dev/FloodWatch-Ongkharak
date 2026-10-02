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

type Location struct {
	Type        string    `json:"type"`
	Coordinates []float64 `json:"coordinates"`
}

type Input struct {
	Type                 Type     `json:"type"`
	Description          string   `json:"description"`
	Location             Location `json:"location"`
	EstimatedWaterDepthM *float64 `json:"estimatedWaterDepthM"`
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
	Type                 Type
	Description          string
	Location             Location
	EstimatedWaterDepthM *float64
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
	Media                []Media
}

type Media struct {
	ID          string
	ReportID    string
	ObjectKey   string
	ContentType string
	SizeBytes   int
	CreatedAt   time.Time
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
	ListVerified(context.Context, ListFilter) ([]Report, error)
	ListMine(context.Context, string, Page) ([]Report, error)
	ListAdmin(context.Context, Status, Page) ([]Report, error)
	UpdatePending(context.Context, string, Input) (Report, error)
	Review(context.Context, string, Status, string, string, time.Time) (Report, error)
	CreateMedia(context.Context, Media, string) (Media, error)
	GetMedia(context.Context, string, string) (Media, error)
}

type MediaStorage interface {
	Put(context.Context, string, string, []byte) error
	Open(context.Context, string) (io.ReadCloser, string, error)
	Delete(context.Context, string) error
}
