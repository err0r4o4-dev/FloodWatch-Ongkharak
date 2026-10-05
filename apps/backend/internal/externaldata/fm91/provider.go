package fm91

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"
	_ "time/tzdata"
	"unicode"

	"golang.org/x/net/html"

	"github.com/floodwatch-ongkharak/backend/internal/externaldata"
	"github.com/floodwatch-ongkharak/backend/internal/ingestion"
)

const (
	mainURL          = "https://fm91bkk.com/news"
	fm91BaseURL      = "https://fm91bkk.com"
	maxCandidates    = 50
	maxListingLinks  = 500
	maxRawPayloads   = 64
	maxRawBytes      = 40 << 20
	maxPayloadBytes  = 10 << 20
	requestTimeout   = 90 * time.Second
	requestPacing    = 250 * time.Millisecond
	providerEndpoint = fm91BaseURL
)

var (
	articlePathPattern  = regexp.MustCompile(`^/newsarticle/([0-9]+)$`)
	categoryPathPattern = regexp.MustCompile(`^/newscategory/[0-9]+(?:/[^/]*)?$`)
	thaiDatePattern     = regexp.MustCompile(`(?:วันที่\s*)?([0-9๐-๙]{1,2})\s+(มกราคม|กุมภาพันธ์|มีนาคม|เมษายน|พฤษภาคม|มิถุนายน|กรกฎาคม|สิงหาคม|กันยายน|ตุลาคม|พฤศจิกายน|ธันวาคม)\s+([0-9๐-๙]{4})\s+เวลา\s+([0-9๐-๙]{1,2})[.:]([0-9๐-๙]{2})\s*น\.?`)
	clockPattern        = regexp.MustCompile(`เวลา\s+[0-9๐-๙]{1,2}[.:][0-9๐-๙]{2}\s*น\.?`)
	geoPattern          = regexp.MustCompile(`องครักษ์|นครนายก|มศว|คลอง\s*(?:14|15|16)|ทล\.\s*305|รังสิต\s*[-–—]\s*นครนายก`)
	roadPattern         = regexp.MustCompile(`ทล\.\s*305|รังสิต\s*[-–—]\s*นครนายก`)
	eventPattern        = regexp.MustCompile(`น้ำท่วม|น้ำล้น|น้ำขัง|รถเล็ก|ปิดการจราจร|น้ำป่า|ดินสไลด์`)
	waterLevelPattern   = regexp.MustCompile(`(?:ระดับน้ำ|น้ำ(?:ท่วมขัง|ท่วม|ล้น|สูง)?)[^\n.!?]{0,80}?[0-9๐-๙]+(?:[.,][0-9๐-๙]+)?\s*(?:เมตร|ม\.?|เซนติเมตร|ซม\.?)`)
	travelPattern       = regexp.MustCompile(`(?:รถเล็ก|ปิดการจราจร|เลี่ยง|หลีกเลี่ยง)[^\n.!?]*`)
)

var thaiMonths = map[string]time.Month{
	"มกราคม": time.January, "กุมภาพันธ์": time.February, "มีนาคม": time.March,
	"เมษายน": time.April, "พฤษภาคม": time.May, "มิถุนายน": time.June,
	"กรกฎาคม": time.July, "สิงหาคม": time.August, "กันยายน": time.September,
	"ตุลาคม": time.October, "พฤศจิกายน": time.November, "ธันวาคม": time.December,
}

type Provider struct {
	fetcher externaldata.Fetcher
	Clock   func() time.Time
}

type candidate struct {
	URL  string
	Text string
}

func New() (*Provider, error) {
	client, err := ingestion.NewHTTPClient(providerEndpoint, ingestion.HTTPOptions{
		Attempts: 3,
		Timeout:  10 * time.Second,
		MaxBytes: maxPayloadBytes,
	})
	if err != nil {
		return nil, err
	}
	return &Provider{fetcher: fixedFetcher{client: client}, Clock: time.Now}, nil
}

