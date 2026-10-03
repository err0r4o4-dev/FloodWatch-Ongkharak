package config

import (
	"errors"
	"net"
	"net/url"
	"os"
	"strings"
)

type googleSettings struct{ clientID, clientSecret, redirectURI, frontendOrigin string }

func loadGoogle(environment string, allowedOrigins []string) (googleSettings, error) {
	settings := googleSettings{clientID: strings.TrimSpace(os.Getenv("GOOGLE_CLIENT_ID")), clientSecret: strings.TrimSpace(os.Getenv("GOOGLE_CLIENT_SECRET")), redirectURI: strings.TrimSpace(os.Getenv("GOOGLE_REDIRECT_URI")), frontendOrigin: strings.TrimSpace(os.Getenv("AUTH_FRONTEND_ORIGIN"))}
	if settings == (googleSettings{}) {
		return settings, nil
	}
	invalid := errors.New("Google login requires complete, valid GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET, GOOGLE_REDIRECT_URI and AUTH_FRONTEND_ORIGIN settings")
	if settings.clientID == "" || settings.clientSecret == "" || settings.redirectURI == "" || settings.frontendOrigin == "" || len(settings.clientID) > 512 || len(settings.clientSecret) > 4096 || len(settings.redirectURI) > 2048 || len(settings.frontendOrigin) > 2048 {
		return googleSettings{}, invalid
	}
	frontend, err := url.Parse(settings.frontendOrigin)
	if err != nil || !safeGoogleURL(frontend, environment) || frontend.Path != "" || frontend.RawPath != "" {
		return googleSettings{}, invalid
	}
	approved := false
	for _, origin := range allowedOrigins {
		if origin == settings.frontendOrigin {
			approved = true
		}
	}
	if !approved {
		return googleSettings{}, invalid
	}
	callback, err := url.Parse(settings.redirectURI)
	if err != nil || !safeGoogleURL(callback, environment) || callback.Path != "/api/v1/auth/google/callback" || callback.RawPath != "" || callback.Scheme+"://"+callback.Host != settings.frontendOrigin {
		return googleSettings{}, invalid
	}
	return settings, nil
}
func safeGoogleURL(location *url.URL, environment string) bool {
	if location.Host == "" || location.User != nil || location.RawQuery != "" || location.ForceQuery || location.Fragment != "" || location.Opaque != "" {
		return false
	}
	if location.Scheme == "https" {
		return true
	}
	if location.Scheme != "http" || environment == "production" {
		return false
	}
	host := location.Hostname()
	ip := net.ParseIP(host)
	return host == "localhost" || ip != nil && ip.IsLoopback()
}
