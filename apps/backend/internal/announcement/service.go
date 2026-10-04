package announcement

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/externaldata"
)

var ErrInvalid = errors.New("invalid announcement pagination")

type Evidence struct {
	ID              string
	RetrievedAt     time.Time
	LastRetrievedAt time.Time
	Record          externaldata.Record
}

type EvidenceSnapshot struct {
	Enabled       bool
	LastSuccessAt *time.Time
	ErrorCode     *string
	Records       []Evidence
}

type Store interface {
	Evidence(context.Context) (EvidenceSnapshot, error)
}

type Source struct {
	Code          string     `json:"code"`
	Name          string     `json:"name"`
	SourceType    string     `json:"sourceType"`
	URL           string     `json:"url"`
	Enabled       bool       `json:"enabled"`
	Status        string     `json:"status"`
	Freshness     string     `json:"freshness"`
	LastSuccessAt *time.Time `json:"lastSuccessAt"`
}

type Revision struct {
	EvidenceID  string     `json:"evidenceId"`
	Identifier  string     `json:"identifier"`
	MessageType string     `json:"messageType"`
	IssuedAt    time.Time  `json:"issuedAt"`
	RetrievedAt time.Time  `json:"retrievedAt"`
	ExpiresAt   *time.Time `json:"expiresAt"`
	Headline    string     `json:"headline"`
	Description string     `json:"description"`
	AreaCodes   []string   `json:"areaCodes"`
	Reason      *string    `json:"reason"`
}

type Item struct {
	ID              string     `json:"id"`
	Identifier      string     `json:"identifier"`
	Headline        string     `json:"headline"`
	Description     string     `json:"description"`
	Status          string     `json:"status"`
	EffectiveStatus string     `json:"effectiveStatus"`
	Freshness       string     `json:"freshness"`
	IssuedAt        time.Time  `json:"issuedAt"`
	ExpiresAt       *time.Time `json:"expiresAt"`
	AreaCodes       []string   `json:"areaCodes"`
	Reason          *string    `json:"reason"`
	HistoryTotal    int        `json:"historyTotal"`
	History         []Revision `json:"history"`
}

type Result struct {
	RetrievedAt time.Time `json:"retrievedAt"`
	Source      Source    `json:"source"`
	Items       []Item    `json:"items"`
	Limit       int       `json:"limit"`
	Offset      int       `json:"offset"`
	Total       int       `json:"total"`
}

type Service struct {
	store   Store
	enabled bool
	Clock   func() time.Time
}

func NewService(store Store, enabled bool) *Service { return &Service{store: store, enabled: enabled} }

func (service *Service) List(ctx context.Context, limit, offset int) (Result, error) {
	if limit < 1 || limit > 50 || offset < 0 || offset > 10000 {
		return Result{}, ErrInvalid
	}
	now := time.Now().UTC()
	if service.Clock != nil {
		now = service.Clock().UTC()
	}
	result := Result{RetrievedAt: now, Limit: limit, Offset: offset, Items: []Item{}, Source: Source{Code: "TMD_CAP", Name: "Thai Meteorological Department", SourceType: "OFFICIAL", URL: "https://www.tmd.go.th", Enabled: service.enabled, Status: "NOT_CONFIGURED", Freshness: "UNKNOWN"}}
	if !service.enabled {
		return result, nil
	}
	if service.store == nil {
		return Result{}, errors.New("announcement evidence unavailable")
	}
	stored, err := service.store.Evidence(ctx)
	if errors.Is(err, externaldata.ErrNotFound) {
		result.Source.Status = "UNAVAILABLE"
		return result, nil
	}
	if err != nil {
		return Result{}, errors.New("announcement evidence unavailable")
	}
	if !stored.Enabled {
		result.Source.Enabled = false
		return result, nil
	}
	result.Source.Status = "UNAVAILABLE"
	result.Source.LastSuccessAt = stored.LastSuccessAt
	if stored.LastSuccessAt != nil {
		age := now.Sub(*stored.LastSuccessAt)
		result.Source.Status = "DEGRADED"
		result.Source.Freshness = "STALE"
		if age >= 0 && age <= 30*time.Minute && stored.ErrorCode == nil {
			result.Source.Status = "AVAILABLE"
			result.Source.Freshness = "FRESH"
		}
	}
	items := buildItems(stored.Records, result.Source, now)
	result.Total = len(items)
	if offset < len(items) {
		end := offset + limit
		if end > len(items) {
			end = len(items)
		}
		result.Items = items[offset:end]
	}
	return result, nil
}

