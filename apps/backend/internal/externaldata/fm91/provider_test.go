package fm91

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/externaldata"
)

type fixtureFetcher struct {
	responses map[string][]byte
	errs      map[string]error
	calls     []string
}

type contextFetcher struct{}

func (contextFetcher) Fetch(ctx context.Context, _ string) ([]byte, error) {
	return nil, ctx.Err()
}

func (fetcher *fixtureFetcher) Fetch(_ context.Context, requestURL string) ([]byte, error) {
	fetcher.calls = append(fetcher.calls, requestURL)
	if err := fetcher.errs[requestURL]; err != nil {
		return nil, err
	}
	body, ok := fetcher.responses[requestURL]
	if !ok {
		return nil, fmt.Errorf("unexpected fetch %s", requestURL)
	}
	return append([]byte(nil), body...), nil
}

func TestProviderNormalizesRelevantArticleAndDeduplicatesListLinks(t *testing.T) {
	fetcher := &fixtureFetcher{responses: map[string][]byte{
		mainURL:             []byte(listingFixture()),
		articleURL("79953"): []byte(articleFixture()),
	}}
	provider := NewWithFetcher(fetcher)
	provider.Clock = func() time.Time { return time.Date(2026, 10, 5, 18, 0, 0, 0, bangkokLocation) }

	batch, err := provider.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(batch.Records) != 1 {
		t.Fatalf("records = %d, want 1", len(batch.Records))
	}
	if len(fetcher.calls) != 2 || fetcher.calls[1] != articleURL("79953") {
		t.Fatalf("fetch calls = %#v, want one deduplicated article fetch", fetcher.calls)
	}
	record := batch.Records[0]
	if record.Key != "79953" || record.Kind != externaldata.NewsReport || record.Provenance != "MEDIA" {
		t.Fatalf("record identity = %#v", record)
	}
	if record.Headline != "องครักษ์ น้ำท่วมถนน รังสิต-นครนายก" || record.News == nil {
		t.Fatalf("headline/news = %q/%#v", record.Headline, record.News)
	}
	if record.Description != "เกิดน้ำท่วมขังสูง 30 เซนติเมตร บริเวณคลอง 14 บน ทล.305 รถเล็กผ่านลำบาก" {
		t.Fatalf("description = %q", record.Description)
	}
	if record.Description == "" || strings.Contains(record.Description, "caption") || strings.Contains(record.Description, "evil") {
		t.Fatalf("description retained excluded content: %q", record.Description)
	}
	if record.IssuedAt == nil || !record.IssuedAt.Equal(time.Date(2026, 10, 5, 17, 2, 0, 0, bangkokLocation).UTC()) {
		t.Fatalf("issuedAt = %v", record.IssuedAt)
	}
	if record.ObservedAt != nil {
		t.Fatalf("observedAt = %v, publication time must not become event time", record.ObservedAt)
	}
	if record.Latitude != nil || record.Longitude != nil || record.Freshness != "FRESH" {
		t.Fatalf("coordinates/freshness = %v/%v/%q", record.Latitude, record.Longitude, record.Freshness)
	}
	if record.News.ArticleID != "79953" || record.News.URL != articleURL("79953") || record.News.Category != "ภูมิภาค" {
		t.Fatalf("news identity = %#v", record.News)
	}
	assertContains(t, record.News.ImageURLs, "https://fm91bkk.com/upload/79953-main.jpg")
	assertContains(t, record.News.ImageURLs, "https://fm91bkk.com/upload/tb_news/webp/2026-10/05/79953/gallery.webp")
	assertContains(t, record.News.Locations, "องครักษ์")
	assertContains(t, record.News.Locations, "คลอง 14")
	assertContains(t, record.News.Roads, "ทล.305")
	assertContains(t, record.News.Roads, "รังสิต-นครนายก")
	assertContains(t, record.News.WaterLevels, "น้ำท่วมขังสูง 30 เซนติเมตร")
	assertContains(t, record.News.TravelAdvice, "รถเล็กผ่านลำบาก")
	assertContains(t, record.News.MatchedKeywords, "องครักษ์")
	assertContains(t, record.News.MatchedKeywords, "น้ำท่วม")
	if len(batch.Raw) != 2 || batch.Raw[0].URL != mainURL || batch.Raw[1].URL != articleURL("79953") {
		t.Fatalf("raw payloads = %#v", batch.Raw)
	}
}

