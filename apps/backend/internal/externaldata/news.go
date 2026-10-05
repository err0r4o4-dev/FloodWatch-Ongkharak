package externaldata

import (
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

var newsArticleID = regexp.MustCompile(`^[1-9][0-9]{0,18}$`)

func ValidNewsURL(raw, articleID string) bool {
	return newsArticleID.MatchString(articleID) && raw == "https://fm91bkk.com/newsarticle/"+articleID
}

func ValidNewsImageURL(raw string) bool {
	parsed, err := url.Parse(raw)
	return err == nil && parsed.Scheme == "https" && parsed.Host == "fm91bkk.com" && parsed.User == nil && parsed.RawQuery == "" && parsed.Fragment == "" && !strings.Contains(parsed.Path, "..") && (strings.HasPrefix(parsed.Path, "/upload/") || strings.HasPrefix(parsed.Path, "/ckfinder/userfiles/images/"))
}

func ValidNewsRecord(record Record, retrievedAt time.Time) bool {
	news := record.News
	if record.Kind != NewsReport || record.Provenance != "MEDIA" || news == nil || record.Key != news.ArticleID || !ValidNewsURL(news.URL, news.ArticleID) || strings.TrimSpace(record.Headline) == "" || strings.TrimSpace(record.Description) == "" || len(record.Headline) > 2000 || len(record.Description) > 30000 || len(news.Category) > 200 || record.Station != nil || record.Value != nil || record.Metric != "" || record.Unit != "" || record.Datum != "" || len(record.Geometry) != 0 {
		return false
	}
	for _, text := range []string{record.Headline, record.Description, news.Category} {
		if !utf8.ValidString(text) || strings.ContainsRune(text, 0) {
			return false
		}
	}
	for _, at := range []*time.Time{record.IssuedAt, record.ObservedAt} {
		if at != nil && (at.IsZero() || at.After(retrievedAt.Add(time.Minute))) {
			return false
		}
	}
	if (record.Latitude == nil) != (record.Longitude == nil) || record.Latitude != nil && !validCoordinate(*record.Latitude, *record.Longitude) {
		return false
	}
	if len(news.ImageURLs) > 20 {
		return false
	}
	for _, image := range news.ImageURLs {
		if !ValidNewsImageURL(image) {
			return false
		}
	}
	for _, values := range [][]string{news.Locations, news.Roads, news.EventTimes, news.WaterLevels, news.TravelAdvice, news.MatchedKeywords} {
		if len(values) > 50 {
			return false
		}
		for _, value := range values {
			if len(value) > 2000 || !utf8.ValidString(value) || strings.ContainsRune(value, 0) {
				return false
			}
		}
	}
	return true
}
