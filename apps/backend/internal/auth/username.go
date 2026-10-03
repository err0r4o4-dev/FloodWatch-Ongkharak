package auth

import (
	"regexp"
	"strings"
)

var usernamePattern = regexp.MustCompile(`^[a-z0-9_]{3,32}$`)

func normalizeUsername(username string) (string, error) {
	username = strings.ToLower(strings.TrimSpace(username))
	if !usernamePattern.MatchString(username) {
		return "", ErrInvalidRequest
	}
	return username, nil
}
