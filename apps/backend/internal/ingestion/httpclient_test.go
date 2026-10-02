package ingestion

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"
)

func TestHTTPClientBoundsAndRetries(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		attempts++
		switch request.URL.Path {
		case "/retry":
			if attempts < 3 {
				response.WriteHeader(http.StatusServiceUnavailable)
				return
			}
		case "/too-big":
			_, _ = response.Write([]byte(strings.Repeat("x", 32)))
			return
		case "/slow":
			<-request.Context().Done()
			return
		case "/bad":
			response.WriteHeader(http.StatusUnauthorized)
			return
		case "/rate":
			if attempts == 1 {
				response.Header().Set("Retry-After", "0")
				response.WriteHeader(http.StatusTooManyRequests)
				return
			}
		case "/throttle":
			response.Header().Set("Retry-After", "60")
			response.WriteHeader(http.StatusTooManyRequests)
			return
		case "/throttle-overflow":
			response.Header().Set("Retry-After", "9223372036854775807")
			response.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = response.Write([]byte("ok"))
	}))
	defer server.Close()
	client, err := NewHTTPClient(server.URL, HTTPOptions{Timeout: 50 * time.Millisecond, MaxBytes: 8, Attempts: 3, AllowPrivate: true})
	if err != nil {
		t.Fatal(err)
	}
	body, err := client.Fetch(context.Background(), "/retry")
	if err != nil || string(body) != "ok" || attempts != 3 {
		t.Fatalf("retry: %s %v attempts=%d", body, err, attempts)
	}
	if _, err := client.Fetch(context.Background(), "/too-big"); !errors.Is(err, ErrResponseTooLarge) {
		t.Fatalf("size limit: %v", err)
	}
	attempts = 0
	if _, err := client.Fetch(context.Background(), "/rate"); err != nil || attempts != 2 {
		t.Fatalf("429 retry: %v attempts=%d", err, attempts)
	}
	attempts = 0
	if _, err := client.Fetch(context.Background(), "/throttle"); err == nil || attempts != 1 {
		t.Fatalf("Retry-After ignored: %v attempts=%d", err, attempts)
	}
	attempts = 0
	if _, err := client.Fetch(context.Background(), "/throttle-overflow"); err == nil || attempts != 1 {
		t.Fatalf("large Retry-After ignored: %v attempts=%d", err, attempts)
	}
	attempts = 0
	if _, err := client.Fetch(context.Background(), "/bad"); err == nil || attempts != 1 {
		t.Fatalf("permanent status retried: %v attempts=%d", err, attempts)
	}
	if _, err := client.Fetch(context.Background(), "/slow"); err == nil {
		t.Fatal("expected timeout")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := client.Fetch(ctx, "/retry"); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
}

func TestHTTPClientRejectsUnsafeEndpoints(t *testing.T) {
	for _, endpoint := range []string{"http://example.com", "https://127.0.0.1", "https://[::ffff:127.0.0.1]", "https://user:pass@example.com", "https://example.com/#fragment"} {
		if _, err := NewHTTPClient(endpoint, HTTPOptions{}); err == nil {
			t.Errorf("accepted unsafe endpoint %q", endpoint)
		}
	}
}

func TestPublicAddressExcludesSpecialUse(t *testing.T) {
	for _, address := range []string{"198.18.0.1", "198.51.100.1", "203.0.113.1", "192.0.2.1", "192.0.0.1", "192.88.99.1", "100.64.0.1", "fec0::1", "2001:db8::1", "2001::1", "2002::1", "3fff::1", "::ffff:198.18.0.1"} {
		if publicAddress(netip.MustParseAddr(address)) {
			t.Errorf("special-use address accepted: %s", address)
		}
	}
	for _, address := range []string{"8.8.8.8", "1.1.1.1", "2606:4700:4700::1111"} {
		if !publicAddress(netip.MustParseAddr(address)) {
			t.Errorf("public address rejected: %s", address)
		}
	}
}
