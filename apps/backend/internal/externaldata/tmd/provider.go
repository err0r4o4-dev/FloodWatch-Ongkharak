package tmd

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"math"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/externaldata"
	"github.com/floodwatch-ongkharak/backend/internal/ingestion"
)

const (
	tmdBase      = "https://www.tmd.go.th"
	rssPath      = "/api/xml/CAP"
	maxDocs      = 10
	capNamespace = "urn:oasis:names:tc:emergency:cap:1.2"
)

var errInvalidTMD = errors.New("invalid TMD CAP response")

type Provider struct {
	fetcher externaldata.Fetcher
	now     func() time.Time
}

func New() (*Provider, error) {
	client, err := ingestion.NewHTTPClient(tmdBase, ingestion.HTTPOptions{
		Timeout: 10 * time.Second, MaxBytes: 5 << 20, Attempts: 2,
	})
	if err != nil {
		return nil, err
	}
	return newProvider(client)
}

func NewWithFetcher(fetcher externaldata.Fetcher) (*Provider, error) {
	return newProvider(fetcher)
}

func newProvider(fetcher externaldata.Fetcher) (*Provider, error) {
	if fetcher == nil {
		return nil, errors.New("TMD fetcher unavailable")
	}
	return &Provider{fetcher: fetcher, now: time.Now}, nil
}

func (provider *Provider) Fetch(ctx context.Context) (externaldata.Batch, error) {
	retrievedAt := provider.now().UTC()
	rssBody, err := provider.fetcher.Fetch(ctx, rssPath)
	if err != nil {
		return externaldata.Batch{}, err
	}
	rss, err := parseRSS(rssBody)
	if err != nil {
		return externaldata.Batch{}, err
	}
	batch := externaldata.Batch{
		RetrievedAt: retrievedAt,
		Raw: []externaldata.RawPayload{{
			URL: tmdBase + rssPath, ContentType: "application/rss+xml", Body: append([]byte(nil), rssBody...),
		}},
	}
	known := make(map[string]areaContext)
	alerts := make([]capAlert, 0, len(rss.Channel.Items))
	seenLinks := make(map[string]struct{})
	seenAlerts := make(map[string][]byte)
	for _, item := range rss.Channel.Items {
		link, err := capLink(item.Link)
		if err != nil {
			return externaldata.Batch{}, err
		}
		if _, seen := seenLinks[link]; seen {
			continue
		}
		seenLinks[link] = struct{}{}
		body, err := provider.fetcher.Fetch(ctx, linkPath(link))
		if err != nil {
			return externaldata.Batch{}, err
		}
		batch.Raw = append(batch.Raw, externaldata.RawPayload{
			URL: link, ContentType: "application/xml", Body: append([]byte(nil), body...),
		})
		alert, err := parseCAP(body, retrievedAt)
		if err != nil {
			return externaldata.Batch{}, err
		}
		if alert.Status != "Actual" || alert.MsgType == "Exercise" {
			continue
		}
		if alert.MsgType != "Alert" && alert.MsgType != "Update" && alert.MsgType != "Cancel" {
			return externaldata.Batch{}, errInvalidTMD
		}
		identity := alert.Identifier + ":" + alert.MsgType
		content, err := json.Marshal(alert)
		if err != nil {
			return externaldata.Batch{}, errInvalidTMD
		}
		if previous, exists := seenAlerts[identity]; exists {
			if !bytes.Equal(previous, content) {
				return externaldata.Batch{}, errInvalidTMD
			}
			continue
		}
		seenAlerts[identity] = content
		alerts = append(alerts, alert)
		if match, matched := matchingContext(alert.Infos); matched {
			known[alert.Identifier] = match
		}
	}
	for pass := 0; pass < len(alerts); pass++ {
		for _, alert := range alerts {
			if _, exists := known[alert.Identifier]; exists {
				continue
			}
			if hasDeclaredArea(alert.Infos) {
				continue
			}
			for _, reference := range referenceIdentifiers(alert.References) {
				if match, exists := known[reference]; exists {
					known[alert.Identifier] = match
					break
				}
			}
		}
	}
	for _, alert := range alerts {
		if !alert.Expiry.IsZero() && !alert.Expiry.After(retrievedAt) {
			continue
		}
		match, matched := matchingContext(alert.Infos)
		if !matched && !hasDeclaredArea(alert.Infos) {
			for _, reference := range referenceIdentifiers(alert.References) {
				if previous, ok := known[reference]; ok {
					match, matched = previous, true
					break
				}
			}
		}
		if !matched && alert.MsgType == "Alert" {
			continue
		}
		if matched {
			known[alert.Identifier] = match
		}
		record := makeRecord(alert, match)
		if !matched && alert.MsgType == "Update" {
			if record.Reason == "" {
				record.Reason = "GEOGRAPHIC_SCOPE_UNCONFIRMED"
			}
			record.Freshness = "UNKNOWN"
		}
		batch.Records = append(batch.Records, record)
	}
	return batch, nil
}

