package config

import (
	"strings"
	"testing"
)

func TestGoogleConfigurationOptionalAndValidated(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*testing.T)
		valid  bool
	}{
		{name: "disabled", valid: true, change: func(*testing.T) {}},
		{name: "complete", valid: true, change: googleEnvironment},
		{name: "partial", change: func(t *testing.T) { t.Setenv("GOOGLE_CLIENT_SECRET", "sensitive-test-secret") }},
		{name: "wrong callback", change: func(t *testing.T) {
			googleEnvironment(t)
			t.Setenv("GOOGLE_REDIRECT_URI", "http://localhost:5173/callback")
		}},
		{name: "unapproved origin", change: func(t *testing.T) { googleEnvironment(t); t.Setenv("AUTH_FRONTEND_ORIGIN", "http://localhost:8099") }},
		{name: "credentials in URL", change: func(t *testing.T) {
			googleEnvironment(t)
			t.Setenv("GOOGLE_REDIRECT_URI", "http://user:password@localhost:5173/api/v1/auth/google/callback")
		}},
		{name: "non-loopback http", change: func(t *testing.T) {
			googleEnvironment(t)
			t.Setenv("AUTH_FRONTEND_ORIGIN", "http://example.test")
			t.Setenv("CORS_ALLOWED_ORIGINS", "http://example.test")
			t.Setenv("GOOGLE_REDIRECT_URI", "http://example.test/api/v1/auth/google/callback")
		}},
		{name: "different callback origin", change: func(t *testing.T) {
			googleEnvironment(t)
			t.Setenv("GOOGLE_REDIRECT_URI", "http://localhost:8080/api/v1/auth/google/callback")
		}},
		{name: "production http", change: func(t *testing.T) {
			googleEnvironment(t)
			t.Setenv("APP_ENV", "production")
			t.Setenv("COOKIE_SECURE", "true")
			t.Setenv("JWT_SECRET", "strong-production-example-secret-for-tests-only-42")
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			setValidEnvironment(t)
			for _, name := range []string{"GOOGLE_CLIENT_ID", "GOOGLE_CLIENT_SECRET", "GOOGLE_REDIRECT_URI", "AUTH_FRONTEND_ORIGIN"} {
				t.Setenv(name, "")
			}
			test.change(t)
			configuration, err := Load()
			if test.valid && err != nil {
				t.Fatal(err)
			}
			if !test.valid && err == nil {
				t.Fatal("unsafe Google configuration accepted")
			}
			if err != nil && strings.Contains(err.Error(), "sensitive-test-secret") {
				t.Fatal("secret exposed in config error")
			}
			if test.name == "complete" && configuration.GoogleClientID != "test-client" {
				t.Fatal("Google config not loaded")
			}
		})
	}
}
func googleEnvironment(t *testing.T) {
	t.Setenv("GOOGLE_CLIENT_ID", "test-client")
	t.Setenv("GOOGLE_CLIENT_SECRET", "sensitive-test-secret")
	t.Setenv("GOOGLE_REDIRECT_URI", "http://localhost:5173/api/v1/auth/google/callback")
	t.Setenv("AUTH_FRONTEND_ORIGIN", "http://localhost:5173")
}
