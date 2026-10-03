package googleauth

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/mail"
	"net/url"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/floodwatch-ongkharak/backend/internal/auth"
	"golang.org/x/oauth2"
)

type Config struct{ ClientID, ClientSecret, RedirectURI string }
type endpoints struct{ issuer, authorization, token, keys string }
type Provider struct {
	oauth  oauth2.Config
	issuer string
	keys   oidc.KeySet
	client *http.Client
}

func New(ctx context.Context, configuration Config) (*Provider, error) {
	return newProvider(ctx, configuration, endpoints{issuer: "https://accounts.google.com", authorization: "https://accounts.google.com/o/oauth2/v2/auth", token: "https://oauth2.googleapis.com/token", keys: "https://www.googleapis.com/oauth2/v3/certs"})
}

func newProvider(ctx context.Context, configuration Config, locations endpoints) (*Provider, error) {
	if configuration.ClientID == "" || configuration.ClientSecret == "" || configuration.RedirectURI == "" {
		return nil, auth.ErrInvalidConfiguration
	}
	client := &http.Client{Timeout: 10 * time.Second, Transport: boundedTransport{base: http.DefaultTransport}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	keyContext := oidc.ClientContext(ctx, client)
	keys := oidc.NewRemoteKeySet(keyContext, locations.keys)
	return &Provider{
		oauth:  oauth2.Config{ClientID: configuration.ClientID, ClientSecret: configuration.ClientSecret, RedirectURL: configuration.RedirectURI, Scopes: []string{oidc.ScopeOpenID, "email"}, Endpoint: oauth2.Endpoint{AuthURL: locations.authorization, TokenURL: locations.token, AuthStyle: oauth2.AuthStyleInParams}},
		issuer: locations.issuer, keys: keys, client: client,
	}, nil
}

func (provider *Provider) AuthorizationURL(state, nonce, verifier string) string {
	return provider.oauth.AuthCodeURL(state, oauth2.SetAuthURLParam("nonce", nonce), oauth2.S256ChallengeOption(verifier))
}

func (provider *Provider) Verify(ctx context.Context, code, verifier, nonce string) (auth.GoogleIdentity, error) {
	if code == "" || len(code) > 4096 || verifier == "" || nonce == "" {
		return auth.GoogleIdentity{}, auth.ErrGoogleIdentity
	}
	ctx = context.WithValue(ctx, oauth2.HTTPClient, provider.client)
	token, err := provider.oauth.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		var exchangeError *oauth2.RetrieveError
		if errors.As(err, &exchangeError) && (exchangeError.ErrorCode == "invalid_grant" || exchangeError.ErrorCode == "invalid_request") {
			return auth.GoogleIdentity{}, auth.ErrGoogleIdentity
		}
		return auth.GoogleIdentity{}, auth.ErrGoogleProvider
	}
	rawToken, ok := token.Extra("id_token").(string)
	if !ok || rawToken == "" || len(rawToken) > 16384 {
		return auth.GoogleIdentity{}, auth.ErrGoogleIdentity
	}
	observedKeys := &signatureObservation{keys: provider.keys}
	idToken, err := oidc.NewVerifier(provider.issuer, observedKeys, &oidc.Config{ClientID: provider.oauth.ClientID, SupportedSigningAlgs: []string{oidc.RS256}}).Verify(ctx, rawToken)
	if err != nil {
		var networkError *url.Error
		if errors.Is(observedKeys.err, auth.ErrGoogleProvider) || errors.As(observedKeys.err, &networkError) || errors.Is(observedKeys.err, context.DeadlineExceeded) {
			return auth.GoogleIdentity{}, auth.ErrGoogleProvider
		}
		return auth.GoogleIdentity{}, auth.ErrGoogleIdentity
	}
	var claims struct {
		Email           string `json:"email"`
		EmailVerified   bool   `json:"email_verified"`
		AuthorizedParty string `json:"azp"`
	}
	if idToken.Claims(&claims) != nil || idToken.Nonce != nonce || !claims.EmailVerified || idToken.IssuedAt.IsZero() || idToken.IssuedAt.After(time.Now().Add(time.Minute)) || len(idToken.Audience) > 1 && claims.AuthorizedParty == "" || claims.AuthorizedParty != "" && claims.AuthorizedParty != provider.oauth.ClientID {
		return auth.GoogleIdentity{}, auth.ErrGoogleIdentity
	}
	address, err := mail.ParseAddress(claims.Email)
	if err != nil || address.Address != claims.Email || len(claims.Email) > 254 || !strings.Contains(strings.SplitN(claims.Email, "@", 2)[1], ".") {
		return auth.GoogleIdentity{}, auth.ErrGoogleIdentity
	}
	if idToken.Subject == "" || len(idToken.Subject) > 255 {
		return auth.GoogleIdentity{}, auth.ErrGoogleIdentity
	}
	for _, character := range idToken.Subject {
		if character < 33 || character > 126 {
			return auth.GoogleIdentity{}, auth.ErrGoogleIdentity
		}
	}
	return auth.GoogleIdentity{Subject: idToken.Subject, Email: strings.ToLower(claims.Email)}, nil
}

type boundedTransport struct{ base http.RoundTripper }

type signatureObservation struct {
	keys oidc.KeySet
	err  error
}

func (observation *signatureObservation) VerifySignature(ctx context.Context, token string) ([]byte, error) {
	payload, err := observation.keys.VerifySignature(ctx, token)
	observation.err = err
	return payload, err
}

func (transport boundedTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	response, err := transport.base.RoundTrip(request)
	if err != nil {
		return nil, err
	}
	if request.Method == http.MethodGet && response.StatusCode != http.StatusOK {
		response.Body.Close()
		return nil, auth.ErrGoogleProvider
	}
	response.Body = &boundedBody{Reader: io.LimitReader(response.Body, 1<<20), Closer: response.Body}
	return response, nil
}

type boundedBody struct {
	io.Reader
	io.Closer
}
