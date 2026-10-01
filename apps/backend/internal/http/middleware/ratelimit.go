package middleware

import (
	"io"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type rateEntry struct {
	count int
	reset time.Time
}

type RateLimiter struct {
	mu      sync.Mutex
	entries map[string]rateEntry
}

func NewRateLimiter() *RateLimiter {
	return &RateLimiter{entries: make(map[string]rateEntry)}
}

func (limiter *RateLimiter) Limit(route string, maximum int, window time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			address, _, err := net.SplitHostPort(request.RemoteAddr)
			if err != nil {
				address = request.RemoteAddr
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
