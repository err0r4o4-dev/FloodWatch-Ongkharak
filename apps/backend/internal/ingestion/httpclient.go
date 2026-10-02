package ingestion

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"syscall"
	"time"
)

var ErrResponseTooLarge = errors.New("provider response exceeds size limit")

type HTTPOptions struct {
	Timeout      time.Duration
	MaxBytes     int64
	Attempts     int
	AllowPrivate bool
}

type HTTPClient struct {
	endpoint *url.URL
	client   *http.Client
	options  HTTPOptions
}

func NewHTTPClient(endpoint string, options HTTPOptions) (*HTTPClient, error) {
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Hostname() == "" || parsed.User != nil || parsed.Fragment != "" || parsed.RawQuery != "" || parsed.Path != "" && parsed.Path != "/" || parsed.Scheme != "https" && !(options.AllowPrivate && parsed.Scheme == "http") {
		return nil, errors.New("invalid configured provider endpoint")
	}
	if address, err := netip.ParseAddr(parsed.Hostname()); err == nil && !options.AllowPrivate && !publicAddress(address) {
		return nil, errors.New("provider endpoint must be public")
	}
	if options.Timeout == 0 {
		options.Timeout = 10 * time.Second
	}
	if options.MaxBytes == 0 {
		options.MaxBytes = 1 << 20
	}
	if options.Attempts == 0 {
		options.Attempts = 3
	}
	if options.Timeout < time.Millisecond || options.Timeout > time.Minute || options.MaxBytes < 1 || options.MaxBytes > 10<<20 || options.Attempts < 1 || options.Attempts > 4 {
		return nil, errors.New("invalid provider HTTP bounds")
	}
	transport := &http.Transport{Proxy: nil, DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		if !options.AllowPrivate {
			addresses, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
			if err != nil || len(addresses) == 0 {
				return nil, errors.New("provider DNS resolution failed")
			}
			for _, candidate := range addresses {
				if !publicAddress(candidate) {
					return nil, errors.New("provider resolved to a non-public address")
				}
			}
			host = addresses[0].String()
		}
		return (&net.Dialer{Timeout: options.Timeout}).DialContext(ctx, network, net.JoinHostPort(host, port))
	}}
	return &HTTPClient{endpoint: parsed, options: options, client: &http.Client{
		Timeout: options.Timeout, Transport: transport,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}, nil
}

func publicAddress(address netip.Addr) bool {
	address = address.Unmap()
	if !address.IsValid() || !address.IsGlobalUnicast() || address.IsPrivate() || address.IsLoopback() || address.IsLinkLocalUnicast() {
		return false
	}
	if address.Is4() {
		for _, prefix := range []string{"0.0.0.0/8", "100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "192.88.99.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "224.0.0.0/4", "240.0.0.0/4"} {
			if netip.MustParsePrefix(prefix).Contains(address) {
				return false
			}
		}
		return true
	}
	if !netip.MustParsePrefix("2000::/3").Contains(address) {
		return false
	}
	for _, prefix := range []string{"2001::/23", "2001:db8::/32", "2002::/16", "3fff::/20"} {
		if netip.MustParsePrefix(prefix).Contains(address) {
			return false
		}
	}
	return true
}

func (client *HTTPClient) Fetch(ctx context.Context, path string) ([]byte, error) {
	if !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") {
		return nil, errors.New("provider path must be relative to configured origin")
	}
	requestURL := *client.endpoint
	requestURL.Path = strings.SplitN(path, "?", 2)[0]
	if parts := strings.SplitN(path, "?", 2); len(parts) == 2 {
		requestURL.RawQuery = parts[1]
	}
	for attempt := 0; attempt < client.options.Attempts; attempt++ {
		retryDelay := time.Duration(100<<attempt)*time.Millisecond + time.Duration(rand.IntN(50))*time.Millisecond
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL.String(), nil)
		if err != nil {
			return nil, err
		}
		request.Header.Set("User-Agent", "FloodWatch-Ongkharak-Ingestion/1")
		response, err := client.client.Do(request)
		if err == nil {
			if response.StatusCode == http.StatusOK {
				body, readErr := io.ReadAll(io.LimitReader(response.Body, client.options.MaxBytes+1))
				_ = response.Body.Close()
				if readErr != nil {
					return nil, errors.New("provider response read failed")
				}
				if int64(len(body)) > client.options.MaxBytes {
					return nil, ErrResponseTooLarge
				}
				return body, nil
			}
			_ = response.Body.Close()
			if !retryableStatus(response.StatusCode) || attempt == client.options.Attempts-1 {
				return nil, fmt.Errorf("provider HTTP status %s", strconv.Itoa(response.StatusCode))
			}
			if response.StatusCode == http.StatusTooManyRequests {
				if value := response.Header.Get("Retry-After"); value != "" {
					if seconds, parseErr := strconv.Atoi(value); parseErr == nil && seconds >= 0 {
						if seconds > 2 {
							return nil, errors.New("provider rate limit exceeds retry budget")
						}
						retryDelay = max(retryDelay, time.Duration(seconds)*time.Second)
					} else if at, parseErr := http.ParseTime(value); parseErr == nil {
						if at.After(time.Now().Add(2 * time.Second)) {
							return nil, errors.New("provider rate limit exceeds retry budget")
						}
						retryDelay = max(retryDelay, time.Until(at))
					}
					if retryDelay > 2*time.Second {
						return nil, errors.New("provider rate limit exceeds retry budget")
					}
				}
			}
		} else if attempt == client.options.Attempts-1 || ctx.Err() != nil || !retryableNetworkError(err) {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return nil, errors.New("provider connection failed")
		}
		pause := time.NewTimer(retryDelay)
		select {
		case <-ctx.Done():
			pause.Stop()
			return nil, ctx.Err()
		case <-pause.C:
		}
	}
	return nil, errors.New("provider retry limit reached")
}

func retryableNetworkError(err error) bool {
	var networkError net.Error
	return errors.As(err, &networkError) && (networkError.Timeout() || networkError.Temporary()) || errors.Is(err, syscall.ECONNRESET) || errors.Is(err, io.EOF)
}

func retryableStatus(status int) bool {
	return status == http.StatusTooManyRequests || status == http.StatusBadGateway || status == http.StatusServiceUnavailable || status == http.StatusGatewayTimeout
}
