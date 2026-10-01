package auth

import "errors"

var (
	ErrInvalidConfiguration = errors.New("invalid auth configuration")
	ErrInvalidRequest       = errors.New("invalid auth request")
	ErrInvalidCredentials   = errors.New("invalid credentials")
	ErrInvalidRefreshToken  = errors.New("invalid refresh token")
	ErrUnauthenticated      = errors.New("unauthenticated")
	ErrBootstrapDenied      = errors.New("bootstrap admin denied")
)
