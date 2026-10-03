package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/floodwatch-ongkharak/backend/internal/mailer"
)

const defaultSMTPPort = 587

func loadSMTP(environment string) (mailer.Config, error) {
	host := strings.TrimSpace(os.Getenv("SMTP_HOST"))
	portValue := strings.TrimSpace(os.Getenv("SMTP_PORT"))
	username := strings.TrimSpace(os.Getenv("SMTP_USERNAME"))
	password := os.Getenv("SMTP_PASSWORD")
	from := strings.TrimSpace(os.Getenv("SMTP_FROM"))
	requireTLSValue := strings.TrimSpace(os.Getenv("SMTP_REQUIRE_TLS"))

	port := defaultSMTPPort
	if portValue != "" {
		parsed, err := strconv.Atoi(portValue)
		if err != nil || parsed < 1 || parsed > 65535 {
			return mailer.Config{}, fmt.Errorf("SMTP_PORT must be a valid TCP port")
		}
		port = parsed
	}
	requireTLS := true
	if requireTLSValue != "" {
		parsed, err := strconv.ParseBool(requireTLSValue)
		if err != nil {
			return mailer.Config{}, fmt.Errorf("SMTP_REQUIRE_TLS must be true or false")
		}
		requireTLS = parsed
	}
	if environment == "production" && !requireTLS {
		return mailer.Config{}, fmt.Errorf("SMTP_REQUIRE_TLS must be true in production")
	}
	if host == "" && from == "" && username == "" && password == "" {
		return mailer.Config{Port: port, RequireTLS: requireTLS}, nil
	}
	if host == "" {
		return mailer.Config{}, fmt.Errorf("SMTP_HOST is required when SMTP is configured")
	}
	if from == "" {
		return mailer.Config{}, fmt.Errorf("SMTP_FROM is required when SMTP is configured")
	}
	if (username == "") != (password == "") {
		return mailer.Config{}, fmt.Errorf("SMTP_USERNAME and SMTP_PASSWORD must be configured together")
	}
	configuration := mailer.Config{
		Host:       host,
		Port:       port,
		Username:   username,
		Password:   password,
		From:       from,
		RequireTLS: requireTLS,
	}
	if _, err := mailer.New(configuration); err != nil {
		return mailer.Config{}, err
	}
	return configuration, nil
}
