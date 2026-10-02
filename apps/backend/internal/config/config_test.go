package config

import (
	"strings"
	"testing"
)

func TestLoadRequiresValidAppEnvironment(t *testing.T) {
	setValidEnvironment(t)
	t.Setenv("APP_ENV", "preview")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "APP_ENV") {
		t.Fatalf("Load() error = %v, want APP_ENV validation error", err)
	}
}

func TestLoadAllowsUnconfiguredDevelopmentPush(t *testing.T) {
	setValidEnvironment(t)
	t.Setenv("VAPID_PUBLIC_KEY", "")
	t.Setenv("VAPID_PRIVATE_KEY", "")
	t.Setenv("VAPID_SUBSCRIBER", "")

	configuration, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if configuration.VAPIDPublicKey != "" || configuration.VAPIDPrivateKey != "" || configuration.VAPIDSubscriber != "" {
		t.Fatal("unconfigured push must remain disabled")
	}
}

func TestLoadRejectsPartialPushConfiguration(t *testing.T) {
	setValidEnvironment(t)
	t.Setenv("VAPID_PUBLIC_KEY", "public-key")
	t.Setenv("VAPID_PRIVATE_KEY", "")
	t.Setenv("VAPID_SUBSCRIBER", "")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "VAPID_PRIVATE_KEY") {
		t.Fatalf("Load() error = %v, want VAPID_PRIVATE_KEY validation error", err)
	}
}

func TestLoadRequiresPushConfigurationInProduction(t *testing.T) {
	setValidEnvironment(t)
	t.Setenv("APP_ENV", "production")
	t.Setenv("JWT_SECRET", "strong-production-example-secret-for-tests-only-42")
	t.Setenv("COOKIE_SECURE", "true")
	t.Setenv("VAPID_PUBLIC_KEY", "")
	t.Setenv("VAPID_PRIVATE_KEY", "")
	t.Setenv("VAPID_SUBSCRIBER", "")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "VAPID_PUBLIC_KEY") {
		t.Fatalf("Load() error = %v, want VAPID_PUBLIC_KEY validation error", err)
	}
}

func TestLoadDisablesSwaggerInProduction(t *testing.T) {
	setValidEnvironment(t)
	t.Setenv("APP_ENV", "production")
	t.Setenv("JWT_SECRET", "strong-production-example-secret-for-tests-only-42")
	t.Setenv("COOKIE_SECURE", "true")
	t.Setenv("SWAGGER_ENABLED", "true")

	configuration, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if configuration.SwaggerEnabled {
		t.Fatal("SwaggerEnabled = true, want false in production")
	}
}

func TestLoadRejectsInvalidMinIOEndpoint(t *testing.T) {
	setValidEnvironment(t)
	t.Setenv("MINIO_ENDPOINT", "minio.invalid/path")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "MINIO_ENDPOINT") {
		t.Fatalf("Load() error = %v, want MINIO_ENDPOINT validation error", err)
	}
}

func TestLoadRejectsWeakJWTSecret(t *testing.T) {
	setValidEnvironment(t)
	t.Setenv("JWT_SECRET", "short")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "JWT_SECRET") {
		t.Fatalf("Load() error = %v, want JWT_SECRET validation error", err)
	}
}

func TestLoadRejectsInsecureProductionCookie(t *testing.T) {
	setValidEnvironment(t)
	t.Setenv("APP_ENV", "production")
	t.Setenv("JWT_SECRET", "strong-production-example-secret-for-tests-only-42")
	t.Setenv("COOKIE_SECURE", "false")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "COOKIE_SECURE") {
		t.Fatalf("Load() error = %v, want COOKIE_SECURE validation error", err)
	}
}

func TestLoadRejectsInvalidTokenLifetimeAndSameSite(t *testing.T) {
	for _, test := range []struct {
		name  string
		value string
	}{
		{name: "JWT_ACCESS_TTL", value: "0s"},
		{name: "REFRESH_TOKEN_TTL", value: "not-a-duration"},
		{name: "COOKIE_SAMESITE", value: "None"},
	} {
		t.Run(test.name, func(t *testing.T) {
			setValidEnvironment(t)
			t.Setenv(test.name, test.value)
			_, err := Load()
			if err == nil || !strings.Contains(err.Error(), test.name) {
				t.Fatalf("Load() error = %v, want %s validation error", err, test.name)
			}
		})
	}
}

