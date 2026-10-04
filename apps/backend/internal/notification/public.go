package notification

import (
	"errors"
	"time"
)

var ErrInvalidPublicNotificationFilter = errors.New("invalid public notification filter")

type PublicNotificationBBox struct {
	MinLng float64
	MinLat float64
	MaxLng float64
	MaxLat float64
}

type PublicNotificationFilter struct {
	Limit  int
	Offset int
	BBox   *PublicNotificationBBox
}

type PublicNotificationLocation struct {
	Type        string    `json:"type"`
	Coordinates []float64 `json:"coordinates"`
}

type PublicNotificationItem struct {
	ID         string                      `json:"id"`
	Severity   string                      `json:"severity"`
	SourceType string                      `json:"sourceType"`
	CreatedAt  time.Time                   `json:"createdAt"`
	Title      string                      `json:"title"`
	Message    string                      `json:"message"`
	URL        string                      `json:"url"`
	Location   *PublicNotificationLocation `json:"location"`
}

type PublicNotificationPage struct {
	Items []PublicNotificationItem
	Total int
}