type rssDocument struct {
	XMLName xml.Name `xml:"rss"`
	Channel *struct {
		Items []struct {
			Link string `xml:"link"`
		} `xml:"item"`
	} `xml:"channel"`
}

type capDocument struct {
	XMLName    xml.Name `xml:"alert"`
	Identifier string   `xml:"identifier"`
	Sender     string   `xml:"sender"`
	Sent       string   `xml:"sent"`
	Status     string   `xml:"status"`
	MsgType    string   `xml:"msgType"`
	References string   `xml:"references"`
	Infos      []struct {
		Effective   string `xml:"effective"`
		Expires     string `xml:"expires"`
		Headline    string `xml:"headline"`
		Description string `xml:"description"`
		Areas       []struct {
			AreaDesc string `xml:"areaDesc"`
			Geocodes []struct {
				ValueName string `xml:"valueName"`
				Value     string `xml:"value"`
			} `xml:"geocode"`
			Polygon string `xml:"polygon"`
		} `xml:"area"`
	} `xml:"info"`
}

type capAlert struct {
	Identifier        string
	Sender            string
	Sent              time.Time
	Status            string
	MsgType           string
	References        string
	Expiry            time.Time
	Effective         time.Time
	TimingUnconfirmed bool
	Infos             []capInfo
}

type capInfo struct {
	Headline    string
	Description string
	Areas       []capArea
	Expiry      time.Time
	Effective   time.Time
}

type capArea struct {
	AreaDesc string
	Codes    []string
	Geometry []byte
	Matches  bool
}

type areaContext struct {
	Geometry []byte
	Codes    []string
}

func parseRSS(body []byte) (rssDocument, error) {
	if unsafeXML(body) {
		return rssDocument{}, errInvalidTMD
	}
	var document rssDocument
	if xml.Unmarshal(stripBOM(body), &document) != nil || document.XMLName.Local != "rss" || document.Channel == nil || len(document.Channel.Items) > maxDocs {
		return rssDocument{}, errInvalidTMD
	}
	for _, item := range document.Channel.Items {
		if strings.TrimSpace(item.Link) == "" {
			return rssDocument{}, errInvalidTMD
		}
	}
	return document, nil
}

