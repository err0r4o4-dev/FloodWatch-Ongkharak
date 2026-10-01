package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"sync"

	"golang.org/x/crypto/argon2"
)

const (
	minimumPasswordLength = 12
	maximumPasswordLength = 128
	argonTime             = 3
	argonMemory           = 64 * 1024
	argonParallelism      = 1
	argonSaltLength       = 16
	argonKeyLength        = 32
)

var (
	dummyHashOnce sync.Once
	dummyHash     string
	dummyHashErr  error
)

func validatePassword(password string) error {
	if len(password) < minimumPasswordLength || len(password) > maximumPasswordLength {
		return ErrInvalidRequest
	}
	return nil
}

func hashPassword(password string) (string, error) {
	salt := make([]byte, argonSaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}

	hash := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonParallelism, argonKeyLength)
	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		argonMemory,
		argonTime,
		argonParallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	), nil
}

func verifyPassword(encodedHash, password string) bool {
	params, salt, expectedHash, err := parsePasswordHash(encodedHash)
	if err != nil {
		return false
	}
	actualHash := argon2.IDKey([]byte(password), salt, params.time, params.memory, params.parallelism, uint32(len(expectedHash)))
	return subtle.ConstantTimeCompare(actualHash, expectedHash) == 1
}

func verifyDummyPassword() {
	dummyHashOnce.Do(func() {
		dummyHash, dummyHashErr = hashPassword("invalid-password-input")
	})
	if dummyHashErr == nil {
		_ = verifyPassword(dummyHash, "invalid-password-input")
	}
}

type argonParameters struct {
	memory      uint32
	time        uint32
	parallelism uint8
}

func parsePasswordHash(encodedHash string) (argonParameters, []byte, []byte, error) {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" {
		return argonParameters{}, nil, nil, errors.New("invalid argon2id hash")
	}

	params := argonParameters{}
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &params.memory, &params.time, &params.parallelism); err != nil {
		return argonParameters{}, nil, nil, errors.New("invalid argon2id parameters")
	}
	if params.memory == 0 || params.memory > argonMemory || params.time == 0 || params.time > 10 || params.parallelism == 0 || params.parallelism > 4 {
		return argonParameters{}, nil, nil, errors.New("unsafe argon2id parameters")
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) < argonSaltLength {
		return argonParameters{}, nil, nil, errors.New("invalid argon2id salt")
	}
	hash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(hash) != argonKeyLength {
		return argonParameters{}, nil, nil, errors.New("invalid argon2id key")
	}
	return params, salt, hash, nil
}

func passwordWithinBounds(password string) bool {
	return len(password) >= minimumPasswordLength && len(password) <= maximumPasswordLength
}
