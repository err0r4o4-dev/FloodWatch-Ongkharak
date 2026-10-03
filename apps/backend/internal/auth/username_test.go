package auth

import (
	"context"
	"errors"
	"testing"
)

func TestUsernameRegistrationAndLogin(t *testing.T) {
	repository := newFakeRepository()
	service := newTestService(t, repository)
	user, err := service.RegisterWithUsername(context.Background(), "person@example.test", "  Person_Name  ", "correct horse battery staple")
	if err != nil || user.Username != "person_name" {
		t.Fatalf("register username = %q, error = %v", user.Username, err)
	}
	for _, identifier := range []string{"person@example.test", " PERSON_NAME "} {
		result, err := service.Login(context.Background(), identifier, "correct horse battery staple")
		if err != nil || result.User.ID != user.ID {
			t.Fatalf("login by %q: %v", identifier, err)
		}
	}
	_, err = service.RegisterWithUsername(context.Background(), "another@example.test", "PERSON_NAME", "correct horse battery staple")
	if !errors.Is(err, ErrUsernameExists) {
		t.Fatalf("duplicate username error = %v", err)
	}
	for _, username := range []string{"", "ab", "has space", "name@example.test", "ชื่อผู้ใช้", "abcdefghijklmnopqrstuvwxyz1234567"} {
		if _, err := service.RegisterWithUsername(context.Background(), "valid@example.test", username, "correct horse battery staple"); !errors.Is(err, ErrInvalidRequest) {
			t.Fatalf("username %q error = %v", username, err)
		}
	}
	for _, identifier := range []string{"unknown_name", "bad username"} {
		if _, err := service.Login(context.Background(), identifier, "correct horse battery staple"); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("login %q error = %v", identifier, err)
		}
	}
}

func TestAuthenticationRejectsPriorPasswordVersion(t *testing.T) {
	repository := newFakeRepository()
	service := newTestService(t, repository)
	user, err := service.Register(context.Background(), "person@example.test", "correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	session, err := service.Login(context.Background(), user.Email, "correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	stored := repository.usersByID[user.ID]
	stored.AuthVersion++
	repository.storeUser(stored)
	if _, err := service.Authenticate(context.Background(), session.AccessToken); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("old access token survived version change: %v", err)
	}
}