func parseCAP(body []byte, receivedAt time.Time) (capAlert, error) {
	if unsafeXML(body) {
		return capAlert{}, errInvalidTMD
	}
	var document capDocument
	if xml.Unmarshal(stripBOM(body), &document) != nil || document.XMLName.Local != "alert" || document.XMLName.Space != capNamespace {
		return capAlert{}, errInvalidTMD
	}
	identifier := strings.TrimSpace(document.Identifier)
	sender := strings.TrimSpace(document.Sender)
	status := strings.TrimSpace(document.Status)
	msgType := strings.TrimSpace(document.MsgType)
	references := strings.TrimSpace(document.References)
	if identifier == "" || sender == "" || status == "" || msgType == "" || len(identifier) > 256 || len(references) > 2048 {
		return capAlert{}, errInvalidTMD
	}
	if msgType == "Update" || msgType == "Cancel" {
		if !validReferences(references) {
			return capAlert{}, errInvalidTMD
		}
	} else if references != "" && !validReferences(references) {
		return capAlert{}, errInvalidTMD
	}
	if msgType != "Cancel" && len(document.Infos) == 0 {
		return capAlert{}, errInvalidTMD
	}
	sent, err := parseCAPTime(document.Sent)
	if err != nil {
		return capAlert{}, errInvalidTMD
	}
	infos := make([]capInfo, 0, len(document.Infos))
	for _, info := range document.Infos {
		var infoExpiry, infoEffective time.Time
		if strings.TrimSpace(info.Expires) != "" {
			infoExpiry, err = parseCAPTime(info.Expires)
			if err != nil {
				return capAlert{}, errInvalidTMD
			}
		} else if msgType != "Cancel" {
			return capAlert{}, errInvalidTMD
		}
		if strings.TrimSpace(info.Effective) != "" {
			infoEffective, err = parseCAPTime(info.Effective)
			if err != nil {
				return capAlert{}, errInvalidTMD
			}
		}
		if !infoExpiry.IsZero() && (sent.After(infoExpiry) || !infoEffective.IsZero() && !infoEffective.Before(infoExpiry)) {
			return capAlert{}, errInvalidTMD
		}
		headline, err := limitedText(info.Headline, 512)
		if err != nil {
			return capAlert{}, err
		}
		description, err := limitedText(info.Description, 4096)
		if err != nil {
			return capAlert{}, err
		}
		areas := make([]capArea, 0, len(info.Areas))
		for _, area := range info.Areas {
			areaDesc, err := limitedText(area.AreaDesc, 512)
			if err != nil {
				return capAlert{}, err
			}
			codes := make([]string, 0, len(area.Geocodes))
			for _, code := range area.Geocodes {
				value := strings.TrimSpace(code.Value)
				if strings.TrimSpace(code.ValueName) == "" || value == "" || len(value) > 128 {
					return capAlert{}, errInvalidTMD
				}
				codes = append(codes, value)
			}
			geometry, hasPolygon, err := polygonGeometry(area.Polygon)
			if err != nil {
				return capAlert{}, err
			}
			matches := containsCode(codes, "TH-26") || (hasPolygon && pointInPolygon(101, 14.1, geometry))
			areas = append(areas, capArea{AreaDesc: areaDesc, Codes: codes, Geometry: geometry, Matches: matches})
		}
		infos = append(infos, capInfo{Headline: headline, Description: description, Areas: areas, Expiry: infoExpiry, Effective: infoEffective})
	}
	if sent.IsZero() {
		return capAlert{}, errInvalidTMD
	}
	var expiry, effective time.Time
	_, local := matchingContext(infos)
	for _, info := range infos {
		if _, matches := matchingContext([]capInfo{info}); local && !matches {
			continue
		}
		if !info.Expiry.IsZero() && (expiry.IsZero() || info.Expiry.Before(expiry)) {
			expiry = info.Expiry
		}
		if info.Effective.After(effective) {
			effective = info.Effective
		}
	}
	_ = receivedAt
	timingUnconfirmed := !expiry.IsZero() && !effective.IsZero() && !effective.Before(expiry)
	if timingUnconfirmed {
		expiry, effective = time.Time{}, time.Time{}
	}
	return capAlert{Identifier: identifier, Sender: sender, Sent: sent, Status: status, MsgType: msgType, References: references, Expiry: expiry, Effective: effective, TimingUnconfirmed: timingUnconfirmed, Infos: infos}, nil
}

func hasDeclaredArea(infos []capInfo) bool {
	for _, info := range infos {
		if len(info.Areas) > 0 {
			return true
		}
	}
	return false
}

func matchingContext(infos []capInfo) (areaContext, bool) {
	for _, info := range infos {
		for _, area := range info.Areas {
			if area.Matches {
				return areaContext{Geometry: append([]byte(nil), area.Geometry...), Codes: append([]string(nil), area.Codes...)}, true
			}
		}
	}
	return areaContext{}, false
}

func makeRecord(alert capAlert, area areaContext) externaldata.Record {
	record := externaldata.Record{
		Key: alert.Identifier + ":" + alert.MsgType, Kind: externaldata.OfficialAlert, Provenance: "OFFICIAL",
		IssuedAt: &alert.Sent, Headline: alertHeadline(alert), Description: alertDescription(alert),
		AlertIdentifier: alert.Identifier, AlertMessageType: alert.MsgType, AlertReferences: referenceIdentifiers(alert.References),
		AreaCodes: area.Codes, Geometry: append([]byte(nil), area.Geometry...), Freshness: "UNKNOWN",
	}
	if !alert.Expiry.IsZero() {
		record.ValidTo = &alert.Expiry
	}
	if !alert.Effective.IsZero() {
		record.ValidFrom = &alert.Effective
	}
	if alert.TimingUnconfirmed {
		record.Reason = "CONFLICTING_TIME_WINDOWS"
	}
	return record
}

func alertHeadline(alert capAlert) string {
	for _, info := range alert.Infos {
		if info.Headline != "" {
			return info.Headline
		}
	}
	return ""
}

func alertDescription(alert capAlert) string {
	for _, info := range alert.Infos {
		if info.Description != "" {
			return info.Description
		}
	}
	return ""
}

