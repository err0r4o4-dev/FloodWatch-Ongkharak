package weather

import (
	"context"
	"sync"
	"time"
)

type Service struct {
	provider    Provider
	now         func() time.Time
	mutex       sync.Mutex
	cached      *Current
	retrievedAt time.Time
	nextAttempt time.Time
	failed      bool
	inFlight    chan struct{}
}

func NewService(provider Provider) *Service { return &Service{provider: provider, now: time.Now} }

func (service *Service) Get(ctx context.Context) Snapshot {
	for {
		if ctx.Err() != nil {
			return unavailable("REQUEST_CANCELED")
		}
		if service == nil || service.provider == nil {
			return unavailable("NOT_CONFIGURED")
		}
		service.mutex.Lock()
		if service.now().Before(service.nextAttempt) {
			result := service.snapshot()
			service.mutex.Unlock()
			return result
		}
		pending := service.inFlight
		if pending == nil {
			pending = make(chan struct{})
			service.inFlight = pending
			go service.refresh(pending)
		}
		service.mutex.Unlock()
		select {
		case <-ctx.Done():
			return unavailable("REQUEST_CANCELED")
		case <-pending:
		}
	}
}

func (service *Service) refresh(pending chan struct{}) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	current, err := service.provider.FetchCurrent(ctx)
	service.mutex.Lock()
	defer service.mutex.Unlock()
	now := service.now().UTC()
	service.failed = err != nil
	service.nextAttempt = now.Add(time.Minute)
	if err == nil {
		service.cached = &current
		service.retrievedAt = now
		service.nextAttempt = now.Add(10 * time.Minute)
	}
	service.inFlight = nil
	close(pending)
}

func (service *Service) snapshot() Snapshot {
	result := unavailable("UPSTREAM_UNAVAILABLE")
	if service.cached == nil {
		return result
	}
	age := service.now().Sub(service.cached.ObservedAt)
	if age > 6*time.Hour || age < -time.Minute {
		return result
	}
	current := *service.cached
	retrieved := service.retrievedAt
	result.Current = &current
	result.RetrievedAt = &retrieved
	result.Freshness = "FRESH"
	result.Reason = nil
	if service.failed {
		result.Freshness = "STALE"
	} else if age > time.Hour {
		reason := "DATA_STALE"
		result.Freshness = "STALE"
		result.Reason = &reason
	}
	if service.failed {
		reason := "UPSTREAM_UNAVAILABLE"
		result.Reason = &reason
	}
	return result
}

func unavailable(reason string) Snapshot {
	return Snapshot{Location: Location{Name: "Ongkharak", Latitude: 14.1, Longitude: 101}, Source: Source{Name: "Open-Meteo", URL: "https://open-meteo.com/", Kind: "MODEL"}, Freshness: "UNKNOWN", Reason: &reason}
}
