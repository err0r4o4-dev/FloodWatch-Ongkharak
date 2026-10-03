package googleauth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/auth"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/oauth2"
)

func TestProviderAuthorizationUsesStateNonceAndPKCE(t *testing.T) {
	provider, err := New(context.Background(), Config{ClientID: "test-client", ClientSecret: "test-only-secret", RedirectURI: "http://localhost/api/v1/auth/google/callback"})
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := url.Parse(provider.AuthorizationURL("state", "nonce", "verifier"))
	query := parsed.Query()
	if parsed.Host != "accounts.google.com" || query.Get("state") != "state" || query.Get("nonce") != "nonce" || query.Get("code_challenge_method") != "S256" || query.Get("code_challenge") != oauth2.S256ChallengeFromVerifier("verifier") || query.Get("scope") != "openid email" || query.Get("access_type") == "offline" {
		t.Fatalf("unsafe authorization parameters: %v", query)
	}
}

func TestProviderValidatesSignedIdentityAndExchange(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	var idToken string
	var keysUnavailable bool
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		if request.URL.Path == "/keys" {
			if keysUnavailable {
				response.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			json.NewEncoder(response).Encode(map[string]any{"keys": []any{map[string]any{"kty": "RSA", "kid": "test-key", "alg": "RS256", "use": "sig", "n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()), "e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes())}}})
			return
		}
		if err := request.ParseForm(); err != nil {
			t.Error(err)
		}
		if request.Form.Get("code_verifier") != "test-verifier" || request.Form.Get("code") != "test-code" || request.Form.Get("redirect_uri") != "http://localhost/callback" {
			t.Error("exchange omitted binding")
		}
		json.NewEncoder(response).Encode(map[string]any{"access_token": "fixture-access", "token_type": "Bearer", "id_token": idToken})
	}))
	defer server.Close()
	provider, err := newProvider(context.Background(), Config{ClientID: "test-client", ClientSecret: "test-only-secret", RedirectURI: "http://localhost/callback"}, endpoints{issuer: server.URL, authorization: server.URL + "/authorize", token: server.URL + "/token", keys: server.URL + "/keys"})
	if err != nil {
		t.Fatal(err)
	}
	valid := func() jwt.MapClaims {
		return jwt.MapClaims{"iss": server.URL, "aud": "test-client", "sub": "stable-subject", "email": "person@example.test", "email_verified": true, "nonce": "test-nonce", "iat": time.Now().Unix(), "exp": time.Now().Add(time.Hour).Unix()}
	}
	for _, test := range []struct {
		name   string
		mutate func(jwt.MapClaims)
		good   bool
	}{
		{name: "valid", good: true, mutate: func(jwt.MapClaims) {}},
		{name: "wrong audience", mutate: func(claims jwt.MapClaims) { claims["aud"] = "another-client" }},
		{name: "wrong issuer", mutate: func(claims jwt.MapClaims) { claims["iss"] = "https://evil.test" }},
		{name: "expired", mutate: func(claims jwt.MapClaims) { claims["exp"] = time.Now().Add(-time.Hour).Unix() }},
		{name: "wrong nonce", mutate: func(claims jwt.MapClaims) { claims["nonce"] = "other-nonce" }},
		{name: "unverified email", mutate: func(claims jwt.MapClaims) { claims["email_verified"] = false }},
		{name: "malformed email", mutate: func(claims jwt.MapClaims) { claims["email"] = "not-an-email" }},
		{name: "missing subject", mutate: func(claims jwt.MapClaims) { delete(claims, "sub") }},
		{name: "wrong azp", mutate: func(claims jwt.MapClaims) { claims["azp"] = "other-client" }},
		{name: "future issued", mutate: func(claims jwt.MapClaims) { claims["iat"] = time.Now().Add(time.Hour).Unix() }},
	} {
		t.Run(test.name, func(t *testing.T) {
			claims := valid()
			test.mutate(claims)
			token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
			token.Header["kid"] = "test-key"
			idToken, err = token.SignedString(key)
			if err != nil {
				t.Fatal(err)
			}
			identity, err := provider.Verify(context.Background(), "test-code", "test-verifier", "test-nonce")
			if test.good {
				if err != nil || identity.Subject != "stable-subject" {
					t.Fatalf("verified identity: %+v %v", identity, err)
				}
			} else if !errors.Is(err, auth.ErrGoogleIdentity) {
				t.Fatalf("invalid claims accepted or wrong error: %v", err)
			}
		})
	}
	idToken = "not-a-jwt"
	if _, err := provider.Verify(context.Background(), "test-code", "test-verifier", "test-nonce"); !errors.Is(err, auth.ErrGoogleIdentity) {
		t.Fatal("malformed JWT accepted")
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, valid())
	token.Header["kid"] = "test-key"
	idToken, err = token.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	keysUnavailable = true
	freshProvider, err := newProvider(context.Background(), Config{ClientID: "test-client", ClientSecret: "test-only-secret", RedirectURI: "http://localhost/callback"}, endpoints{issuer: server.URL, authorization: server.URL + "/authorize", token: server.URL + "/token", keys: server.URL + "/keys"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := freshProvider.Verify(context.Background(), "test-code", "test-verifier", "test-nonce"); !errors.Is(err, auth.ErrGoogleProvider) {
		t.Fatalf("key provider outage disguised as invalid credentials: %v", err)
	}
}

func TestProviderDoesNotExposeExchangeErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.WriteHeader(503)
		response.Write([]byte(strings.Repeat("private-provider-error", 100)))
	}))
	defer server.Close()
	provider, _ := newProvider(context.Background(), Config{ClientID: "test-client", ClientSecret: "test-only-secret", RedirectURI: "http://localhost/callback"}, endpoints{issuer: server.URL, authorization: server.URL, token: server.URL, keys: server.URL})
	if _, err := provider.Verify(context.Background(), "code", "verifier", "nonce"); !errors.Is(err, auth.ErrGoogleProvider) || strings.Contains(err.Error(), "private") {
		t.Fatalf("unsafe exchange error: %v", err)
	}
}