func NewWithFetcher(fetcher externaldata.Fetcher) *Provider {
	return &Provider{fetcher: fetcher, Clock: time.Now}
}

func (provider *Provider) Fetch(ctx context.Context) (externaldata.Batch, error) {
	if provider == nil || provider.fetcher == nil {
		return externaldata.Batch{}, errors.New("FM91 fetcher is unavailable")
	}
	requestContext, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	var lastRequest time.Time

	retrievedAt := time.Now().UTC()
	if provider.Clock != nil {
		retrievedAt = provider.Clock().UTC()
	}
	listingBody, err := provider.fetchPaced(requestContext, mainURL, &lastRequest)
	if err != nil {
		return externaldata.Batch{}, fmt.Errorf("FM91 listing fetch failed: %w", err)
	}
	if err := validPayload(listingBody, 0); err != nil {
		return externaldata.Batch{}, err
	}
	candidates, err := parseListing(listingBody)
	if err != nil {
		return externaldata.Batch{}, err
	}

	batch := externaldata.Batch{
		RetrievedAt: retrievedAt,
		Raw: []externaldata.RawPayload{{
			URL: mainURL, ContentType: "text/html", Body: append([]byte(nil), listingBody...),
		}},
	}
	rawTotal := len(listingBody)
	for _, item := range candidates {
		body, fetchErr := provider.fetchPaced(requestContext, item.URL, &lastRequest)
		if fetchErr != nil {
			return externaldata.Batch{}, fmt.Errorf("FM91 article fetch failed: %w", fetchErr)
		}
		if err := validPayload(body, rawTotal); err != nil {
			return externaldata.Batch{}, err
		}
		rawTotal += len(body)
		article, parseErr := parseArticle(body, item.URL)
		if parseErr != nil {
			return externaldata.Batch{}, parseErr
		}
		batch.Raw = append(batch.Raw, externaldata.RawPayload{
			URL: item.URL, ContentType: "text/html", Body: append([]byte(nil), body...),
		})
		if !article.Relevant {
			continue
		}
		record := makeRecord(article, retrievedAt)
		if !externaldata.ValidNewsRecord(record, retrievedAt) {
			return externaldata.Batch{}, errors.New("FM91 article record is invalid")
		}
		batch.Records = append(batch.Records, record)
	}
	if len(batch.Raw) > maxRawPayloads || rawTotal > maxRawBytes {
		return externaldata.Batch{}, errors.New("FM91 raw evidence bounds exceeded")
	}
	return batch, nil
}

type parsedArticle struct {
	ID           string
	URL          string
	Category     string
	Headline     string
	Description  string
	IssuedAt     *time.Time
	ImageURLs    []string
	Locations    []string
	Roads        []string
	EventTimes   []string
	WaterLevels  []string
	TravelAdvice []string
	Keywords     []string
	ObservedAt   *time.Time
	Relevant     bool
}

func parseListing(body []byte) ([]candidate, error) {
	document, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return nil, errors.New("FM91 listing HTML is malformed")
	}
	root := findNode(document, func(node *html.Node) bool {
		return node.Type == html.ElementNode && node.Data == "div" && hasClasses(node, "container", "mt-2")
	})
	if root == nil {
		return nil, errors.New("FM91 listing HTML changed")
	}
	heading := findDescendant(root, "h1")
	if heading == nil || normalizeWhitespace(nodeText(heading, false)) != "รวมข่าว" || findListingColumn(root) == nil {
		return nil, errors.New("FM91 listing HTML changed")
	}

	seen := make(map[string]struct{})
	candidates := make([]candidate, 0)
	for _, anchor := range descendants(root, "a") {
		href := strings.TrimSpace(attribute(anchor, "href"))
		if !strings.Contains(href, "/newsarticle/") {
			continue
		}
		canonical, _, err := canonicalArticleURL(href)
		if err != nil {
			return nil, errors.New("FM91 listing contains an unsafe article URL")
		}
		if _, exists := seen[canonical]; exists {
			continue
		}
		seen[canonical] = struct{}{}
		if len(seen) > maxListingLinks {
			return nil, errors.New("FM91 candidate limit exceeded")
		}
		text := nodeText(anchor, true)
		if text == "" {
			return nil, errors.New("FM91 listing article has no title")
		}
		if !matchesPrefilter(text) {
			continue
		}
		candidates = append(candidates, candidate{URL: canonical, Text: text})
		if len(candidates) > maxCandidates {
			return nil, errors.New("FM91 candidate limit exceeded")
		}
	}
	if len(seen) == 0 {
		return nil, errors.New("FM91 listing has no article links")
	}
	return candidates, nil
}

