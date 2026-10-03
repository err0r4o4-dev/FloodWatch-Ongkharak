package router

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"testing"
)

func TestGoogleRoutesExistWithoutConfiguration(t *testing.T) {
	server := New(Dependencies{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	for _, test := range []struct {
		method, path string
		status       int
	}{{"GET", "/api/v1/auth/google/config", 200}, {"GET", "/api/v1/auth/google/start", 503}, {"GET", "/api/v1/auth/google/callback", 503}, {"POST", "/api/v1/auth/google/link", 503}} {
		response := httptest.NewRecorder()
		server.ServeHTTP(response, httptest.NewRequest(test.method, test.path, nil))
		if response.Code != test.status {
			t.Errorf("%s %s: %d want %d", test.method, test.path, response.Code, test.status)
		}
	}
}

func TestGoogleOpenAPIPathsAndPasswordOnlyLink(t *testing.T) {
	data, err := staticFiles.ReadFile("openapi.json")
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	for _, suffix := range []string{"config", "start", "callback", "link"} {
		if len(document.Paths["/api/v1/auth/google/"+suffix]) == 0 {
			t.Errorf("missing Google contract: %s", suffix)
		}
	}
	var link struct {
		RequestBody struct {
			Content map[string]struct {
				Schema struct {
					AdditionalProperties bool           `json:"additionalProperties"`
					Required             []string       `json:"required"`
					Properties           map[string]any `json:"properties"`
				} `json:"schema"`
			} `json:"content"`
		} `json:"requestBody"`
	}
	if err := json.Unmarshal(document.Paths["/api/v1/auth/google/link"]["post"], &link); err != nil {
		t.Fatal(err)
	}
	schema := link.RequestBody.Content["application/json"].Schema
	if schema.AdditionalProperties || len(schema.Properties) != 1 || len(schema.Required) != 1 || schema.Required[0] != "password" {
		t.Fatal("link contract accepts account identifiers or does not require password")
	}
}
