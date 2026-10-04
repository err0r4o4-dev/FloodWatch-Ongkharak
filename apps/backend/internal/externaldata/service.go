package externaldata

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/url"
	"strings"
	"sync"
	"time"
)

func Definitions() []Definition {
	return []Definition{
		{Code: "THAIWATER_WATER", Name: "ThaiWater water stations", Kind: Observation, Provenance: "OFFICIAL", URL: "https://www.thaiwater.net", Interval: 10 * time.Minute, MaxAge: 6 * time.Hour},
		{Code: "THAIWATER_RAIN", Name: "ThaiWater rain stations", Kind: Observation, Provenance: "OFFICIAL", URL: "https://www.thaiwater.net", Interval: 10 * time.Minute, MaxAge: 3 * time.Hour},
		{Code: "THAIWATER_GATE", Name: "ThaiWater water gates", Kind: Observation, Provenance: "OFFICIAL", URL: "https://www.thaiwater.net", Interval: 15 * time.Minute, MaxAge: 6 * time.Hour},
		{Code: "OPENMETEO_ECMWF", Name: "ECMWF via Open-Meteo", Kind: Forecast, Provenance: "MODEL", URL: "https://open-meteo.com", Interval: time.Hour, MaxAge: 3 * time.Hour},
		{Code: "OPENMETEO_GFS", Name: "GFS via Open-Meteo", Kind: Forecast, Provenance: "MODEL", URL: "https://open-meteo.com", Interval: time.Hour, MaxAge: 3 * time.Hour},
		{Code: "OPENMETEO_ICON", Name: "ICON via Open-Meteo", Kind: Forecast, Provenance: "MODEL", URL: "https://open-meteo.com", Interval: time.Hour, MaxAge: 3 * time.Hour},
		{Code: "OPENMETEO_ENSEMBLE", Name: "ECMWF ensemble via Open-Meteo", Kind: Forecast, Provenance: "MODEL", URL: "https://open-meteo.com", Interval: time.Hour, MaxAge: 3 * time.Hour},
		{Code: "GLOFAS", Name: "GloFAS via Open-Meteo", Kind: Forecast, Provenance: "MODEL", URL: "https://open-meteo.com", Interval: 6 * time.Hour, MaxAge: 36 * time.Hour},
		{Code: "HII_WRF", Name: "HII WRF-ROMS", Kind: Forecast, Provenance: "MODEL", URL: "https://www.thaiwater.net/forecast/wrfroms", Interval: 6 * time.Hour, MaxAge: 36 * time.Hour},
		{Code: "TMD_CAP", Name: "TMD official warnings", Kind: OfficialAlert, Provenance: "OFFICIAL", URL: "https://www.tmd.go.th/api/xml/CAP", Interval: 10 * time.Minute, MaxAge: 30 * time.Minute},
	}
}

type Service struct {
	store     Store
	objects   ObjectStore
	providers map[string]Provider
	mutex     sync.Mutex
	next      map[string]time.Time
	running   map[string]bool
	Clock     func() time.Time
}

func NewService(store Store, objects ObjectStore, configured map[string]Provider) (*Service, error) {
	known := map[string]bool{}
	for _, definition := range Definitions() {
		known[definition.Code] = true
	}
	providers := make(map[string]Provider, len(configured))
	for code, provider := range configured {
		if !known[code] || provider == nil {
			return nil, errors.New("unregistered external data provider")
		}
		providers[code] = provider
	}
	if len(providers) > 0 && (store == nil || objects == nil) {
		return nil, errors.New("external data persistence is unavailable")
	}
	return &Service{store: store, objects: objects, providers: providers, next: map[string]time.Time{}, running: map[string]bool{}}, nil
}

func (service *Service) now() time.Time {
	if service.Clock != nil {
		return service.Clock().UTC()
	}
	return time.Now().UTC()
}