func parseArticle(body []byte, requestedURL string) (parsedArticle, error) {
	document, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return parsedArticle{}, errors.New("FM91 article HTML is malformed")
	}
	requested, requestedID, err := canonicalArticleURL(requestedURL)
	if err != nil {
		return parsedArticle{}, errors.New("FM91 article URL is unsafe")
	}
	metaURL := metaContent(document, "og:url")
	canonical, articleID, err := canonicalArticleURL(metaURL)
	if err != nil || canonical != requested || articleID != requestedID {
		return parsedArticle{}, errors.New("FM91 article canonical URL is invalid")
	}

	headingContainer := findNode(document, func(node *html.Node) bool {
		return node.Type == html.ElementNode && node.Data == "div" && hasClasses(node, "container", "mt-3")
	})
	if headingContainer == nil {
		return parsedArticle{}, errors.New("FM91 article header changed")
	}
	headlineNode := findDescendant(headingContainer, "h1")
	if headlineNode == nil {
		return parsedArticle{}, errors.New("FM91 article headline is missing")
	}
	headline := nodeText(headlineNode, false)
	if headline == "" {
		return parsedArticle{}, errors.New("FM91 article headline is empty")
	}
	categoryNode := findNode(headingContainer, func(node *html.Node) bool {
		if node.Type != html.ElementNode || node.Data != "a" || strings.TrimSpace(nodeText(node, false)) == "" {
			return false
		}
		path, ok := trustedPath(attribute(node, "href"))
		return ok && categoryPathPattern.MatchString(path)
	})
	if categoryNode == nil {
		return parsedArticle{}, errors.New("FM91 article category is missing")
	}

	var issuedAt *time.Time
	if publication := findNode(headingContainer, func(node *html.Node) bool {
		return node.Type == html.ElementNode && node.Data == "p" && hasClass(node, "mt-3")
	}); publication != nil {
		if timestamp, ok := parseThaiDateTime(nodeText(publication, false)); ok {
			issuedAt = &timestamp
		}
	}

	bodyNode := findNode(document, func(node *html.Node) bool {
		return node.Type == html.ElementNode && node.Data == "div" && hasClass(node, "news-pra")
	})
	if bodyNode == nil {
		return parsedArticle{}, errors.New("FM91 article body is missing")
	}
	description := bodyText(bodyNode)
	if description == "" {
		return parsedArticle{}, errors.New("FM91 article body is empty")
	}
	if len([]rune(description)) > 30000 {
		return parsedArticle{}, errors.New("FM91 article body is too large")
	}

	articleText := headline + "\n" + description
	locations := uniqueMatches(geoPattern, articleText)
	roads := uniqueMatches(roadPattern, articleText)
	eventKeywords := uniqueMatches(eventPattern, articleText)
	waterLevels := uniqueMatches(waterLevelPattern, description)
	eventTimes := uniqueMatches(thaiDatePattern, description)
	if len(eventTimes) == 0 {
		eventTimes = uniqueMatches(clockPattern, description)
	}
	travelAdvice := uniqueMatches(travelPattern, description)
	keywords := appendUnique(nil, locations...)
	keywords = appendUnique(keywords, eventKeywords...)
	observedAt := eventObservationTime(eventTimes)
	images := articleImages(document, bodyNode)

	return parsedArticle{
		ID: articleID, URL: canonical, Category: nodeText(categoryNode, false), Headline: headline,
		Description: description, IssuedAt: issuedAt, ImageURLs: images, Locations: locations,
		Roads: roads, EventTimes: eventTimes, WaterLevels: waterLevels, TravelAdvice: travelAdvice,
		Keywords: keywords, ObservedAt: observedAt, Relevant: containsAny(locations, eventKeywords),
	}, nil
}

