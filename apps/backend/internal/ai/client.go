package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/flood"
)

type AnalyzeMediaRequest struct {
	MediaURL string `json:"media_url"`
	ReportID string `json:"report_id,omitempty"`
}

type FloodEstimate struct {
	Severity     flood.Severity `json:"severity"`
	WaterDepthM  *float64       `json:"water_depth_m"`
	Confidence   float64        `json:"confidence"`
	BoundingBox  []float64      `json:"bounding_box"`
}

type RoadEstimate struct {
	Status       flood.RoadStatus `json:"status"`
	Confidence   float64          `json:"confidence"`
	BoundingBox  []float64        `json:"bounding_box"`
}

type AnalyzeMediaResponse struct {
	Flood          *FloodEstimate `json:"flood"`
	Road           *RoadEstimate  `json:"road"`
	AnalysisTimeMs int            `json:"analysis_time_ms"`
	ModelVersion   string         `json:"model_version"`
}

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string, timeout time.Duration) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *Client) AnalyzeMedia(ctx context.Context, mediaURL, reportID string) (*AnalyzeMediaResponse, error) {
	reqBody, err := json.Marshal(AnalyzeMediaRequest{
		MediaURL: mediaURL,
		ReportID: reportID,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal AI request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/analyze/media", bytes.NewReader(reqBody))
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

	var result AnalyzeMediaResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode AI response: %w", err)
	}

	return &result, nil
}
