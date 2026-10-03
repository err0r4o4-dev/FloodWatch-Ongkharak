package middleware

import (
	"context"
	"net/netip"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestLimiterRejectsExcessRequestsPerRemoteAddress(t *testing.T) {
	limiter := NewRateLimiter()
	handler := limiter.Limit("login", 2, time.Minute)(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusNoContent)
	}))

	for attempt := 1; attempt <= 3; attempt++ {
		request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
		request.RemoteAddr = "192.0.2.10:1234"
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if attempt < 3 && response.Code != http.StatusNoContent {
			t.Fatalf("attempt %d = %d, want 204", attempt, response.Code)
		}
		if attempt == 3 && (response.Code != http.StatusTooManyRequests || response.Header().Get("Retry-After") == "") {
			t.Fatalf("limited attempt = %d with Retry-After %q", response.Code, response.Header().Get("Retry-After"))
		}
	}

	otherRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	otherRequest.RemoteAddr = "192.0.2.11:1234"
	otherResponse := httptest.NewRecorder()
	handler.ServeHTTP(otherResponse, otherRequest)
	if otherResponse.Code != http.StatusNoContent {
		t.Fatalf("other address = %d, want 204", otherResponse.Code)
	}
}

func TestLimiterSeparatesClientsBehindTrustedProxy(t *testing.T) {
	limiter := NewRateLimiterWithTrustedProxies([]netip.Prefix{netip.MustParsePrefix("192.0.2.0/24")})
	handler := limiter.Limit("request-otp", 5, time.Hour)(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusNoContent)
	}))

	for _, client := range []string{"203.0.113.10", "203.0.113.11"} {
		for attempt := 1; attempt <= 6; attempt++ {
			request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/password/request-otp", nil)
			request.RemoteAddr = "192.0.2.10:1234"
			request.Header.Set("X-Forwarded-For", client+", 192.0.2.10")
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			want := http.StatusNoContent
			if attempt == 6 {
				want = http.StatusTooManyRequests
			}
			if response.Code != want {
				t.Fatalf("client %s attempt %d = %d, want %d", client, attempt, response.Code, want)
			}
		}
	}
}

func TestTrustedProxyLimiterIgnoresSpoofedDirectForwarding(t *testing.T) {
	limiter := NewRateLimiterWithTrustedProxies([]netip.Prefix{netip.MustParsePrefix("192.0.2.0/24")})
	handler := limiter.Limit("request-otp", 1, time.Hour)(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusNoContent)
	}))

	for _, forwarded := range []string{"203.0.113.10", "203.0.113.11"} {
		request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/password/request-otp", nil)
		request.RemoteAddr = "198.51.100.10:1234"
		request.Header.Set("X-Forwarded-For", forwarded)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		want := http.StatusNoContent
		if forwarded == "203.0.113.11" {
			want = http.StatusTooManyRequests
		}
		if response.Code != want {
			t.Fatalf("forwarded %s = %d, want %d", forwarded, response.Code, want)
		}
	}
}

func TestTrustedProxyLimiterUsesNearestUntrustedHop(t *testing.T) {
	limiter := NewRateLimiterWithTrustedProxies([]netip.Prefix{netip.MustParsePrefix("192.0.2.0/24")})
	handler := limiter.Limit("request-otp", 1, time.Hour)(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusNoContent)
	}))

	for _, forwarded := range []string{
		"198.51.100.1, 203.0.113.10, 192.0.2.10",
		"198.51.100.2, 203.0.113.10, 192.0.2.10",
	} {
		request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/password/request-otp", nil)
		request.RemoteAddr = "192.0.2.10:1234"
		request.Header.Set("X-Forwarded-For", forwarded)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		want := http.StatusNoContent
		if strings.Contains(forwarded, "198.51.100.2") {
			want = http.StatusTooManyRequests
		}
		if response.Code != want {
			t.Fatalf("forwarded %q = %d, want %d", forwarded, response.Code, want)
		}
	}
}

