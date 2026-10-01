package auth

import (
	"context"
	"errors"
	"time"
)

type Role string

const (
	RoleUser  Role = "USER"
	RoleAdmin Role = "ADMIN"
)

var (
	ErrNotFound        = errors.New("account not found")
	ErrEmailExists     = errors.New("email already exists")
	ErrSessionInvalid  = errors.New("refresh session invalid")
	ErrSessionExpired  = errors.New("refresh session expired")
	ErrAccountDisabled = errors.New("account disabled")
)

type User struct {
	ID           string
	Email        string
	PasswordHash string
	Role         Role
	CreatedAt    time.Time
	UpdatedAt    time.Time
	DisabledAt   *time.Time
}

type RefreshSession struct {
	ID        string
	UserID    string
	TokenHash []byte
	ExpiresAt time.Time
}

type Repository interface {
	CreateUser(context.Context, User) (User, error)
	FindUserByEmail(context.Context, string) (User, error)
	FindUserByID(context.Context, string) (User, error)
	CreateRefreshSession(context.Context, RefreshSession) error
	RotateRefreshSession(context.Context, []byte, RefreshSession, time.Time) (User, error)
	RevokeRefreshSession(context.Context, []byte, time.Time) error
}
