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
	settings := googleSettings{clientID: strings.TrimSpace(os.Getenv("GOOGLE_OAUTH_CLIENT_ID")), clientSecret: strings.TrimSpace(os.Getenv("GOOGLE_OAUTH_CLIENT_SECRET")), redirectURI: strings.TrimSpace(os.Getenv("GOOGLE_OAUTH_REDIRECT_URL"))}
	if settings == (googleSettings{}) {
		return settings, nil
	}
	invalid := errors.New("Google login requires complete, valid GOOGLE_OAUTH_CLIENT_ID, GOOGLE_OAUTH_CLIENT_SECRET and GOOGLE_OAUTH_REDIRECT_URL settings")
	if settings.clientID == "" || settings.clientSecret == "" || settings.redirectURI == "" || len(settings.clientID) > 512 || len(settings.clientSecret) > 4096 || len(settings.redirectURI) > 2048 {
		return googleSettings{}, invalid
	}
	callback, err := url.Parse(settings.redirectURI)
	if err != nil || !safeGoogleURL(callback, environment) || callback.Path != "/api/v1/auth/google/callback" || callback.RawPath != "" {
		return googleSettings{}, invalid
	}
	settings.frontendOrigin = callback.Scheme + "://" + callback.Host
	for _, origin := range allowedOrigins {
		if origin == settings.frontendOrigin {
			return settings, nil
		}
	}
	if len(allowedOrigins) != 1 {
		return googleSettings{}, invalid
	}
	frontend, err := url.Parse(allowedOrigins[0])
	if err != nil || !safeGoogleURL(frontend, environment) || frontend.Path != "" || frontend.RawPath != "" || frontend.Scheme != callback.Scheme || frontend.Hostname() != callback.Hostname() {
		return googleSettings{}, invalid
	}
	settings.frontendOrigin = allowedOrigins[0]
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