func makeRecord(article parsedArticle, retrievedAt time.Time) externaldata.Record {
	observedAt := article.ObservedAt
	if observedAt != nil && observedAt.After(retrievedAt) {
		observedAt = nil
	}
	freshness := "FRESH"
	if article.IssuedAt == nil {
		freshness = "UNKNOWN"
	}
	return externaldata.Record{
		Key: article.ID, Kind: externaldata.NewsReport, Provenance: "MEDIA",
		IssuedAt: article.IssuedAt, ObservedAt: observedAt, Headline: article.Headline, Description: article.Description,
		Freshness: freshness,
		News: &externaldata.NewsArticle{
			ArticleID: article.ID, URL: article.URL, Category: article.Category, ImageURLs: article.ImageURLs,
			Locations: article.Locations, Roads: article.Roads, EventTimes: article.EventTimes,
			WaterLevels: article.WaterLevels, TravelAdvice: article.TravelAdvice, MatchedKeywords: article.Keywords,
		},
	}
}

func validPayload(body []byte, currentTotal int) error {
	if len(body) == 0 || len(body) > maxPayloadBytes || currentTotal+len(body) > maxRawBytes {
		return errors.New("FM91 raw evidence bounds exceeded")
	}
	return nil
}

func canonicalArticleURL(value string) (string, string, error) {
	path, ok := trustedPath(value)
	if !ok {
		return "", "", errors.New("unsafe FM91 article URL")
	}
	matches := articlePathPattern.FindStringSubmatch(path)
	if len(matches) != 2 || !externaldata.ValidNewsURL(fm91BaseURL+path, matches[1]) {
		return "", "", errors.New("invalid FM91 article URL")
	}
	return fm91BaseURL + path, matches[1], nil
}

