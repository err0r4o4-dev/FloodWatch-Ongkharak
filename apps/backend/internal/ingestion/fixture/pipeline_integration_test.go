package fixture

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/flood"
	"github.com/floodwatch-ongkharak/backend/internal/ingestion"
	"github.com/floodwatch-ongkharak/backend/internal/postgres/ingestionrepo"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestFixturePipelineWithPostGISAndControlledHTTP(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL for PostGIS fixture pipeline test")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	source := flood.DataSource{ID: uuid.NewString(), Code: "SYNTH_" + strings.ToUpper(uuid.NewString()[:8]), SourceType: flood.SourceCommunity, Provider: "Synthetic fixture test", Enabled: true}
	if _, err := pool.Exec(ctx, `INSERT INTO data_sources (id,code,name,source_type,provider) VALUES ($1,$2,$3,$4,$5)`, source.ID, source.Code, "Synthetic fixture source", source.SourceType, source.Provider); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for _, query := range []string{
			`DELETE FROM water_readings WHERE source_id=$1`,
			`DELETE FROM water_stations WHERE source_id=$1`,
			`DELETE FROM ingestion_runs WHERE source_id=$1`,
			`DELETE FROM data_sources WHERE id=$1`,
		} {
			if _, err := pool.Exec(context.Background(), query, source.ID); err != nil {
				t.Errorf("clean up synthetic fixture: %v", err)
			}
		}
	})
	observedAt := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	valid := fmt.Sprintf(`{"stationCode":"SYNTH_STATION","stationName":"Synthetic station","longitude":"102.01","latitude":"15.01","observedAt":%q,"waterLevel":{"value":"125","unit":"cm"}}`, observedAt.Format(time.RFC3339))
	invalid := fmt.Sprintf(`{"stationCode":"SYNTH_OTHER","stationName":"Synthetic invalid station","longitude":"102.02","latitude":"15.02","observedAt":%q,"waterLevel":{"value":"1","unit":"unknown"}}`, observedAt.Format(time.RFC3339))
	nulName := fmt.Sprintf(`{"stationCode":"SYNTH_NUL","stationName":"Synthetic \u0000station","longitude":"102.02","latitude":"15.02","observedAt":%q,"waterLevel":{"value":"1","unit":"m"}}`, observedAt.Format(time.RFC3339))
	mode, requests := "partial", 0
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		requests++
		switch mode {
		case "partial":
			_, _ = fmt.Fprintf(response, `{"kind":"SYNTHETIC_TEST_ONLY","observations":[%s,%s]}`, valid, invalid)
		case "nul":
			_, _ = fmt.Fprintf(response, `{"kind":"SYNTHETIC_TEST_ONLY","observations":[%s,%s]}`, valid, nulName)
		case "valid":
			_, _ = fmt.Fprintf(response, `{"kind":"SYNTHETIC_TEST_ONLY","observations":[%s]}`, valid)
		case "503":
			if requests == 1 {
				response.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			_, _ = fmt.Fprintf(response, `{"kind":"SYNTHETIC_TEST_ONLY","observations":[%s]}`, valid)
		case "429":
			if requests == 1 {
				response.Header().Set("Retry-After", "0")
				response.WriteHeader(http.StatusTooManyRequests)
				return
			}
			_, _ = fmt.Fprintf(response, `{"kind":"SYNTHETIC_TEST_ONLY","observations":[%s]}`, valid)
		case "malformed":
			_, _ = response.Write([]byte(`{"kind":`))
		case "oversized":
			_, _ = response.Write([]byte(strings.Repeat("x", 1024)))
		case "slow":
			<-request.Context().Done()
		}
	}))
	defer server.Close()
	client, err := ingestion.NewHTTPClient(server.URL, ingestion.HTTPOptions{Timeout: 50 * time.Millisecond, MaxBytes: 512, Attempts: 2, AllowPrivate: true})
	if err != nil {
		t.Fatal(err)
	}
	engine := &ingestion.Engine{Store: ingestionrepo.New(pool), Providers: map[string]ingestion.Registration{source.Code: {Provider: New(client)}}, RunTimeout: time.Second}
	run := func(next string) ingestion.Run {
		t.Helper()
		mode, requests = next, 0
		result, err := engine.Execute(ctx, source)
		if err != nil {
			t.Fatalf("execute %s: %v", next, err)
		}
		return result
	}
	partial := run("partial")
	if partial.Status != ingestion.Partial || partial.FetchedCount != 2 || partial.AcceptedCount != 1 || partial.RejectedCount != 1 || partial.InsertedCount != 1 {
		t.Fatalf("partial run: %+v", partial)
	}
	if rejectedNUL := run("nul"); rejectedNUL.Status != ingestion.Partial || rejectedNUL.FetchedCount != 2 || rejectedNUL.AcceptedCount != 1 || rejectedNUL.RejectedCount != 1 || rejectedNUL.DuplicateCount != 1 {
		t.Fatalf("NUL station field was persisted: %+v", rejectedNUL)
	}
	if rejectedNUL := run("nul"); rejectedNUL.Status != ingestion.Partial || rejectedNUL.FetchedCount != 2 || rejectedNUL.AcceptedCount != 1 || rejectedNUL.RejectedCount != 1 || rejectedNUL.DuplicateCount != 1 {
		t.Fatalf("NUL station field was persisted: %+v", rejectedNUL)
	}
	for _, transient := range []string{"503", "429"} {
		result := run(transient)
		if result.Status != ingestion.Success || result.DuplicateCount != 1 || requests != 2 {
			t.Fatalf("retry %s: %+v requests=%d", transient, result, requests)
		}
	}
	for _, failure := range []string{"malformed", "oversized", "slow"} {
		result := run(failure)
		if result.Status != ingestion.Failed || result.InsertedCount != 0 {
			t.Fatalf("failure %s: %+v", failure, result)
		}
	}
	var readingCount, failures int
	var readingSourceType, healthStatus string
	if err := pool.QueryRow(ctx, `SELECT count(*), max(d.source_type) FROM water_readings r JOIN data_sources d ON d.id=r.source_id WHERE r.source_id=$1`, source.ID).Scan(&readingCount, &readingSourceType); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT health_status, consecutive_failures FROM data_sources WHERE id=$1`, source.ID).Scan(&healthStatus, &failures); err != nil {
		t.Fatal(err)
	}
	if readingCount != 1 || readingSourceType != "COMMUNITY" || healthStatus != "UNAVAILABLE" || failures != 3 {
		t.Fatalf("failure erased data or promoted source: readings=%d source=%s health=%s failures=%d", readingCount, readingSourceType, healthStatus, failures)
	}
	if recovered := run("valid"); recovered.Status != ingestion.Success || recovered.DuplicateCount != 1 {
		t.Fatalf("recovery: %+v", recovered)
	}
	if err := pool.QueryRow(ctx, `SELECT health_status, consecutive_failures FROM data_sources WHERE id=$1`, source.ID).Scan(&healthStatus, &failures); err != nil || healthStatus != "HEALTHY" || failures != 0 {
		t.Fatalf("recovered health=%s failures=%d err=%v", healthStatus, failures, err)
	}
}