func (service *Service) Poll(ctx context.Context) error {
	var wait sync.WaitGroup
	var failures []error
	var failureMutex sync.Mutex
	slots := make(chan struct{}, 2)
	for _, definition := range Definitions() {
		provider, enabled := service.providers[definition.Code]
		if !enabled {
			continue
		}
		service.mutex.Lock()
		due := !service.running[definition.Code] && !service.now().Before(service.next[definition.Code])
		if due {
			service.running[definition.Code] = true
			service.next[definition.Code] = service.now().Add(definition.Interval)
		}
		service.mutex.Unlock()
		if !due {
			continue
		}
		select {
		case slots <- struct{}{}:
		case <-ctx.Done():
			service.release(definition.Code)
			wait.Wait()
			return ctx.Err()
		}
		wait.Add(1)
		go func(definition Definition, provider Provider) {
			defer wait.Done()
			defer func() { <-slots; service.release(definition.Code) }()
			attempt, cancel := context.WithTimeout(ctx, 90*time.Second)
			defer cancel()
			if err := service.execute(attempt, definition, provider); err != nil {
				failureMutex.Lock()
				failures = append(failures, fmt.Errorf("%s: %w", definition.Code, err))
				failureMutex.Unlock()
			}
		}(definition, provider)
	}
	wait.Wait()
	return errors.Join(failures...)
}

func (service *Service) release(code string) {
	service.mutex.Lock()
	defer service.mutex.Unlock()
	service.running[code] = false
}

