package ai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/flood"
	"github.com/google/uuid"
)

const (
	maxMediaBytes    = 5 * 1024 * 1024
	maxResponseBytes = 2 * 1024 * 1024
)

type AnalyzeMediaRequest struct {
	MediaBase64 string `json:"media_base64"`
	ContentType string `json:"content_type"`
	ReportID    string `json:"report_id"`
	MediaID     string `json:"media_id"`
}

type FloodEstimate struct {
	Severity    flood.Severity `json:"severity"`
	WaterDepthM *float64       `json:"water_depth_m"`
	Confidence  float64        `json:"confidence"`
	BoundingBox []float64      `json:"bounding_box"`
}

type RoadEstimate struct {
	Status      flood.RoadStatus `json:"status"`
	Confidence  float64          `json:"confidence"`
	BoundingBox []float64        `json:"bounding_box"`
}

type AnalyzeMediaResponse struct {
	Flood          *FloodEstimate `json:"flood"`
	Road           *RoadEstimate  `json:"road"`
	AnalysisTimeMs int            `json:"analysis_time_ms"`
	ModelVersion   string         `json:"model_version"`
	IsMock         bool           `json:"is_mock"`
	Limitations    []string       `json:"limitations"`
}

type responseEnvelope struct {
	Flood          *FloodEstimate `json:"flood"`
	Road           *RoadEstimate  `json:"road"`
	AnalysisTimeMs int            `json:"analysis_time_ms"`
	ModelVersion   string         `json:"model_version"`
	IsMock         *bool          `json:"is_mock"`
	Limitations    []string       `json:"limitations"`
}

type Analyzer interface {
	AnalyzeMedia(context.Context, []byte, string, string, string) (*AnalyzeMediaResponse, error)
}

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string, timeout time.Duration) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: timeout,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

func (c *Client) AnalyzeMedia(ctx context.Context, media []byte, contentType, reportID, mediaID string) (*AnalyzeMediaResponse, error) {
	if len(media) == 0 || len(media) > maxMediaBytes || !validContentType(contentType) || uuid.Validate(reportID) != nil || uuid.Validate(mediaID) != nil {
		return nil, ErrInvalid
	}
	payload, err := json.Marshal(AnalyzeMediaRequest{MediaBase64: base64.StdEncoding.EncodeToString(media), ContentType: contentType, ReportID: reportID, MediaID: mediaID})
	if err != nil {
		return nil, fmt.Errorf("marshal AI request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/analyze/media", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create AI request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute AI request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("AI service returned status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read AI response: %w", err)
	}
	if len(body) > maxResponseBytes {
		return nil, ErrInvalid
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	var envelope responseEnvelope
	if err := decoder.Decode(&envelope); err != nil {
		return nil, fmt.Errorf("decode AI response: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, ErrInvalid
	}
	if err := validateResponse(envelope); err != nil {
		return nil, err
	}
	return &AnalyzeMediaResponse{Flood: envelope.Flood, Road: envelope.Road, AnalysisTimeMs: envelope.AnalysisTimeMs, ModelVersion: envelope.ModelVersion, IsMock: *envelope.IsMock, Limitations: envelope.Limitations}, nil
}

func validateResponse(result responseEnvelope) error {
	if result.IsMock == nil {
		return ErrInvalid
	}
	return ValidateAnalysisResponse(AnalyzeMediaResponse{Flood: result.Flood, Road: result.Road, AnalysisTimeMs: result.AnalysisTimeMs, ModelVersion: result.ModelVersion, IsMock: *result.IsMock, Limitations: result.Limitations})
}

func ValidateAnalysisResponse(result AnalyzeMediaResponse) error {
	modelVersion := strings.TrimSpace(result.ModelVersion)
	if result.IsMock || len(modelVersion) < 1 || len(modelVersion) > 100 || strings.Contains(strings.ToLower(modelVersion), "mock") || result.AnalysisTimeMs < 0 || result.AnalysisTimeMs > 300000 || len(result.Limitations) < 1 || len(result.Limitations) > 10 {
		return ErrInvalid
	}
	for _, limitation := range result.Limitations {
		if strings.TrimSpace(limitation) == "" || len(limitation) > 500 {
			return ErrInvalid
		}
	}
	if result.Flood != nil {
		if !validSeverity(result.Flood.Severity) || !validConfidence(result.Flood.Confidence) || !validBox(result.Flood.BoundingBox) || (result.Flood.WaterDepthM != nil && (!finite(*result.Flood.WaterDepthM) || *result.Flood.WaterDepthM < 0 || *result.Flood.WaterDepthM > 50)) {
			return ErrInvalid
		}
	}
	if result.Road != nil && (!validRoadStatus(result.Road.Status) || !validConfidence(result.Road.Confidence) || !validBox(result.Road.BoundingBox)) {
		return ErrInvalid
	}
	return nil
}

func ValidateEvidencePayload(modelVersion string, floodEstimate *FloodEstimate, roadEstimate *RoadEstimate, limitations []string) error {
	return ValidateAnalysisResponse(AnalyzeMediaResponse{Flood: floodEstimate, Road: roadEstimate, ModelVersion: modelVersion, IsMock: false, AnalysisTimeMs: 0, Limitations: limitations})
}

func validContentType(value string) bool {
	return value == "image/jpeg" || value == "image/png" || value == "image/webp"
}
func validSeverity(value flood.Severity) bool {
	return value == flood.SeverityInfo || value == flood.SeverityWatch || value == flood.SeverityWarning || value == flood.SeverityCritical || value == flood.SeverityUnknown
}
func validRoadStatus(value flood.RoadStatus) bool {
	return value == flood.RoadPassable || value == flood.RoadWatch || value == flood.RoadSmallVehicle || value == flood.RoadClosed || value == flood.RoadUnknown
}
func validConfidence(value float64) bool { return finite(value) && value >= 0 && value <= 1 }
func validBox(value []float64) bool {
	if value == nil {
		return true
	}
	if len(value) != 4 {
		return false
	}
	for _, coordinate := range value {
		if !finite(coordinate) || coordinate < 0 || coordinate > 1 {
			return false
		}
	}
	return value[0] <= value[2] && value[1] <= value[3]
}
func finite(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }
