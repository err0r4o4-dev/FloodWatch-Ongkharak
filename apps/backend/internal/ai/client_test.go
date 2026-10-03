package ai_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/ai"
)

func TestClientSendsPrivateMediaPayloadAndAcceptsNonMockResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if body["media_base64"] != base64.StdEncoding.EncodeToString([]byte("image")) || body["content_type"] != "image/png" {
			t.Fatalf("request body = %#v", body)
		}
		if body["report_id"] != "11111111-1111-4111-8111-111111111111" || body["media_id"] != "22222222-2222-4222-8222-222222222222" {
			t.Fatalf("request IDs = %#v", body)
		}
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"flood":{"severity":"WARNING","water_depth_m":1.2,"confidence":0.9,"bounding_box":[0.1,0.2,0.8,0.9]},"road":null,"analysis_time_ms":12,"model_version":"vision-1","is_mock":false,"limitations":["synthetic test"]}`))
	}))
	defer server.Close()

	client := ai.NewClient(server.URL, time.Second)
	result, err := client.AnalyzeMedia(context.Background(), []byte("image"), "image/png", "11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222")
	if err != nil {
		t.Fatalf("AnalyzeMedia failed: %v", err)
	}
	if result.Flood == nil || result.Flood.Severity != "WARNING" || result.IsMock {
		t.Fatalf("result = %#v", result)
	}
}

func TestClientRejectsMissingOrMockModelMetadata(t *testing.T) {
	cases := []string{
		`{"flood":null,"road":null,"analysis_time_ms":1,"model_version":"vision-1","limitations":["x"]}`,
		`{"flood":null,"road":null,"analysis_time_ms":1,"model_version":"vision-1","is_mock":true,"limitations":["x"]}`,
		`{"flood":null,"road":null,"analysis_time_ms":1,"model_version":"mock","is_mock":false,"limitations":["x"]}`,
		`{"flood":{"severity":"NOT_VALID","confidence":0.5,"bounding_box":null},"road":null,"analysis_time_ms":1,"model_version":"vision-1","is_mock":false,"limitations":["x"]}`,
	}
	for _, payload := range cases {
		t.Run(payload, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
				response.Header().Set("Content-Type", "application/json")
				_, _ = response.Write([]byte(payload))
			}))
			defer server.Close()

			_, err := ai.NewClient(server.URL, time.Second).AnalyzeMedia(context.Background(), []byte("image"), "image/png", "11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222")
			if err == nil {
				t.Fatal("AnalyzeMedia accepted unsafe model response")
			}
		})
	}
}

func TestClientRejectsOversizedMediaAndInvalidIDs(t *testing.T) {
	client := ai.NewClient("http://127.0.0.1:1", time.Second)
	if _, err := client.AnalyzeMedia(context.Background(), []byte(strings.Repeat("x", 5*1024*1024+1)), "image/png", "11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222"); err == nil {
		t.Fatal("AnalyzeMedia accepted oversized media")
	}
	if _, err := client.AnalyzeMedia(context.Background(), []byte("image"), "image/gif", "not-a-uuid", "not-a-uuid"); err == nil {
		t.Fatal("AnalyzeMedia accepted invalid media metadata")
	}
}