func (service *Service) Run(ctx context.Context, logger *slog.Logger) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		if err := service.Poll(ctx); err != nil && ctx.Err() == nil && logger != nil {
			logger.Warn("external data ingestion deferred", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (service *Service) execute(ctx context.Context, definition Definition, provider Provider) error {
	runID, err := service.store.Start(ctx, definition.Code, service.now())
	if err != nil {
		return errors.New("ingestion lease unavailable")
	}
	fail := func(code string) error {
		finalize, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := service.store.Fail(finalize, runID, definition.Code, code, service.now()); err != nil {
			return errors.New("ingestion failure could not be finalized")
		}
		return errors.New(code)
	}
	batch, err := provider.Fetch(ctx)
	if err != nil {
		return fail("FETCH_FAILED")
	}
	if err := validateBatch(definition, batch, service.now()); err != nil {
		return fail("INVALID_PAYLOAD")
	}
	references := make([]RawReference, 0, len(batch.Raw))
	for _, raw := range batch.Raw {
		digest := fmt.Sprintf("%x", sha256.Sum256(raw.Body))
		key := "external-data/" + definition.Code + "/" + digest
		if err := service.objects.Put(ctx, key, raw.ContentType, raw.Body); err != nil {
			return fail("ARCHIVE_FAILED")
		}
		references = append(references, RawReference{URL: raw.URL, ContentType: raw.ContentType, ObjectKey: key, SHA256: digest, Size: len(raw.Body)})
	}
	if err := service.store.Complete(ctx, runID, definition.Code, batch, references); err != nil {
		return fail("PERSIST_FAILED")
	}
	return nil
}

func validateBatch(definition Definition, batch Batch, now time.Time) error {
	if batch.RetrievedAt.IsZero() || batch.RetrievedAt.After(now.Add(time.Minute)) || now.Sub(batch.RetrievedAt) > 5*time.Minute || len(batch.Records) > 20000 || len(batch.Raw) == 0 || len(batch.Raw) > 16 {
		return errors.New("invalid batch bounds")
	}
	total := 0
	for _, raw := range batch.Raw {
		total += len(raw.Body)
		if len(raw.Body) == 0 || len(raw.Body) > 10<<20 || total > 40<<20 || !trustedURL(definition.Code, raw.URL) {
			return errors.New("invalid raw evidence")
		}
	}
	keys := map[string]bool{}
	for _, record := range batch.Records {
		if record.Key == "" || len(record.Key) > 512 || keys[record.Key] || record.Kind != definition.Kind || record.Provenance != definition.Provenance || len(record.Description) > 30000 || len(record.Headline) > 2000 || len(record.Geometry) > 2<<20 {
			return errors.New("invalid record identity")
		}
		keys[record.Key] = true
		if record.Value != nil && (math.IsNaN(*record.Value) || math.IsInf(*record.Value, 0)) {
			return errors.New("invalid measurement")
		}
		if record.Kind == Observation {
			if record.Station == nil || record.ObservedAt == nil || record.ObservedAt.IsZero() || record.ObservedAt.After(batch.RetrievedAt.Add(time.Minute)) || record.Station.Code == "" || record.Station.Name == "" || !validCoordinate(record.Station.Latitude, record.Station.Longitude) {
				return errors.New("invalid station observation")
			}
		}
		if record.Kind == Forecast {
			unverifiedRaster := definition.Code == "HII_WRF" && record.Model == "HII_WRF_ROMS" && record.Value == nil && record.Reason != "" && record.ValidFrom == nil && record.ValidTo == nil
			invalidInterval := !unverifiedRaster && (record.ValidFrom == nil || record.ValidTo == nil || !record.ValidFrom.Before(*record.ValidTo))
			if invalidInterval || record.Model == "" || record.Latitude == nil || record.Longitude == nil || !validCoordinate(*record.Latitude, *record.Longitude) {
				return errors.New("invalid forecast interval/grid")
			}
		}
		if record.Kind == OfficialAlert {
			indeterminateTiming := definition.Code == "TMD_CAP" && record.Reason == "CONFLICTING_TIME_WINDOWS" && record.Freshness == "UNKNOWN" && record.ValidFrom == nil && record.ValidTo == nil && (record.AlertMessageType == "Alert" || record.AlertMessageType == "Update")
			if record.AlertIdentifier == "" || record.IssuedAt == nil || record.ValidTo == nil && record.AlertMessageType != "Cancel" && !indeterminateTiming {
				return errors.New("invalid CAP lifecycle")
			}
		}
	}
	return nil
}

func ValidateBatch(code string, batch Batch, now time.Time) error {
	for _, definition := range Definitions() {
		if definition.Code == code {
			return validateBatch(definition, batch, now)
		}
	}
	return errors.New("unregistered external provider")
}

func validCoordinate(latitude, longitude float64) bool {
	return !math.IsNaN(latitude) && !math.IsNaN(longitude) && !math.IsInf(latitude, 0) && !math.IsInf(longitude, 0) && latitude >= -90 && latitude <= 90 && longitude >= -180 && longitude <= 180
}

func trustedURL(code, raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || parsed.User != nil || parsed.Fragment != "" {
		return false
	}
	for name := range parsed.Query() {
		if strings.Contains(strings.ToLower(name), "key") || strings.Contains(strings.ToLower(name), "token") {
			return false
		}
	}
	switch code {
	case "THAIWATER_WATER", "THAIWATER_RAIN", "THAIWATER_GATE":
		return parsed.Host == "api-v3.thaiwater.net" && strings.HasPrefix(parsed.Path, "/api/v1/thaiwater30/public/")
	case "OPENMETEO_ECMWF", "OPENMETEO_GFS", "OPENMETEO_ICON":
		return parsed.Host == "api.open-meteo.com" && parsed.Path == "/v1/forecast"
	case "OPENMETEO_ENSEMBLE":
		return parsed.Host == "ensemble-api.open-meteo.com" && parsed.Path == "/v1/ensemble"
	case "GLOFAS":
		return parsed.Host == "flood-api.open-meteo.com" && parsed.Path == "/v1/flood"
	case "HII_WRF":
		return parsed.Host == "live1.hii.or.th" && strings.HasPrefix(parsed.Path, "/product/latest/wrfroms/tiff/")
	case "TMD_CAP":
		return parsed.Host == "www.tmd.go.th" && (parsed.Path == "/api/xml/CAP" || strings.HasPrefix(parsed.Path, "/uploads/CAP/"))
	default:
		return false
	}
}

func (service *Service) Snapshot(ctx context.Context) (Snapshot, error) {
	now := service.now()
	snapshot := Snapshot{RetrievedAt: now, Sources: make([]SourceSnapshot, 0, 10)}
	for _, definition := range Definitions() {
		_, enabled := service.providers[definition.Code]
		source := SourceSnapshot{Definition: definition, Enabled: enabled, Status: "NOT_CONFIGURED", Freshness: "UNKNOWN", Records: []Record{}}
		if enabled {
			stored, err := service.store.Snapshot(ctx, definition.Code, 24)
			if err != nil {
				if errors.Is(err, ErrNotFound) {
					source.Status = "UNAVAILABLE"
					snapshot.Sources = append(snapshot.Sources, source)
					continue
				}
				return Snapshot{}, errors.New("external data store unavailable")
			}
			if stored.Code == definition.Code && !stored.Enabled {
				source.Enabled = false
				snapshot.Sources = append(snapshot.Sources, source)
				continue
			}
			source = stored
			source.Definition = definition
			source.Enabled = true
			if source.Records == nil {
				source.Records = []Record{}
			}
			source.Status = "UNAVAILABLE"
			source.Freshness = "UNKNOWN"
			if source.LastSuccessAt != nil {
				age := now.Sub(*source.LastSuccessAt)
				if age >= 0 && age <= definition.MaxAge {
					source.Status = "AVAILABLE"
					source.Freshness = "FRESH"
				} else {
					source.Status = "DEGRADED"
					source.Freshness = "STALE"
				}
				if source.ErrorCode != nil {
					source.Status = "DEGRADED"
					source.Freshness = "STALE"
				}
			}
			for index := range source.Records {
				source.Records[index] = presentRecord(source.Records[index], definition, source, now)
			}
		}
		snapshot.Sources = append(snapshot.Sources, source)
	}
	return snapshot, nil
}

func presentRecord(record Record, definition Definition, source SourceSnapshot, now time.Time) Record {
	record.Freshness = "FRESH"
	unknown := func(reason string) Record {
		record.Freshness = "UNKNOWN"
		record.Value = nil
		if record.Reason == "" {
			record.Reason = reason
		}
		return record
	}
	if source.LastSuccessAt == nil || now.Sub(*source.LastSuccessAt) > 2*definition.MaxAge {
		return unknown("SOURCE_EXPIRED")
	}
	if record.Reason != "" {
		return unknown(record.Reason)
	}
	if record.Kind == Observation {
		if record.ObservedAt == nil || record.Value == nil {
			return unknown("MISSING_MEASUREMENT")
		}
		age := now.Sub(*record.ObservedAt)
		if age < -time.Minute || age > 2*definition.MaxAge {
			return unknown("OBSERVATION_EXPIRED")
		}
		if age > definition.MaxAge {
			record.Freshness = "STALE"
		}
	}
	if record.Kind == Forecast {
		if record.Value == nil {
			return unknown("MISSING_MEASUREMENT")
		}
		if record.ValidTo == nil || !record.ValidTo.After(now) {
			return unknown("FORECAST_EXPIRED")
		}
	}
	if record.Kind == OfficialAlert {
		if record.AlertMessageType == "Cancel" {
			return unknown("CANCELLED")
		}
		if record.ValidTo == nil || !record.ValidTo.After(now) {
			return unknown("ALERT_EXPIRED")
		}
		if record.ValidFrom != nil && record.ValidFrom.After(now) {
			return unknown("NOT_YET_EFFECTIVE")
		}
	}
	if source.Freshness != "FRESH" {
		record.Freshness = "STALE"
	}
	return record
}