func trustedPath(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if strings.ContainsAny(value, "?#") {
		return "", false
	}
	if strings.HasPrefix(value, "/") && !strings.HasPrefix(value, "//") {
		value = fm91BaseURL + value
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "https" || parsed.Host != "fm91bkk.com" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || strings.Contains(parsed.Path, "\\") {
		return "", false
	}
	return parsed.Path, true
}

func articleImages(document, bodyNode *html.Node) []string {
	images := make([]string, 0)
	if image := metaContent(document, "og:image"); image != "" {
		if imageURL, ok := normalizeImageURL(image); ok {
			images = appendUnique(images, imageURL)
		}
	}
	for _, node := range descendants(bodyNode, "img") {
		for _, name := range []string{"src", "data-src"} {
			value := strings.TrimSpace(attribute(node, name))
			if imageURL, ok := normalizeImageURL(value); ok {
				images = appendUnique(images, imageURL)
				break
			}
		}
	}
	return images
}

func normalizeImageURL(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if value == "" || strings.ContainsAny(value, "?#") || strings.HasPrefix(value, "//") {
		return "", false
	}
	if !strings.HasPrefix(value, "https://") {
		if strings.HasPrefix(value, "/") {
			value = fm91BaseURL + value
		} else {
			value = fm91BaseURL + "/" + value
		}
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "https" || parsed.Host != "fm91bkk.com" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || strings.Contains(parsed.Path, "\\") || (!strings.HasPrefix(parsed.Path, "/upload/") && !strings.HasPrefix(parsed.Path, "/ckfinder/userfiles/images/")) || !externaldata.ValidNewsImageURL(parsed.String()) {
		return "", false
	}
	return parsed.String(), true
}

func parseThaiDateTime(value string) (time.Time, bool) {
	matches := thaiDatePattern.FindStringSubmatch(normalizeWhitespace(value))
	if len(matches) != 6 {
		return time.Time{}, false
	}
	day, ok := parseThaiNumber(matches[1])
	if !ok {
		return time.Time{}, false
	}
	year, ok := parseThaiNumber(matches[3])
	if !ok || year < 2400 {
		return time.Time{}, false
	}
	hour, ok := parseThaiNumber(matches[4])
	if !ok {
		return time.Time{}, false
	}
	minute, ok := parseThaiNumber(matches[5])
	if !ok || hour > 23 || minute > 59 {
		return time.Time{}, false
	}
	month, ok := thaiMonths[matches[2]]
	if !ok {
		return time.Time{}, false
	}
	location, err := time.LoadLocation("Asia/Bangkok")
	if err != nil {
		return time.Time{}, false
	}
	timestamp := time.Date(year-543, month, day, hour, minute, 0, 0, location)
	if timestamp.Day() != day || timestamp.Month() != month || timestamp.Year() != year-543 {
		return time.Time{}, false
	}
	return timestamp.UTC(), true
}

func parseThaiNumber(value string) (int, bool) {
	value = strings.Map(func(r rune) rune {
		if r >= '๐' && r <= '๙' {
			return '0' + (r - '๐')
		}
		return r
	}, value)
	var number int
	for _, r := range value {
		if r < '0' || r > '9' {
			return 0, false
		}
		number = number*10 + int(r-'0')
	}
	return number, value != ""
}

func eventObservationTime(eventTimes []string) *time.Time {
	if len(eventTimes) != 1 {
		return nil
	}
	if timestamp, ok := parseThaiDateTime(eventTimes[0]); ok {
		return &timestamp
	}
	return nil
}

func matchesPrefilter(value string) bool {
	return geoPattern.MatchString(value) || eventPattern.MatchString(value)
}

func containsAny(first, second []string) bool { return len(first) > 0 && len(second) > 0 }

func uniqueMatches(pattern *regexp.Regexp, value string) []string {
	result := make([]string, 0)
	for _, match := range pattern.FindAllString(value, -1) {
		match = normalizeWhitespace(match)
		if match != "" {
			result = appendUnique(result, match)
		}
	}
	return result
}

func appendUnique(values []string, additions ...string) []string {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		seen[value] = struct{}{}
	}
	for _, value := range additions {
		if value == "" {
			continue
		}
		if _, exists := seen[value]; !exists {
			values = append(values, value)
			seen[value] = struct{}{}
		}
	}
	return values
}

func bodyText(root *html.Node) string {
	var paragraphs []string
	var walk func(*html.Node, *strings.Builder)
	walk = func(node *html.Node, current *strings.Builder) {
		if node.Type == html.ElementNode && (node.Data == "script" || node.Data == "style" || node.Data == "noscript" || node.Data == "template" || hasClass(node, "product-gallery") || node.Data == "figcaption") {
			return
		}
		if node.Type == html.TextNode {
			current.WriteString(node.Data)
			return
		}
		if node.Type == html.ElementNode && (node.Data == "p" || node.Data == "div" || node.Data == "br" || node.Data == "li") && current.Len() > 0 {
			if text := normalizeWhitespace(current.String()); text != "" {
				paragraphs = append(paragraphs, text)
			}
			current.Reset()
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child, current)
		}
		if node.Type == html.ElementNode && (node.Data == "p" || node.Data == "div" || node.Data == "li") && current.Len() > 0 {
			if text := normalizeWhitespace(current.String()); text != "" {
				paragraphs = append(paragraphs, text)
			}
			current.Reset()
		}
	}
	var current strings.Builder
	for child := root.FirstChild; child != nil; child = child.NextSibling {
		walk(child, &current)
	}
	if current.Len() > 0 {
		if text := normalizeWhitespace(current.String()); text != "" {
			paragraphs = append(paragraphs, text)
		}
	}
	return strings.Join(paragraphs, "\n\n")
}