func TestTrustedProxyLimiterFallsBackForMalformedOrDuplicateForwarding(t *testing.T) {
	for _, test := range []struct {
		name   string
		set    func(*http.Request)
		second func(*http.Request)
	}{
		{name: "malformed", set: func(request *http.Request) { request.Header.Set("X-Forwarded-For", "not-an-ip") }, second: func(request *http.Request) { request.Header.Set("X-Forwarded-For", "still-not-an-ip") }},
		{name: "duplicate", set: func(request *http.Request) { request.Header.Add("X-Forwarded-For", "203.0.113.10"); request.Header.Add("X-Forwarded-For", "203.0.113.11") }, second: func(request *http.Request) { request.Header.Add("X-Forwarded-For", "203.0.113.12"); request.Header.Add("X-Forwarded-For", "203.0.113.13") }},
	} {
		t.Run(test.name, func(t *testing.T) {
			limiter := NewRateLimiterWithTrustedProxies([]netip.Prefix{netip.MustParsePrefix("192.0.2.0/24")})
			handler := limiter.Limit("request-otp", 1, time.Hour)(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
				response.WriteHeader(http.StatusNoContent)
			}))
			request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/password/request-otp", nil)
			request.RemoteAddr = "192.0.2.10:1234"
			test.set(request)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusNoContent {
				t.Fatalf("first request = %d, want 204", response.Code)
			}

			second := httptest.NewRequest(http.MethodPost, "/api/v1/auth/password/request-otp", nil)
			second.RemoteAddr = "192.0.2.10:1234"
			test.second(second)
			secondResponse := httptest.NewRecorder()
			handler.ServeHTTP(secondResponse, second)
			if secondResponse.Code != http.StatusTooManyRequests {
				t.Fatalf("fallback request = %d, want 429", secondResponse.Code)
			}
		})
	}
}

func TestTrustedProxyLimiterSupportsIPv6AndBoundsForwarding(t *testing.T) {
	limiter := NewRateLimiterWithTrustedProxies([]netip.Prefix{netip.MustParsePrefix("2001:db8:1::/48")})
	handler := limiter.Limit("request-otp", 1, time.Hour)(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusNoContent)
	}))
	for _, client := range []string{"2001:db8:2::1", "2001:db8:2::2"} {
		request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/password/request-otp", nil)
		request.RemoteAddr = "[2001:db8:1::10]:1234"
		request.Header.Set("X-Forwarded-For", client+", 2001:db8:1::10")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusNoContent {
			t.Fatalf("IPv6 client %s = %d, want 204", client, response.Code)
		}
	}

	for _, forwarded := range []string{
		strings.Repeat("1", maxForwardedForLength+1),
		"2001:db8:2::1, " + strings.Repeat("2001:db8:2::2, ", maxForwardedForHops),
	} {
		if forwarded == strings.Repeat("1", maxForwardedForLength+1) {
			peer := httptest.NewRequest(http.MethodPost, "/api/v1/auth/password/request-otp", nil)
			peer.RemoteAddr = "[2001:db8:1::10]:1234"
			peer.Header.Set("X-Forwarded-For", "not-an-ip")
			peerResponse := httptest.NewRecorder()
			handler.ServeHTTP(peerResponse, peer)
			if peerResponse.Code != http.StatusNoContent {
				t.Fatalf("peer fallback seed = %d, want 204", peerResponse.Code)
			}
		}
		request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/password/request-otp", nil)
		request.RemoteAddr = "[2001:db8:1::10]:1234"
		request.Header.Set("X-Forwarded-For", forwarded)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusTooManyRequests {
			t.Fatalf("bounded fallback = %d, want 429", response.Code)
		}
	}
}

func TestAuthenticatedLimiterSeparatesUsersBehindProxy(t *testing.T) {
	limiter := NewRateLimiter()
	handler := limiter.LimitAuthenticated("report-create", 1, time.Hour)(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusNoContent)
	}))
	for _, test := range []struct {
		userID string
		want   int
	}{
		{"user-one", http.StatusNoContent},
		{"user-one", http.StatusTooManyRequests},
		{"user-two", http.StatusNoContent},
	} {
		request := httptest.NewRequest(http.MethodPost, "/api/v1/reports", nil)
		request.RemoteAddr = "172.18.0.2:1234"
		request.Header.Set("X-Forwarded-For", "203.0.113.1")
		request = request.WithContext(context.WithValue(request.Context(), principalKey{}, Principal{UserID: test.userID}))
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != test.want {
			t.Fatalf("user %s = %d, want %d", test.userID, response.Code, test.want)
		}
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/reports", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("missing principal = %d, want 401", response.Code)
	}
}
