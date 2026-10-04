package notification

import "github.com/floodwatch-ongkharak/backend/internal/flood"

func validNotificationLocale(locale string) bool {
	return locale == "th" || locale == "en"
}

func localizedDeliveryPayload(job DeliveryJob) Payload {
	locale := job.Locale
	if locale != "en" {
		locale = "th"
	}
	severity := map[flood.Severity][2]string{
		flood.SeverityInfo:     {"ข้อมูล", "Information"},
		flood.SeverityWatch:    {"เฝ้าระวัง", "Watch"},
		flood.SeverityWarning:  {"เตือนภัย", "Warning"},
		flood.SeverityCritical: {"วิกฤต", "Critical"},
	}
	sources := map[flood.SourceType][2]string{
		flood.SourceOfficial:  {"แหล่งทางการ", "Official source"},
		flood.SourceCommunity: {"รายงานชุมชน", "Community report"},
		flood.SourceAI:        {"หลักฐาน AI", "AI evidence"},
	}
	index := 0
	if locale == "en" {
		index = 1
	}
	status, ok := severity[job.Severity]
	if !ok {
		status = [2]string{"ไม่ทราบสถานะ", "Unknown status"}
	}
	source, ok := sources[job.SourceType]
	if !ok {
		source = [2]string{"ไม่ทราบแหล่งข้อมูล", "Source unknown"}
	}
	message := source[index] + " · มีประกาศใหม่ เปิดดูรายละเอียดต้นฉบับ"
	if locale == "en" {
		message = source[index] + " · New announcement. Open to read the original details."
	}
	return Payload{Locale: locale, Title: "Flood Watch · " + status[index], Message: message, URL: job.Payload.URL}
}