func TestLoadConfiguresDevelopmentFixture(t *testing.T) {
	setValidEnvironment(t)
	t.Setenv("INGESTION_FIXTURE_URL", "http://127.0.0.1:8090")
	t.Setenv("INGESTION_FIXTURE_SOURCE_CODE", "SYNTH_FIXTURE")

	configuration, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if configuration.IngestionFixtureURL != "http://127.0.0.1:8090" || configuration.IngestionFixtureSourceCode != "SYNTH_FIXTURE" {
		t.Fatalf("fixture configuration = %+v, want configured fixture", configuration)
	}
}

func TestLoadRejectsUnsafeFixtureConfiguration(t *testing.T) {
	for _, test := range []struct {
		name string
		set  func(*testing.T)
		want string
	}{
		{
			name: "missing source code",
			set: func(t *testing.T) {
				t.Setenv("INGESTION_FIXTURE_URL", "http://127.0.0.1:8090")
			},
			want: "INGESTION_FIXTURE_SOURCE_CODE",
		},
		{
			name: "source code without URL",
			set: func(t *testing.T) {
				t.Setenv("INGESTION_FIXTURE_SOURCE_CODE", "SYNTH_FIXTURE")
			},
			want: "INGESTION_FIXTURE_URL",
		},
		{
			name: "non-synthetic source code",
			set: func(t *testing.T) {
				t.Setenv("INGESTION_FIXTURE_URL", "http://127.0.0.1:8090")
				t.Setenv("INGESTION_FIXTURE_SOURCE_CODE", "COMMUNITY_FIXTURE")
			},
			want: "INGESTION_FIXTURE_SOURCE_CODE",
		},
		{
			name: "production fixture",
			set: func(t *testing.T) {
				t.Setenv("APP_ENV", "production")
				t.Setenv("JWT_SECRET", "strong-production-example-secret-for-tests-only-42")
				t.Setenv("COOKIE_SECURE", "true")
				t.Setenv("INGESTION_FIXTURE_URL", "https://fixture.example.test")
				t.Setenv("INGESTION_FIXTURE_SOURCE_CODE", "SYNTH_FIXTURE")
			},
			want: "INGESTION_FIXTURE_URL",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			setValidEnvironment(t)
			test.set(t)

			_, err := Load()
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Load() error = %v, want %s validation error", err, test.want)
			}
		})
	}
}

func setValidEnvironment(t *testing.T) {
	t.Helper()
	t.Setenv("APP_ENV", "development")
	t.Setenv("API_PORT", "8080")
	t.Setenv("DATABASE_URL", "postgres://floodwatch:development-only@localhost:5432/floodwatch?sslmode=disable")
	t.Setenv("REDIS_URL", "redis://localhost:6379/0")
	t.Setenv("MINIO_ENDPOINT", "http://localhost:9000")
	t.Setenv("MINIO_ACCESS_KEY", "minioadmin")
	t.Setenv("MINIO_SECRET_KEY", "development-only-secret")
	t.Setenv("MINIO_BUCKET", "floodwatch")
	t.Setenv("CORS_ALLOWED_ORIGINS", "http://localhost:5173")
	t.Setenv("JWT_ISSUER", "floodwatch-ongkharak")
	t.Setenv("JWT_SECRET", "development-only-change-this-long-secret-before-deployment")
	t.Setenv("JWT_ACCESS_TTL", "15m")
	t.Setenv("REFRESH_TOKEN_TTL", "168h")
	t.Setenv("COOKIE_NAME", "fw_refresh")
	t.Setenv("COOKIE_SECURE", "false")
	t.Setenv("COOKIE_SAMESITE", "Lax")
	t.Setenv("AI_SERVICE_URL", "http://localhost:8001")
	t.Setenv("VAPID_PUBLIC_KEY", "test-public-key")
	t.Setenv("VAPID_PRIVATE_KEY", "test-private-key")
	t.Setenv("VAPID_SUBSCRIBER", "mailto:admin@example.test")
	t.Setenv("SWAGGER_ENABLED", "true")
	t.Setenv("INGESTION_FIXTURE_URL", "")
	t.Setenv("INGESTION_FIXTURE_SOURCE_CODE", "")
}