func TestProviderBoundsFilteredCandidatesSeparatelyFromListingLinks(t *testing.T) {
	var listing strings.Builder
	listing.WriteString(`<html><body><div class="container mt-2"><h1>รวมข่าว</h1><div class="col-xl-8 col-lg-8 col-md-12 col-12">`)
	for i := 1; i <= 51; i++ {
		fmt.Fprintf(&listing, `<a href="/newsarticle/%d"><img alt="ข่าวทั่วไป"><b>ข่าวทั่วไป</b></a>`, i)
	}
	listing.WriteString(`<a href="/newsarticle/79953"><img alt="องครักษ์ น้ำท่วม"><b>องครักษ์ น้ำท่วม</b></a></div></div></body></html>`)
	fetcher := &fixtureFetcher{responses: map[string][]byte{
		mainURL:             []byte(listing.String()),
		articleURL("79953"): []byte(articleFixture()),
	}}
	provider := NewWithFetcher(fetcher)
	provider.Clock = func() time.Time { return time.Date(2026, 10, 5, 18, 0, 0, 0, bangkokLocation) }
	batch, err := provider.Fetch(context.Background())
	if err != nil || len(batch.Records) != 1 {
		t.Fatalf("prefiltered candidate result = %#v, err=%v", batch.Records, err)
	}
}

func TestProviderRequiresGeographyAndEventInArticleBody(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "wrong province", body: articleWithBody("น้ำท่วมในชลบุรี", "ฝนตกหนักในชลบุรี")},
		{name: "sidebar only geography", body: articleWithSidebar("น้ำท่วมในพื้นที่อื่น", "องครักษ์")},
		{name: "geography without event", body: articleWithBody("รายงานสภาพอากาศ", "องครักษ์")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fetcher := &fixtureFetcher{responses: map[string][]byte{
				mainURL:             []byte(listingFor("79953", "น้ำท่วม องครักษ์")),
				articleURL("79953"): []byte(test.body),
			}}
			provider := NewWithFetcher(fetcher)
			provider.Clock = func() time.Time { return time.Date(2026, 10, 5, 12, 0, 0, 0, bangkokLocation) }
			batch, err := provider.Fetch(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if len(batch.Records) != 0 {
				t.Fatalf("records = %d, want no relevant records", len(batch.Records))
			}
		})
	}
}

func TestProviderLeavesMissingPublicationFreshnessUnknownAndDoesNotInferEventTime(t *testing.T) {
	fetcher := &fixtureFetcher{responses: map[string][]byte{
		mainURL:             []byte(listingFor("79953", "น้ำท่วม องครักษ์")),
		articleURL("79953"): []byte(articleWithBody("องครักษ์ น้ำท่วมขัง", "ระดับน้ำเพิ่มขึ้นแต่ไม่มีหน่วย")),
	}}
	provider := NewWithFetcher(fetcher)
	provider.Clock = func() time.Time { return time.Date(2026, 10, 5, 12, 0, 0, 0, bangkokLocation) }
	batch, err := provider.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(batch.Records) != 1 {
		t.Fatalf("records = %d, want 1", len(batch.Records))
	}
	record := batch.Records[0]
	if record.IssuedAt != nil || record.ObservedAt != nil || record.Freshness != "UNKNOWN" {
		t.Fatalf("timestamps/freshness = %v/%v/%q", record.IssuedAt, record.ObservedAt, record.Freshness)
	}
	if len(record.News.WaterLevels) != 0 || len(record.News.EventTimes) != 0 {
		t.Fatalf("inferred measurements/times = %#v/%#v", record.News.WaterLevels, record.News.EventTimes)
	}
}

