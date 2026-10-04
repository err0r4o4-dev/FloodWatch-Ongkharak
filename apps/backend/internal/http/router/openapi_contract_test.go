package router

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/floodwatch-ongkharak/backend/internal/admin"
	"github.com/floodwatch-ongkharak/backend/internal/ai"
	"github.com/floodwatch-ongkharak/backend/internal/alert"
	"github.com/floodwatch-ongkharak/backend/internal/auth"
	"github.com/floodwatch-ongkharak/backend/internal/flood"
	"github.com/floodwatch-ongkharak/backend/internal/notification"
	"github.com/floodwatch-ongkharak/backend/internal/report"
	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"

	"gopkg.in/yaml.v3"
)

func TestFeatureRoutesHaveOpenAPIContracts(t *testing.T) {
	content, err := staticFiles.ReadFile("openapi.json")
	if err != nil {
		t.Fatal(err)
	}
	var specification struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(content, &specification); err != nil {
		t.Fatal(err)
	}
	redisClient := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	defer redisClient.Close()
	handler := New(Dependencies{Logger: slog.Default(), AuthService: &auth.Service{}, FloodService: &flood.Service{}, ReportService: &report.Service{}, AlertService: &alert.Service{}, AdminService: admin.NewService(nil), AIService: ai.NewEvidenceService(nil), NotificationService: notification.NewService(slog.Default(), nil, "", "", ""), RedisClient: redisClient})
	err = chi.Walk(handler.(chi.Routes), func(method, route string, handler http.Handler, middleware ...func(http.Handler) http.Handler) error {
		if _, ok := specification.Paths[route][strings.ToLower(method)]; !ok {
			t.Errorf("%s %s has no OpenAPI operation", method, route)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestOpenAPIComponentReferencesResolve(t *testing.T) {
	content, err := staticFiles.ReadFile("openapi.json")
	if err != nil {
		t.Fatal(err)
	}
	var specification map[string]any
	if err := json.Unmarshal(content, &specification); err != nil {
		t.Fatal(err)
	}
	var check func(any)
	check = func(value any) {
		switch node := value.(type) {
		case map[string]any:
			if reference, ok := node["$ref"].(string); ok {
				if !strings.HasPrefix(reference, "#/") {
					t.Errorf("unexpected external reference %s", reference)
					return
				}
				var target any = specification
				for _, segment := range strings.Split(strings.TrimPrefix(reference, "#/"), "/") {
					object, ok := target.(map[string]any)
					if !ok {
						t.Errorf("unresolved reference %s", reference)
						return
					}
					target, ok = object[segment]
					if !ok {
						t.Errorf("unresolved reference %s", reference)
						return
					}
				}
			}
			for _, child := range node {
				check(child)
			}
		case []any:
			for _, child := range node {
				check(child)
			}
		}
	}
	check(specification)
}

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

func TestGeoJSONCoordinatesAreTypedInOpenAPI(t *testing.T) {
	embeddedBytes, err := staticFiles.ReadFile("openapi.json")
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Components struct {
			Schemas map[string]struct {
				Properties           map[string]map[string]any `json:"properties"`
				OneOf                []map[string]any          `json:"oneOf"`
				AdditionalProperties *bool                     `json:"additionalProperties"`
				PrefixItems          []map[string]any          `json:"prefixItems"`
			} `json:"schemas"`
		} `json:"components"`
	}
	if err := json.Unmarshal(embeddedBytes, &document); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"FloodGeometry", "PolygonGeometry"} {
		if len(document.Components.Schemas[name].OneOf) == 0 {
			t.Errorf("%s must pair geometry type with its coordinates", name)
		}
	}
	for _, name := range []string{"PointGeometry", "SimplePolygonGeometry", "MultiPolygonGeometry", "MultiLineGeometry"} {
		if schema := document.Components.Schemas[name]; schema.AdditionalProperties == nil || *schema.AdditionalProperties {
			t.Errorf("%s accepts unsupported geometry properties", name)
		}
	}
	position := document.Components.Schemas["Position"].PrefixItems
	if len(position) != 2 || position[0]["minimum"] != float64(-180) || position[0]["maximum"] != float64(180) || position[1]["minimum"] != float64(-90) || position[1]["maximum"] != float64(90) {
		t.Errorf("position has no longitude/latitude bounds: %+v", position)
	}
	for _, name := range []string{"PointGeometry", "SimplePolygonGeometry", "MultiPolygonGeometry", "MultiLineGeometry"} {
		coordinates := document.Components.Schemas[name].Properties["coordinates"]
		if coordinates["type"] != "array" && coordinates["oneOf"] == nil && coordinates["$ref"] == nil {
			t.Errorf("%s has untyped coordinates: %+v", name, coordinates)
		}
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