func buildItems(evidence []Evidence, source Source, now time.Time) []Item {
	records := make([]Evidence, 0, len(evidence))
	seen := map[string]bool{}
	for _, entry := range evidence {
		record := entry.Record
		if seen[entry.ID] || entry.ID == "" || record.Kind != externaldata.OfficialAlert || record.Provenance != "OFFICIAL" || record.AlertIdentifier == "" || record.IssuedAt == nil || record.IssuedAt.After(now.Add(time.Minute)) {
			continue
		}
		if record.AlertMessageType != "Alert" && record.AlertMessageType != "Update" && record.AlertMessageType != "Cancel" {
			continue
		}
		seen[entry.ID] = true
		records = append(records, entry)
	}
	sort.Slice(records, func(first, second int) bool {
		left, right := records[first], records[second]
		if !left.Record.IssuedAt.Equal(*right.Record.IssuedAt) {
			return left.Record.IssuedAt.Before(*right.Record.IssuedAt)
		}
		if !left.RetrievedAt.Equal(right.RetrievedAt) {
			return left.RetrievedAt.Before(right.RetrievedAt)
		}
		return left.ID < right.ID
	})
	parents := make([]int, len(records))
	identifiers := map[string][]int{}
	for index, entry := range records {
		parents[index] = index
		identifiers[entry.Record.AlertIdentifier] = append(identifiers[entry.Record.AlertIdentifier], index)
	}
	var root func(int) int
	root = func(index int) int {
		if parents[index] != index {
			parents[index] = root(parents[index])
		}
		return parents[index]
	}
	join := func(first, second int) {
		left, right := root(first), root(second)
		if left < right {
			parents[right] = left
		} else {
			parents[left] = right
		}
	}
	missingReferences := map[string]int{}
	for index, entry := range records {
		join(index, identifiers[entry.Record.AlertIdentifier][0])
		if entry.Record.AlertMessageType == "Alert" {
			continue
		}
		for _, reference := range entry.Record.AlertReferences {
			if matches := identifiers[reference]; len(matches) > 0 {
				other := matches[0]
				if !records[other].Record.IssuedAt.After(*entry.Record.IssuedAt) {
					join(index, other)
				}
			} else if other, exists := missingReferences[reference]; exists {
				join(index, other)
			} else {
				missingReferences[reference] = index
			}
		}
	}
	groups := map[int][]Evidence{}
	for index, entry := range records {
		identifier := root(index)
		groups[identifier] = append(groups[identifier], entry)
	}
	items := []Item{}
	for _, group := range groups {
		applicable := false
		for _, entry := range group {
			if hasTargetArea(entry.Record.AreaCodes) || len(entry.Record.Geometry) > 0 && entry.Record.Reason == "" {
				applicable = true
			}
		}
		if !applicable {
			continue
		}
		latest, ambiguous := currentEvidence(group)
		record := latest.Record
		item := Item{ID: group[0].ID, Identifier: record.AlertIdentifier, Headline: record.Headline, Description: record.Description, IssuedAt: *record.IssuedAt, ExpiresAt: record.ValidTo, AreaCodes: append([]string{}, record.AreaCodes...), Status: "ACTIVE", Freshness: "FRESH", HistoryTotal: len(group), History: []Revision{}}
		if record.AlertMessageType == "Update" {
			item.Status = "UPDATED"
		}
		if record.AlertMessageType == "Cancel" {
			item.Status = "CANCELLED"
			for index := len(group) - 1; index >= 0; index-- {
				if group[index].ID == latest.ID {
					continue
				}
				prior := group[index].Record
				if item.Headline == "" {
					item.Headline = prior.Headline
				}
				if item.Description == "" {
					item.Description = prior.Description
				}
				if len(item.AreaCodes) == 0 && len(prior.AreaCodes) > 0 {
					item.AreaCodes = append([]string{}, prior.AreaCodes...)
				}
			}
		} else if record.ValidTo != nil && !record.ValidTo.After(now) {
			item.Status = "EXPIRED"
		}
		item.EffectiveStatus = item.Status
		if item.Status == "ACTIVE" || item.Status == "UPDATED" {
			reason := record.Reason
			if record.ValidTo == nil && reason == "" {
				reason = "MISSING_EXPIRY"
			}
			if source.Freshness != "FRESH" {
				reason = "SOURCE_UNAVAILABLE"
			}
			if record.ValidFrom != nil && record.ValidFrom.After(now) {
				reason = "NOT_YET_EFFECTIVE"
			}
			if reason != "" {
				item.EffectiveStatus = "UNKNOWN"
				item.Freshness = "UNKNOWN"
				item.Reason = &reason
			}
		}
		if ambiguous && item.Status != "CANCELLED" {
			reason := "CONFLICTING_LIFECYCLE"
			item.EffectiveStatus, item.Freshness, item.Reason = "UNKNOWN", "UNKNOWN", &reason
		}
		for index := len(group) - 1; index >= 0 && len(item.History) < 20; index-- {
			entry := group[index]
			record := entry.Record
			revision := Revision{EvidenceID: entry.ID, Identifier: record.AlertIdentifier, MessageType: record.AlertMessageType, IssuedAt: *record.IssuedAt, RetrievedAt: entry.RetrievedAt, ExpiresAt: record.ValidTo, Headline: record.Headline, Description: record.Description, AreaCodes: append([]string{}, record.AreaCodes...)}
			if record.Reason != "" {
				reason := record.Reason
				revision.Reason = &reason
			}
			item.History = append(item.History, revision)
		}
		items = append(items, item)
	}
	sort.Slice(items, func(first, second int) bool {
		if !items[first].IssuedAt.Equal(items[second].IssuedAt) {
			return items[first].IssuedAt.After(items[second].IssuedAt)
		}
		return items[first].ID < items[second].ID
	})
	return items
}

