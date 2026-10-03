package auth

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type accessClaims struct {
	Role        Role  `json:"role"`
	AuthVersion int64 `json:"authVersion,omitempty"`
	jwt.RegisteredClaims
}

func (service *Service) issueAccessToken(user User, issuedAt time.Time) (string, time.Time, error) {
	expiresAt := issuedAt.Add(service.accessTTL)
	claims := accessClaims{
		Role:        user.Role,
		AuthVersion: user.AuthVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    service.issuer,
			Subject:   user.ID,
			IssuedAt:  jwt.NewNumericDate(issuedAt),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString(service.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign access token: %w", err)
	}
	return signedToken, expiresAt, nil
}

func (service *Service) parseAccessToken(bearerToken string) (accessClaims, error) {
	rawToken := strings.TrimSpace(bearerToken)
	if len(rawToken) >= len("Bearer ") && strings.EqualFold(rawToken[:len("Bearer ")], "Bearer ") {
		rawToken = strings.TrimSpace(rawToken[len("Bearer "):])
	}
	if rawToken == "" {
		return accessClaims{}, ErrUnauthenticated
	}

	claims := accessClaims{}
	parsedToken, err := jwt.ParseWithClaims(
		rawToken,
		&claims,
		func(token *jwt.Token) (any, error) {
			if token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
				return nil, errors.New("unexpected JWT signing method")
			}
			return service.secret, nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(service.issuer),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
	)
	if err != nil || parsedToken == nil || !parsedToken.Valid || claims.Subject == "" || claims.IssuedAt == nil || claims.ExpiresAt == nil || !validRole(claims.Role) {
		return accessClaims{}, ErrUnauthenticated
	}
	return claims, nil
}
