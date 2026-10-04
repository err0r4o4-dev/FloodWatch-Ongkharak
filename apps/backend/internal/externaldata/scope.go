package externaldata

import "strings"

func IsOngkharakStation(station *Station) bool {
	return station != nil && strings.TrimSpace(station.District) == "องครักษ์" && strings.TrimSpace(station.Province) == "นครนายก"
}