func currentEvidence(group []Evidence) (Evidence, bool) {
	latest := group[len(group)-1]
	for _, entry := range group {
		if entry.Record.IssuedAt.Equal(*latest.Record.IssuedAt) && lifecycleRank(entry.Record.AlertMessageType) > lifecycleRank(latest.Record.AlertMessageType) {
			latest = entry
		}
	}
	candidates := map[string]Evidence{}
	for _, entry := range group {
		if !entry.Record.IssuedAt.Equal(*latest.Record.IssuedAt) || entry.Record.AlertMessageType != latest.Record.AlertMessageType {
			continue
		}
		previous, exists := candidates[entry.Record.AlertIdentifier]
		if !exists || !lastRetrieval(entry).Before(lastRetrieval(previous)) {
			candidates[entry.Record.AlertIdentifier] = entry
		}
	}
	superseded := map[string]bool{}
	for _, entry := range group {
		if !entry.Record.IssuedAt.Equal(*latest.Record.IssuedAt) || entry.Record.AlertMessageType != latest.Record.AlertMessageType {
			continue
		}
		for _, reference := range entry.Record.AlertReferences {
			if reference != entry.Record.AlertIdentifier {
				superseded[reference] = true
			}
		}
	}
	terminalCount := 0
	for identifier, entry := range candidates {
		if superseded[identifier] {
			continue
		}
		terminalCount++
		if terminalCount == 1 || entry.ID > latest.ID {
			latest = entry
		}
	}
	return latest, terminalCount != 1
}

func lifecycleRank(messageType string) int {
	switch messageType {
	case "Cancel":
		return 2
	case "Update":
		return 1
	default:
		return 0
	}
}

func lastRetrieval(entry Evidence) time.Time {
	if entry.LastRetrievedAt.IsZero() {
		return entry.RetrievedAt
	}
	return entry.LastRetrievedAt
}

func hasTargetArea(codes []string) bool {
	for _, code := range codes {
		if strings.EqualFold(strings.TrimSpace(code), "TH-26") {
			return true
		}
	}
	return false
}