func capLink(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || strings.Contains(value, "\\") {
		return "", errInvalidTMD
	}
	if strings.HasPrefix(value, "/") {
		value = tmdBase + value
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "https" || parsed.Host != "www.tmd.go.th" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || !strings.HasPrefix(parsed.Path, "/uploads/CAP/") || path.Clean(parsed.Path) != parsed.Path || path.Ext(parsed.Path) != ".xml" {
		return "", errInvalidTMD
	}
	if path.Base(parsed.Path) == ".xml" || strings.Contains(parsed.Path, "//") {
		return "", errInvalidTMD
	}
	return parsed.String(), nil
}

func linkPath(value string) string {
	parsed, _ := url.Parse(value)
	return parsed.EscapedPath()
}

func parseCAPTime(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(value))
	if err != nil {
		return time.Time{}, err
	}
	return parsed.UTC(), nil
}

func polygonGeometry(value string) ([]byte, bool, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, false, nil
	}
	parts := strings.Fields(value)
	if len(parts) < 4 || len(parts) > 1000 {
		return nil, false, errInvalidTMD
	}
	coordinates := make([][]float64, 0, len(parts))
	for _, part := range parts {
		values := strings.Split(part, ",")
		if len(values) != 2 {
			return nil, false, errInvalidTMD
		}
		latitude, errLat := parseFinite(values[0])
		longitude, errLon := parseFinite(values[1])
		if errLat != nil || errLon != nil || latitude < -90 || latitude > 90 || longitude < -180 || longitude > 180 {
			return nil, false, errInvalidTMD
		}
		coordinates = append(coordinates, []float64{longitude, latitude})
	}
	if coordinates[0][0] != coordinates[len(coordinates)-1][0] || coordinates[0][1] != coordinates[len(coordinates)-1][1] {
		return nil, false, errInvalidTMD
	}
	geometry := struct {
		Type        string        `json:"type"`
		Coordinates [][][]float64 `json:"coordinates"`
	}{Type: "Polygon", Coordinates: [][][]float64{coordinates}}
	encoded, err := jsonMarshal(geometry)
	if err != nil {
		return nil, false, errInvalidTMD
	}
	return encoded, true, nil
}

func pointInPolygon(longitude, latitude float64, geometry []byte) bool {
	var polygon struct {
		Coordinates [][][]float64 `json:"coordinates"`
	}
	if jsonUnmarshal(geometry, &polygon) != nil || len(polygon.Coordinates) == 0 {
		return false
	}
	ring := polygon.Coordinates[0]
	inside := false
	for index, current := range ring {
		previous := ring[(index+len(ring)-1)%len(ring)]
		if (current[1] > latitude) != (previous[1] > latitude) && longitude < (previous[0]-current[0])*(latitude-current[1])/(previous[1]-current[1])+current[0] {
			inside = !inside
		}
	}
	return inside
}

func containsCode(codes []string, wanted string) bool {
	for _, code := range codes {
		if strings.EqualFold(strings.TrimSpace(code), wanted) {
			return true
		}
	}
	return false
}

func referenceIdentifiers(value string) []string {
	identifiers := make([]string, 0)
	for _, token := range strings.Fields(value) {
		parts := strings.Split(token, ",")
		if len(parts) >= 2 && strings.TrimSpace(parts[1]) != "" {
			identifiers = append(identifiers, strings.TrimSpace(parts[1]))
		} else if strings.TrimSpace(token) != "" {
			identifiers = append(identifiers, strings.TrimSpace(token))
		}
	}
	return identifiers
}

func validReferences(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	for _, token := range strings.Fields(value) {
		parts := strings.Split(token, ",")
		if len(parts) != 3 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
			return false
		}
		if _, err := parseCAPTime(parts[2]); err != nil {
			return false
		}
	}
	return true
}

func limitedText(value string, limit int) (string, error) {
	value = strings.TrimSpace(value)
	if len([]rune(value)) > limit {
		return "", fmt.Errorf("TMD text exceeds limit")
	}
	return value, nil
}

func parseFinite(value string) (float64, error) {
	parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
		return 0, errInvalidTMD
	}
	return parsed, nil
}

func unsafeXML(body []byte) bool {
	text := strings.ToLower(string(stripBOM(body)))
	return strings.Contains(text, "<!doctype") || strings.Contains(text, "<!entity")
}

func stripBOM(body []byte) []byte {
	return bytes.TrimPrefix(body, []byte{0xef, 0xbb, 0xbf})
}

// Small wrappers keep the parser helpers easy to test without exposing encoding details.
func jsonMarshal(value any) ([]byte, error) {
	return json.Marshal(value)
}

func jsonUnmarshal(body []byte, value any) error {
	return json.Unmarshal(body, value)
}

var _ externaldata.Provider = (*Provider)(nil)