func TestProviderSetsObservedAtOnlyForExplicitBodyEventTime(t *testing.T) {
	fetcher := &fixtureFetcher{responses: map[string][]byte{
		mainURL:             []byte(listingFor("79953", "องครักษ์ น้ำท่วม")),
		articleURL("79953"): []byte(articleWithBody("องครักษ์ น้ำท่วม", "เหตุการณ์วันที่ 5 ตุลาคม 2569 เวลา 16:30 น. น้ำท่วมขัง")),
	}}
	provider := NewWithFetcher(fetcher)
	provider.Clock = func() time.Time { return time.Date(2026, 10, 5, 18, 0, 0, 0, bangkokLocation) }
	batch, err := provider.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(batch.Records) != 1 || batch.Records[0].ObservedAt == nil {
		t.Fatalf("record/event time = %#v", batch.Records)
	}
	if !batch.Records[0].ObservedAt.Equal(time.Date(2026, 10, 5, 16, 30, 0, 0, bangkokLocation).UTC()) {
		t.Fatalf("observedAt = %v", batch.Records[0].ObservedAt)
	}
	assertContains(t, batch.Records[0].News.EventTimes, "วันที่ 5 ตุลาคม 2569 เวลา 16:30 น.")
}

func TestProviderRetainsSimpleClockWithoutInventingObservedDate(t *testing.T) {
	fetcher := &fixtureFetcher{responses: map[string][]byte{
		mainURL:             []byte(listingFor("79953", "องครักษ์ น้ำท่วม")),
		articleURL("79953"): []byte(articleWithBody("องครักษ์ น้ำท่วม", "เหตุการณ์น้ำท่วม เวลา 16.00 น.")),
	}}
	provider := NewWithFetcher(fetcher)
	provider.Clock = func() time.Time { return time.Date(2026, 10, 5, 18, 0, 0, 0, bangkokLocation) }
	batch, err := provider.Fetch(context.Background())
	if err != nil || len(batch.Records) != 1 {
		t.Fatalf("clock-only record = %#v, err=%v", batch.Records, err)
	}
	if batch.Records[0].ObservedAt != nil {
		t.Fatalf("clock-only event inferred observedAt = %v", batch.Records[0].ObservedAt)
	}
	assertContains(t, batch.Records[0].News.EventTimes, "เวลา 16.00 น.")
}

func TestProviderUsesReFetchedArticleAndFailsWhenArticleBodyIsMissing(t *testing.T) {
	fetcher := &fixtureFetcher{responses: map[string][]byte{
		mainURL:             []byte(listingFor("79953", "องครักษ์ น้ำท่วม รุ่นเก่า")),
		articleURL("79953"): []byte(articleWithBody("องครักษ์ น้ำท่วม ฉบับแก้ไข", "องครักษ์ น้ำท่วมขังฉบับแก้ไข")),
	}}
	provider := NewWithFetcher(fetcher)
	provider.Clock = func() time.Time { return time.Date(2026, 10, 5, 18, 0, 0, 0, bangkokLocation) }
	batch, err := provider.Fetch(context.Background())
	if err != nil || len(batch.Records) != 1 || batch.Records[0].Headline != "องครักษ์ น้ำท่วม ฉบับแก้ไข" {
		t.Fatalf("revised article = %#v, err=%v", batch.Records, err)
	}

	missingBody := strings.Replace(articleWithBody("องครักษ์ น้ำท่วม", "องครักษ์"), `<div class="container"><div class="news-pra"><p>องครักษ์</p></div></div>`, "<div class=\"container\"></div>", 1)
	fetcher.responses[articleURL("79953")] = []byte(missingBody)
	if _, err := provider.Fetch(context.Background()); err == nil {
		t.Fatal("accepted article without news-pra body")
	}
}

func TestProviderHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := NewWithFetcher(contextFetcher{}).Fetch(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation error = %v", err)
	}
}

func TestProviderRejectsMalformedDriftUnsafeAndOversizedInputs(t *testing.T) {
	tests := []struct {
		name       string
		main       string
		articleURL string
		article    string
	}{
		{name: "malformed listing", main: "<html><body><div class=container>", articleURL: "", article: ""},
		{name: "listing drift", main: "<html><body><main>changed markup</main></body></html>", articleURL: "", article: ""},
		{name: "unmarked empty listing", main: `<html><body><div class="container mt-2"><h1>รวมข่าว</h1><div class="col-xl-8 col-lg-8 col-md-12 col-12"></div></div></body></html>`, articleURL: "", article: ""},
		{name: "unsafe article link", main: listingWithHref("https://evil.example/newsarticle/79953"), articleURL: "", article: ""},
		{name: "unsafe canonical", main: listingFor("79953", "น้ำท่วม องครักษ์"), articleURL: articleURL("79953"), article: strings.Replace(articleFixture(), `content="https://fm91bkk.com/newsarticle/79953"`, `content="https://evil.example/newsarticle/79953"`, 1)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fetcher := &fixtureFetcher{responses: map[string][]byte{mainURL: []byte(test.main)}}
			if test.articleURL != "" {
				fetcher.responses[test.articleURL] = []byte(test.article)
			}
			if _, err := NewWithFetcher(fetcher).Fetch(context.Background()); err == nil {
				t.Fatal("accepted malformed or unsafe input")
			}
		})
	}
}

