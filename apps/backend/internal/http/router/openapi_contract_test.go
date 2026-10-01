package router

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestEmbeddedOpenAPIContractMatchesCanonicalDocument(t *testing.T) {
	canonicalPath := filepath.Join("..", "..", "..", "..", "..", "docs", "api", "openapi.yaml")
	canonicalBytes, err := os.ReadFile(canonicalPath)
	if err != nil {
		t.Fatalf("read canonical OpenAPI contract: %v", err)
	}
	embeddedBytes, err := staticFiles.ReadFile("openapi.json")
	if err != nil {
		t.Fatalf("read embedded OpenAPI contract: %v", err)
	}

	canonicalDocument, err := normalizeYAML(canonicalBytes)
	if err != nil {
		t.Fatalf("parse canonical OpenAPI contract: %v", err)
	}
	embeddedDocument, err := normalizeJSON(embeddedBytes)
	if err != nil {
		t.Fatalf("parse embedded OpenAPI contract: %v", err)
	}
	if !reflect.DeepEqual(canonicalDocument, embeddedDocument) {
		t.Fatal("embedded OpenAPI contract differs from docs/api/openapi.yaml")
	}
}

func normalizeYAML(documentBytes []byte) (any, error) {
	var document any
	if err := yaml.Unmarshal(documentBytes, &document); err != nil {
		return nil, err
	}
	jsonBytes, err := json.Marshal(document)
	if err != nil {
		return nil, err
	}
	return normalizeJSON(jsonBytes)
}

func normalizeJSON(documentBytes []byte) (any, error) {
	var document any
	if err := json.Unmarshal(documentBytes, &document); err != nil {
		return nil, err
	}
	return document, nil
}
