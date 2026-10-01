package healthcheck

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type Postgres struct {
	Pool    *pgxpool.Pool
	Timeout time.Duration
}

func (Postgres) Name() string { return "postgres" }

func (checker Postgres) Check(ctx context.Context) error {
	checkContext, cancel := context.WithTimeout(ctx, checker.Timeout)
	defer cancel()
	return checker.Pool.Ping(checkContext)
}

type Redis struct {
	Client  *redis.Client
	Timeout time.Duration
}

func (Redis) Name() string { return "redis" }

func (checker Redis) Check(ctx context.Context) error {
	checkContext, cancel := context.WithTimeout(ctx, checker.Timeout)
	defer cancel()
	return checker.Client.Ping(checkContext).Err()
}

type MinIO struct {
	Endpoint string
	Client   *http.Client
	Timeout  time.Duration
}

func (MinIO) Name() string { return "minio" }

func (checker MinIO) Check(ctx context.Context) error {
	// MinIO liveness avoids requiring object permissions during Phase 0.
	checkContext, cancel := context.WithTimeout(ctx, checker.Timeout)
	defer cancel()
	request, err := http.NewRequestWithContext(checkContext, http.MethodGet, strings.TrimRight(checker.Endpoint, "/")+"/minio/health/live", nil)
	if err != nil {
		return fmt.Errorf("create MinIO liveness request: %w", err)
	}
	response, err := checker.Client.Do(request)
	if err != nil {
		return fmt.Errorf("request MinIO liveness: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("MinIO liveness returned HTTP %d", response.StatusCode)
	}
	return nil
}
