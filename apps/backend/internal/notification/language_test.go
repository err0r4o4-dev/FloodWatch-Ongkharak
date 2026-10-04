package notification

import (
	"strings"
	"testing"

	"github.com/floodwatch-ongkharak/backend/internal/flood"
)

func TestLocalizedDeliveryUsesDeviceLanguageWithoutTranslatingOriginal(t *testing.T) {
	job := DeliveryJob{Locale: "en", Severity: flood.Severity("WARNING"), SourceType: flood.SourceType("OFFICIAL"), Payload: Payload{Title: "ประกาศต้นฉบับ", Message: "รายละเอียดต้นฉบับ", URL: "/announcements"}}
	payload := localizedDeliveryPayload(job)
	if payload.Locale != "en" || payload.Title != "Flood Watch · Warning" || !strings.Contains(payload.Message, "Official source") || payload.URL != job.Payload.URL {
		t.Fatalf("unexpected English delivery: %#v", payload)
	}
	if job.Payload.Title != "ประกาศต้นฉบับ" || job.Payload.Message != "รายละเอียดต้นฉบับ" {
		t.Fatal("localization changed the original announcement")
	}
	job.Locale = "th"
	if payload = localizedDeliveryPayload(job); payload.Locale != "th" || !strings.Contains(payload.Title, "เตือนภัย") {
		t.Fatalf("unexpected Thai delivery: %#v", payload)
	}
	job.Locale = "unsupported"
	job.Severity = "UNKNOWN"
	if payload = localizedDeliveryPayload(job); payload.Locale != "th" || strings.Contains(payload.Title, "ปลอดภัย") {
		t.Fatalf("invalid locale or unknown status handled unsafely: %#v", payload)
	}
}

func TestPushSubscriptionRejectsUnsupportedLocale(t *testing.T) {
	sub := Subscription{UserID: "user", Endpoint: "https://push.example.test/subscription", P256dh: strings.Repeat("A", 87), Auth: strings.Repeat("A", 22)}
	sub.Locale = "unsupported"
	if validPushSubscription(sub) {
		t.Fatal("unsupported locale was accepted")
	}
	sub.Locale = "en"
	if !validPushSubscription(sub) {
		t.Fatal("English locale rejected")
	}
	sub.Locale = ""
	if !validPushSubscription(sub) {
		t.Fatal("legacy locale rejected")
	}
}