func TestProviderFailsOnFetchErrorAndCandidateOverflow(t *testing.T) {
	fetcher := &fixtureFetcher{errs: map[string]error{mainURL: errors.New("upstream unavailable")}}
	if _, err := NewWithFetcher(fetcher).Fetch(context.Background()); err == nil {
		t.Fatal("fetch outage was hidden")
	}

	var listing strings.Builder
	listing.WriteString(`<html><body><div class="container mt-2"><h1>รวมข่าว</h1><div class="col-xl-8 col-lg-8 col-md-12 col-12">`)
	for i := 1; i <= 51; i++ {
		fmt.Fprintf(&listing, `<a href="/newsarticle/%d"><img alt="น้ำท่วม องครักษ์"></a>`, i)
	}
	listing.WriteString(`</div></div></body></html>`)
	if _, err := NewWithFetcher(&fixtureFetcher{responses: map[string][]byte{mainURL: []byte(listing.String())}}).Fetch(context.Background()); err == nil {
		t.Fatal("candidate overflow was silently discarded")
	}
}

func TestProviderAllowsValidEmptyListingAndPreservesRawBoundaries(t *testing.T) {
	fetcher := &fixtureFetcher{responses: map[string][]byte{
		mainURL: []byte(`<html><body><div class="container mt-2"><h1>รวมข่าว</h1><div class="col-xl-8 col-lg-8 col-md-12 col-12"><a href="/newsarticle/79954"><img alt="ข่าวทั่วไป"><b>ข่าวทั่วไป</b></a></div></div></body></html>`),
	}}
	batch, err := NewWithFetcher(fetcher).Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(batch.Records) != 0 || len(batch.Raw) != 1 || batch.Raw[0].URL != mainURL {
		t.Fatalf("empty batch = %#v", batch)
	}
}

func TestProviderRejectsRawTotalOverFortyMiB(t *testing.T) {
	large := strings.Repeat("x", (10<<20)-100)
	responses := map[string][]byte{mainURL: []byte(listingForMany(4))}
	for i := 1; i <= 4; i++ {
		responses[articleURL(fmt.Sprint(i))] = []byte(articleWithPadding(fmt.Sprint(i), large))
	}
	if _, err := NewWithFetcher(&fixtureFetcher{responses: responses}).Fetch(context.Background()); err == nil {
		t.Fatal("raw total over 40 MiB was accepted")
	}
}

func assertContains(t *testing.T, values []string, want string) {
	t.Helper()
	for _, value := range values {
		if value == want {
			return
		}
	}
	t.Fatalf("%q not found in %#v", want, values)
}

const bangkokLocationName = "Asia/Bangkok"

var bangkokLocation = mustLocation(bangkokLocationName)

func mustLocation(name string) *time.Location {
	location, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return location
}

func listingFixture() string {
	return `<html><body><nav><a href="/newsarticle/999">เกี่ยวข้องอื่น</a></nav><div class="container mt-2"><h1>รวมข่าว</h1><div class="col-xl-8 col-lg-8 col-md-12 col-12">
<article><a href="/newsarticle/79953"><img alt="องครักษ์ น้ำท่วมถนน รังสิต-นครนายก"><b>องครักษ์ น้ำท่วมถนน รังสิต-นครนายก</b><span>น้ำท่วมขัง รถเล็ก</span></a></article>
<article><a href="https://fm91bkk.com/newsarticle/79953"><img alt="องครักษ์ น้ำท่วมถนน รังสิต-นครนายก"><b>องครักษ์ น้ำท่วมถนน รังสิต-นครนายก</b></a></article>
</div></div></body></html>`
}