func nodeText(root *html.Node, includeAlt bool) string {
	var builder strings.Builder
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode && (node.Data == "script" || node.Data == "style" || node.Data == "noscript" || node.Data == "template") {
			return
		}
		if node.Type == html.TextNode {
			builder.WriteString(node.Data)
			builder.WriteByte(' ')
			return
		}
		if includeAlt && node.Type == html.ElementNode && node.Data == "img" {
			builder.WriteString(attribute(node, "alt"))
			builder.WriteByte(' ')
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
	return normalizeWhitespace(builder.String())
}

func normalizeWhitespace(value string) string {
	return strings.Join(strings.FieldsFunc(value, unicode.IsSpace), " ")
}

func metaContent(root *html.Node, property string) string {
	for _, node := range descendants(root, "meta") {
		if strings.EqualFold(strings.TrimSpace(attribute(node, "property")), property) {
			return strings.TrimSpace(attribute(node, "content"))
		}
	}
	return ""
}

func findNode(root *html.Node, predicate func(*html.Node) bool) *html.Node {
	if predicate(root) {
		return root
	}
	for child := root.FirstChild; child != nil; child = child.NextSibling {
		if found := findNode(child, predicate); found != nil {
			return found
		}
	}
	return nil
}

func findDescendant(root *html.Node, tag string) *html.Node {
	return findNode(root, func(node *html.Node) bool { return node.Type == html.ElementNode && node.Data == tag })
}

func descendants(root *html.Node, tag string) []*html.Node {
	result := make([]*html.Node, 0)
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode && node.Data == tag {
			result = append(result, node)
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	for child := root.FirstChild; child != nil; child = child.NextSibling {
		walk(child)
	}
	return result
}

func attribute(node *html.Node, key string) string {
	for _, attr := range node.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}

func hasClass(node *html.Node, wanted string) bool {
	for _, className := range strings.Fields(attribute(node, "class")) {
		if className == wanted {
			return true
		}
	}
	return false
}

func hasClasses(node *html.Node, wanted ...string) bool {
	for _, className := range wanted {
		if !hasClass(node, className) {
			return false
		}
	}
	return true
}

func findListingColumn(root *html.Node) *html.Node {
	return findNode(root, func(node *html.Node) bool {
		return node.Type == html.ElementNode && node.Data == "div" && hasClasses(node, "col-xl-8", "col-lg-8", "col-md-12", "col-12")
	})
}

func (provider *Provider) fetchPaced(ctx context.Context, requestURL string, lastRequest *time.Time) ([]byte, error) {
	if !lastRequest.IsZero() {
		delay := requestPacing - time.Since(*lastRequest)
		if delay > 0 {
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, ctx.Err()
			case <-timer.C:
			}
		}
	}
	body, err := provider.fetcher.Fetch(ctx, requestURL)
	*lastRequest = time.Now()
	return body, err
}

type fixedFetcher struct{ client *ingestion.HTTPClient }

func (fetcher fixedFetcher) Fetch(ctx context.Context, requestURL string) ([]byte, error) {
	if requestURL == mainURL {
		return fetcher.client.Fetch(ctx, "/news")
	}
	canonical, _, err := canonicalArticleURL(requestURL)
	if err != nil || canonical != requestURL {
		return nil, errors.New("FM91 URL is not allowlisted")
	}
	parsed, _ := url.Parse(canonical)
	return fetcher.client.Fetch(ctx, parsed.EscapedPath())
}

var _ externaldata.Provider = (*Provider)(nil)
