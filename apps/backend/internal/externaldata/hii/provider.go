package hii

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/externaldata"
	"github.com/floodwatch-ongkharak/backend/internal/ingestion"
)

const (
	metadataURL = "https://live1.hii.or.th/product/latest/wrfroms/tiff/description_wrfroms.json"
	modelName   = "HII_WRF_ROMS"
	refLat      = 14.1
	refLon      = 101.0
)

var tifURLs = []string{
	"https://live1.hii.or.th/product/latest/wrfroms/tiff/fcst_rain24hr_d03_day1.tif",
	"https://live1.hii.or.th/product/latest/wrfroms/tiff/fcst_rain24hr_d03_day2.tif",
	"https://live1.hii.or.th/product/latest/wrfroms/tiff/fcst_rain24hr_d03_day3.tif",
}

type Provider struct {
	fetcher externaldata.Fetcher
	now     func() time.Time
}

func New() (*Provider, error) {
	client, err := ingestion.NewHTTPClient("https://live1.hii.or.th", ingestion.HTTPOptions{
		Attempts: 2, Timeout: 10 * time.Second, MaxBytes: 5 << 20,
	})
	if err != nil {
		return nil, err
	}
	return &Provider{fetcher: fixedFetcher{client: client}, now: time.Now}, nil
}

func NewWithFetcher(fetcher externaldata.Fetcher) (*Provider, error) {
	if fetcher == nil {
		return nil, errors.New("HII fetcher is required")
	}
	return &Provider{fetcher: fetcher, now: time.Now}, nil
}

type metadataEntry struct {
	Date     string `json:"date"`
	FcstTime string `json:"fcst_time"`
	Filename string `json:"filename"`
}

func (provider *Provider) Fetch(ctx context.Context) (externaldata.Batch, error) {
	metadataBody, err := provider.fetcher.Fetch(ctx, metadataURL)
	if err != nil {
		return externaldata.Batch{}, fmt.Errorf("HII metadata fetch failed: %w", err)
	}
	entries, err := parseMetadata(metadataBody)
	if err != nil {
		return externaldata.Batch{}, err
	}
	retrievedAt := time.Now().UTC()
	if provider.now != nil {
		retrievedAt = provider.now().UTC()
	}
	batch := externaldata.Batch{
		RetrievedAt: retrievedAt,
		Raw:         []externaldata.RawPayload{{URL: metadataURL, ContentType: "application/json", Body: append([]byte(nil), metadataBody...)}},
	}
	for index, entry := range entries {
		body, fetchErr := provider.fetcher.Fetch(ctx, entry.Filename)
		if fetchErr != nil {
			return externaldata.Batch{}, fmt.Errorf("HII TIFF fetch failed: %w", fetchErr)
		}
		image, decodeErr := decodeTIFF(body)
		if decodeErr != nil {
			return externaldata.Batch{}, fmt.Errorf("HII TIFF %d is invalid: %w", index+1, decodeErr)
		}
		batch.Raw = append(batch.Raw, externaldata.RawPayload{URL: entry.Filename, ContentType: "image/tiff", Body: append([]byte(nil), body...)})
		record, recordErr := makeRecord(entry, image, index)
		if recordErr != nil {
			return externaldata.Batch{}, recordErr
		}
		batch.Records = append(batch.Records, record)
	}
	return batch, nil
}

func parseMetadata(body []byte) ([]metadataEntry, error) {
	var groups [][]metadataEntry
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&groups); err != nil {
		return nil, errors.New("invalid HII metadata response")
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF || len(groups) != len(tifURLs) {
		return nil, errors.New("invalid HII metadata response")
	}
	entries := make([]metadataEntry, len(groups))
	for index, group := range groups {
		if len(group) != 1 || group[0].Filename != tifURLs[index] || !validDate(group[0].Date) || !validClock(group[0].FcstTime) {
			return nil, errors.New("HII metadata contains an unsupported forecast entry")
		}
		entries[index] = group[0]
	}
	return entries, nil
}

func makeRecord(entry metadataEntry, image tiffImage, index int) (externaldata.Record, error) {
	row, col, err := image.referenceCell(refLat, refLon)
	if err != nil {
		return externaldata.Record{}, err
	}
	_, present := image.valueAt(row, col)
	latitude, longitude := image.coordinate(row, col)
	reason := fmt.Sprintf("HII rainfall unit, timezone, and accumulation window are unverified (source date %s %s)", entry.Date, entry.FcstTime)
	if !present {
		reason += "; selected cell is NoData"
	}
	return externaldata.Record{
		Key:        fmt.Sprintf("hii:%s:%s:%02d", modelName, entry.Date, index+1),
		Kind:       externaldata.Forecast,
		Provenance: "MODEL",
		Model:      modelName,
		Metric:     "precipitation",
		Latitude:   floatPtr(latitude),
		Longitude:  floatPtr(longitude),
		Freshness:  "FRESH",
		Reason:     reason,
		Value:      nil,
	}, nil
}

func floatPtr(value float64) *float64 { return &value }
func validDate(value string) bool     { _, err := time.Parse("2006-01-02", value); return err == nil }
func validClock(value string) bool    { _, err := time.Parse("15:04", value); return err == nil }

type fixedFetcher struct{ client *ingestion.HTTPClient }

func (fetcher fixedFetcher) Fetch(ctx context.Context, requestURL string) ([]byte, error) {
	allowed := requestURL == metadataURL
	for _, tifURL := range tifURLs {
		allowed = allowed || requestURL == tifURL
	}
	if !allowed {
		return nil, errors.New("HII URL is not allowed")
	}
	parsed, err := url.Parse(requestURL)
	if err != nil || parsed.Host != "live1.hii.or.th" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, errors.New("HII URL is not allowed")
	}
	return fetcher.client.Fetch(ctx, parsed.EscapedPath())
}
