package config

import (
	"strings"
	"testing"
)

func TestLoadSMTPDefaultsToDisabledWithTLSRequired(t *testing.T) {
	setValidEnvironment(t)
	clearSMTPEnvironment(t)

	configuration, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if configuration.SMTPHost != "" || configuration.SMTPFrom != "" || configuration.SMTPUsername != "" || configuration.SMTPPassword != "" {
		t.Fatalf("SMTP configuration = %+v, want disabled", configuration)
	}
	if configuration.SMTPPort != 587 || !configuration.SMTPRequireTLS {
		t.Fatalf("SMTP defaults = port %d, requireTLS %t; want 587, true", configuration.SMTPPort, configuration.SMTPRequireTLS)
	}
}

func TestLoadSMTPRejectsPartialConfiguration(t *testing.T) {
	for _, test := range []struct {
		name string
		set  func(*testing.T)
		want string
	}{
		{
			name: "missing from",
			set: func(t *testing.T) {
				t.Setenv("SMTP_HOST", "localhost")
			},
			want: "SMTP_FROM",
		},
		{
			name: "missing host",
			set: func(t *testing.T) {
				t.Setenv("SMTP_FROM", "no-reply@example.test")
			},
			want: "SMTP_HOST",
		},
		{
			name: "missing password",
			set: func(t *testing.T) {
				t.Setenv("SMTP_HOST", "localhost")
				t.Setenv("SMTP_FROM", "no-reply@example.test")
				t.Setenv("SMTP_USERNAME", "user")
			},
			want: "SMTP_PASSWORD",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			setValidEnvironment(t)
			clearSMTPEnvironment(t)
			test.set(t)

			_, err := Load()
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Load() error = %v, want %s validation error", err, test.want)
			}
		})
	}
}

func TestLoadSMTPRejectsMalformedValues(t *testing.T) {
	for _, test := range []struct {
		name string
		set  func(*testing.T)
		want string
	}{
		{
			name: "port",
			set: func(t *testing.T) {
				t.Setenv("SMTP_PORT", "not-a-port")
			},
			want: "SMTP_PORT",
		},
		{
			name: "from",
			set: func(t *testing.T) {
				t.Setenv("SMTP_HOST", "localhost")
				t.Setenv("SMTP_FROM", "sender\r\nBcc: attacker@example.test")
			},
			want: "SMTP_FROM",
		},
		{
			name: "require TLS",
			set: func(t *testing.T) {
				t.Setenv("SMTP_REQUIRE_TLS", "not-a-bool")
			},
			want: "SMTP_REQUIRE_TLS",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			setValidEnvironment(t)
			clearSMTPEnvironment(t)
			test.set(t)

			_, err := Load()
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Load() error = %v, want %s validation error", err, test.want)
			}
		})
	}
}

func TestLoadSMTPRejectsInsecureProductionTLSSetting(t *testing.T) {
	setValidEnvironment(t)
	t.Setenv("APP_ENV", "production")
	t.Setenv("JWT_SECRET", "strong-production-example-secret-for-tests-only-42")
	t.Setenv("COOKIE_SECURE", "true")
	t.Setenv("SMTP_REQUIRE_TLS", "false")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "SMTP_REQUIRE_TLS") {
		t.Fatalf("Load() error = %v, want SMTP_REQUIRE_TLS validation error", err)
	}
}

func TestLoadSMTPReadsConfiguredValues(t *testing.T) {
	setValidEnvironment(t)
	clearSMTPEnvironment(t)
	t.Setenv("SMTP_HOST", "localhost")
	t.Setenv("SMTP_PORT", "2525")
	t.Setenv("SMTP_USERNAME", "smtp-user")
	t.Setenv("SMTP_PASSWORD", "smtp-password")
	t.Setenv("SMTP_FROM", "no-reply@example.test")
	t.Setenv("SMTP_REQUIRE_TLS", "false")

	configuration, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if configuration.SMTPHost != "localhost" || configuration.SMTPPort != 2525 || configuration.SMTPUsername != "smtp-user" || configuration.SMTPPassword != "smtp-password" || configuration.SMTPFrom != "no-reply@example.test" || configuration.SMTPRequireTLS {
		t.Fatalf("SMTP configuration = %+v, want configured values", configuration)
	}
}

func clearSMTPEnvironment(t *testing.T) {
	t.Helper()
	t.Setenv("SMTP_HOST", "")
	t.Setenv("SMTP_PORT", "")
	t.Setenv("SMTP_USERNAME", "")
	t.Setenv("SMTP_PASSWORD", "")
	t.Setenv("SMTP_FROM", "")
	t.Setenv("SMTP_REQUIRE_TLS", "")
}