func listingFor(id, title string) string {
	return `<html><body><div class="container mt-2"><h1>รวมข่าว</h1><div class="col-xl-8 col-lg-8 col-md-12 col-12"><article><a href="/newsarticle/` + id + `"><img alt="` + title + `"><b>` + title + `</b></a></article></div></div></body></html>`
}

func listingForMany(count int) string {
	var listing strings.Builder
	listing.WriteString(`<html><body><div class="container mt-2"><h1>รวมข่าว</h1><div class="col-xl-8 col-lg-8 col-md-12 col-12">`)
	for i := 1; i <= count; i++ {
		fmt.Fprintf(&listing, `<article><a href="/newsarticle/%d"><img alt="น้ำท่วม องครักษ์"><b>น้ำท่วม องครักษ์</b></a></article>`, i)
	}
	listing.WriteString(`</div></div></body></html>`)
	return listing.String()
}

func listingWithHref(href string) string {
	return `<html><body><div class="container mt-2"><h1>รวมข่าว</h1><div class="col-xl-8 col-lg-8 col-md-12 col-12"><article><a href="` + href + `"><img alt="น้ำท่วม องครักษ์"><b>น้ำท่วม องครักษ์</b></a></article></div></div></body></html>`
}

func articleURL(id string) string { return "https://fm91bkk.com/newsarticle/" + id }

func articleFixture() string {
	return `<html><head>
<meta property="og:url" content="https://fm91bkk.com/newsarticle/79953">
<meta property="og:title" content="องครักษ์ น้ำท่วมถนน รังสิต-นครนายก">
<meta property="og:description" content="ไม่ใช่บทความหลัก">
<meta property="og:image" content="https://fm91bkk.com/upload/79953-main.jpg">
</head><body>
<h1 hidden>FM91</h1><div class="container mt-3"><a href="/newscategory/2/regional">ภูมิภาค</a><h1>องครักษ์ น้ำท่วมถนน รังสิต-นครนายก</h1><p class="mt-3">วันที่ 5 ตุลาคม 2569 เวลา 17:02 น.</p></div>
<div class="container"><div class="row"><div class="col-xl-8 col-lg-8 col-md-12 col-12"><div class="news-pra">
<div class="product-gallery"><img src="upload/tb_news/webp/2026-10/05/79953/gallery.webp" alt="gallery"><figcaption>caption evil</figcaption></div>
<div class="more-img-wrapper">
<p>เกิดน้ำท่วมขังสูง 30 เซนติเมตร บริเวณคลอง 14 บน ทล.305 รถเล็กผ่านลำบาก</p>
</div>
<script>document.body.innerText = 'evil';</script><style>.evil{}</style>
</div></div><aside>องครักษ์ น้ำท่วม รถเล็ก</aside></div></div>
</body></html>`
}

func articleWithBody(headline, body string) string {
	return `<html><head><meta property="og:url" content="https://fm91bkk.com/newsarticle/79953"></head><body><div class="container mt-3"><a href="/newscategory/2/regional">ภูมิภาค</a><h1>` + headline + `</h1><p class="mt-3">วันที่ 5 ตุลาคม 2569</p></div><div class="container"><div class="news-pra"><p>` + body + `</p></div></div></body></html>`
}

func articleWithPadding(id, padding string) string {
	return `<html><head><meta property="og:url" content="https://fm91bkk.com/newsarticle/` + id + `"></head><body><div class="container mt-3"><a href="/newscategory/2/regional">ภูมิภาค</a><h1>องครักษ์ น้ำท่วม</h1><p class="mt-3">วันที่ 5 ตุลาคม 2569</p></div><div class="container"><div class="news-pra"><p>องครักษ์ น้ำท่วม</p><script>` + padding + `</script></div></div></body></html>`
}

func articleWithSidebar(body, sidebar string) string {
	return `<html><head><meta property="og:url" content="https://fm91bkk.com/newsarticle/79953"></head><body><div class="container mt-3"><a href="/newscategory/2/regional">ภูมิภาค</a><h1>น้ำท่วมในพื้นที่อื่น</h1><p class="mt-3">วันที่ 5 ตุลาคม 2569 เวลา 17:02 น.</p></div><div class="container"><div class="news-pra"><p>` + body + `</p></div><aside>` + sidebar + `</aside></div></body></html>`
}
