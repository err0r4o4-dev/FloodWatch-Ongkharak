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
		{name: "partial", change: func(t *testing.T) { t.Setenv("GOOGLE_OAUTH_CLIENT_SECRET", "sensitive-test-secret") }},
		{name: "wrong callback", change: func(t *testing.T) {
			googleEnvironment(t)
			t.Setenv("GOOGLE_OAUTH_REDIRECT_URL", "http://localhost:5173/callback")
		}},
		{name: "unapproved origin", change: func(t *testing.T) {
			googleEnvironment(t)
			t.Setenv("GOOGLE_OAUTH_REDIRECT_URL", "http://127.0.0.1:8099/api/v1/auth/google/callback")
		}},
		{name: "credentials in URL", change: func(t *testing.T) {
			googleEnvironment(t)
			t.Setenv("GOOGLE_OAUTH_REDIRECT_URL", "http://user:password@localhost:5173/api/v1/auth/google/callback")
		}},
		{name: "non-loopback http", change: func(t *testing.T) {
			googleEnvironment(t)
			t.Setenv("CORS_ALLOWED_ORIGINS", "http://example.test")
			t.Setenv("GOOGLE_OAUTH_REDIRECT_URL", "http://example.test/api/v1/auth/google/callback")
		}},
		{name: "approved custom port", valid: true, change: func(t *testing.T) {
			googleEnvironment(t)
			t.Setenv("CORS_ALLOWED_ORIGINS", "http://localhost:8088")
			t.Setenv("GOOGLE_OAUTH_REDIRECT_URL", "http://localhost:8088/api/v1/auth/google/callback")
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
			for _, name := range []string{"GOOGLE_OAUTH_CLIENT_ID", "GOOGLE_OAUTH_CLIENT_SECRET", "GOOGLE_OAUTH_REDIRECT_URL"} {
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
			if test.name == "complete" && (configuration.GoogleClientID != "test-client" || configuration.AuthFrontendOrigin != "http://localhost:5173") {
				t.Fatal("Google config not loaded")
			}
			if test.name == "approved custom port" && configuration.AuthFrontendOrigin != "http://localhost:8088" {
				t.Fatal("frontend origin must be derived from the configured callback")
			}
		})
	}
}

func TestGoogleSplitPortConfiguration(t *testing.T) {
	for _, test := range []struct {
		name        string
		redirectURL string
		origins     []string
		environment string
		valid       bool
	}{
		{name: "development split ports", redirectURL: "http://localhost:8080/api/v1/auth/google/callback", origins: []string{"http://localhost:5173"}, environment: "development", valid: true},
		{name: "production split ports", redirectURL: "https://app.example.test:8080/api/v1/auth/google/callback", origins: []string{"https://app.example.test:5173"}, environment: "production", valid: true},
		{name: "different host", redirectURL: "http://localhost:8080/api/v1/auth/google/callback", origins: []string{"http://127.0.0.1:5173"}, environment: "development"},
		{name: "different scheme", redirectURL: "https://localhost:8080/api/v1/auth/google/callback", origins: []string{"http://localhost:5173"}, environment: "development"},
		{name: "ambiguous frontend", redirectURL: "http://localhost:8080/api/v1/auth/google/callback", origins: []string{"http://localhost:5173", "http://localhost:8088"}, environment: "development"},
		{name: "no frontend", redirectURL: "http://localhost:8080/api/v1/auth/google/callback", environment: "development"},
		{name: "frontend credentials", redirectURL: "http://localhost:8080/api/v1/auth/google/callback", origins: []string{"http://user:password@localhost:5173"}, environment: "development"},
		{name: "frontend path", redirectURL: "http://localhost:8080/api/v1/auth/google/callback", origins: []string{"http://localhost:5173/login"}, environment: "development"},
		{name: "frontend query", redirectURL: "http://localhost:8080/api/v1/auth/google/callback", origins: []string{"http://localhost:5173?next=evil"}, environment: "development"},
		{name: "frontend fragment", redirectURL: "http://localhost:8080/api/v1/auth/google/callback", origins: []string{"http://localhost:5173#fragment"}, environment: "development"},
		{name: "production HTTP", redirectURL: "http://localhost:8080/api/v1/auth/google/callback", origins: []string{"http://localhost:5173"}, environment: "production"},
	} {
		t.Run(test.name, func(t *testing.T) {
			googleEnvironment(t)
			t.Setenv("GOOGLE_OAUTH_REDIRECT_URL", test.redirectURL)
			settings, err := loadGoogle(test.environment, test.origins)
			if !test.valid {
				if err == nil {
					t.Fatal("unsafe split-origin configuration accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if settings.frontendOrigin != test.origins[0] || settings.redirectURI != test.redirectURL {
				t.Fatal("frontend origin or callback URL changed")
			}
		})
	}
}
func googleEnvironment(t *testing.T) {
	t.Setenv("GOOGLE_OAUTH_CLIENT_ID", "test-client")
	t.Setenv("GOOGLE_OAUTH_CLIENT_SECRET", "sensitive-test-secret")
	t.Setenv("GOOGLE_OAUTH_REDIRECT_URL", "http://localhost:5173/api/v1/auth/google/callback")
}

func TestGoogleRedirectOriginValidation(t *testing.T) {
	for _, test := range []struct {
		name        string
		redirectURL string
		origin      string
		environment string
		valid       bool
	}{
		{name: "production HTTPS", redirectURL: "https://app.example.test:8443/api/v1/auth/google/callback", origin: "https://app.example.test:8443", environment: "production", valid: true},
		{name: "loopback IPv6", redirectURL: "http://[::1]:5173/api/v1/auth/google/callback", origin: "http://[::1]:5173", environment: "development", valid: true},
		{name: "production HTTP", redirectURL: "http://localhost:5173/api/v1/auth/google/callback", origin: "http://localhost:5173", environment: "production"},
		{name: "query", redirectURL: "https://app.example.test/api/v1/auth/google/callback?next=evil", origin: "https://app.example.test"},
		{name: "empty query", redirectURL: "https://app.example.test/api/v1/auth/google/callback?", origin: "https://app.example.test"},
		{name: "fragment", redirectURL: "https://app.example.test/api/v1/auth/google/callback#fragment", origin: "https://app.example.test"},
		{name: "encoded path", redirectURL: "https://app.example.test/api/v1/auth/google/%63allback", origin: "https://app.example.test"},
		{name: "trailing slash", redirectURL: "https://app.example.test/api/v1/auth/google/callback/", origin: "https://app.example.test"},
		{name: "missing host", redirectURL: "/api/v1/auth/google/callback"},
		{name: "malformed URL", redirectURL: "https://%/api/v1/auth/google/callback"},
		{name: "oversized URL", redirectURL: "https://app.example.test/api/v1/auth/google/callback?" + strings.Repeat("a", 2048), origin: "https://app.example.test"},
	} {
		t.Run(test.name, func(t *testing.T) {
			googleEnvironment(t)
			t.Setenv("GOOGLE_OAUTH_REDIRECT_URL", test.redirectURL)
			settings, err := loadGoogle(test.environment, []string{test.origin})
			if test.valid {
				if err != nil {
					t.Fatal(err)
				}
				if settings.frontendOrigin != test.origin || settings.redirectURI != test.redirectURL {
					t.Fatal("callback URL or derived origin changed")
				}
			} else if err == nil {
				t.Fatal("unsafe callback accepted")
			}
		})
	}
}
