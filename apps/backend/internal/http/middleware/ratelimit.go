package middleware

import (
	"io"
	"net"
	"net/netip"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	maxForwardedForLength = 2048
	maxForwardedForHops   = 16
)

type rateEntry struct {
	count int
	reset time.Time
}

type RateLimiter struct {
	mu                   sync.Mutex
	entries              map[string]rateEntry
	trustedProxyPrefixes []netip.Prefix
}

func NewRateLimiter() *RateLimiter {
	return newRateLimiter(nil)
}

func NewRateLimiterWithTrustedProxies(prefixes []netip.Prefix) *RateLimiter {
	trusted := make([]netip.Prefix, 0, len(prefixes))
	for _, prefix := range prefixes {
		if !prefix.IsValid() || prefix.Bits() == 0 || prefix.Addr().Is4In6() {
			continue
		}
		trusted = append(trusted, prefix.Masked())
	}
	return newRateLimiter(trusted)
}

func newRateLimiter(prefixes []netip.Prefix) *RateLimiter {
	return &RateLimiter{entries: make(map[string]rateEntry), trustedProxyPrefixes: prefixes}
}

func (limiter *RateLimiter) Limit(route string, maximum int, window time.Duration) func(http.Handler) http.Handler {
	if len(limiter.trustedProxyPrefixes) == 0 {
		return limiter.limit(route, maximum, window, remoteAddress)
	}
	return limiter.limit(route, maximum, window, limiter.trustedProxyAddress)
}

func (limiter *RateLimiter) LimitAuthenticated(route string, maximum int, window time.Duration) func(http.Handler) http.Handler {
	return limiter.limit(route, maximum, window, func(request *http.Request) string {
		principal, ok := PrincipalFromContext(request.Context())
		if !ok {
			return ""
		}
		return principal.UserID
	})
}

func remoteAddress(request *http.Request) string {
	address, _, err := net.SplitHostPort(request.RemoteAddr)
	if err != nil {
		address = request.RemoteAddr
	}
	return address
}

func (limiter *RateLimiter) trustedProxyAddress(request *http.Request) string {
	peer, ok := parseAddress(remoteAddress(request))
	if !ok || !limiter.isTrustedProxy(peer) {
		return remoteAddress(request)
	}
	forwarded, ok := parseForwardedFor(request.Header)
	if !ok {
		return peer.String()
	}
	for index := len(forwarded) - 1; index >= 0; index-- {
		if !limiter.isTrustedProxy(forwarded[index]) {
			return forwarded[index].String()
		}
	}
	return peer.String()
}

func parseAddress(value string) (netip.Addr, bool) {
	value = strings.TrimSpace(value)
	if host, _, err := net.SplitHostPort(value); err == nil {
		value = host
	}
	address, err := netip.ParseAddr(value)
	if err != nil {
		return netip.Addr{}, false
	}
	return address.Unmap(), true
}

func (limiter *RateLimiter) isTrustedProxy(address netip.Addr) bool {
	for _, prefix := range limiter.trustedProxyPrefixes {
		if prefix.Contains(address) {
			return true
		}
	}
	return false
}

func parseForwardedFor(headers http.Header) ([]netip.Addr, bool) {
	var value string
	headerCount := 0
	for name, values := range headers {
		if !strings.EqualFold(name, "X-Forwarded-For") {
			continue
		}
		if len(values) == 0 {
			return nil, false
		}
		headerCount += len(values)
		if headerCount != 1 {
			return nil, false
		}
		value = values[0]
	}
	if headerCount != 1 || len(value) > maxForwardedForLength {
		return nil, false
	}
	hops := strings.Split(value, ",")
	if len(hops) == 0 || len(hops) > maxForwardedForHops {
		return nil, false
	}
	addresses := make([]netip.Addr, 0, len(hops))
	for _, hop := range hops {
		address, ok := parseAddress(hop)
		if !ok {
			return nil, false
		}
		addresses = append(addresses, address)
	}
	return addresses, true
}

func (limiter *RateLimiter) limit(route string, maximum int, window time.Duration, identity func(*http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			address := identity(request)
			if address == "" {
				writeAuthorizationError(response, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required")
				return
			}
			key := route + ":" + address
			now := time.Now()

			limiter.mu.Lock()
			if len(limiter.entries) >= 10000 {
				for storedKey, entry := range limiter.entries {
					if !now.Before(entry.reset) {
						delete(limiter.entries, storedKey)
					}
				}
			}
			entry, exists := limiter.entries[key]
			if !exists || !now.Before(entry.reset) {
				entry = rateEntry{reset: now.Add(window)}
			}
			if entry.count >= maximum || (!exists && len(limiter.entries) >= 10000) {
				limiter.mu.Unlock()
				retrySeconds := int(time.Until(entry.reset).Seconds()) + 1
				if retrySeconds < 1 {
					retrySeconds = 1
				}
				response.Header().Set("Retry-After", strconv.Itoa(retrySeconds))
				response.Header().Set("Content-Type", "application/json; charset=utf-8")
				response.WriteHeader(http.StatusTooManyRequests)
				_, _ = io.WriteString(response, `{"error":{"code":"RATE_LIMITED","message":"Too many requests"}}`)
				return
			}
			entry.count++
			limiter.entries[key] = entry
			limiter.mu.Unlock()
			next.ServeHTTP(response, request)
		})
	}
}
