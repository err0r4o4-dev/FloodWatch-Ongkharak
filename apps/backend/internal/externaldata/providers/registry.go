package providers

import (
	"errors"

	"github.com/floodwatch-ongkharak/backend/internal/externaldata"
	"github.com/floodwatch-ongkharak/backend/internal/externaldata/fm91"
	"github.com/floodwatch-ongkharak/backend/internal/externaldata/hii"
	"github.com/floodwatch-ongkharak/backend/internal/externaldata/openmeteo"
	"github.com/floodwatch-ongkharak/backend/internal/externaldata/thaiwater"
	"github.com/floodwatch-ongkharak/backend/internal/externaldata/tmd"
)

func New(codes []string) (map[string]externaldata.Provider, error) {
	registered := make(map[string]externaldata.Provider, len(codes))
	for _, code := range codes {
		if _, found := registered[code]; found {
			return nil, errors.New("duplicate external provider")
		}
		var provider externaldata.Provider
		var err error
		switch code {
		case "THAIWATER_WATER":
			provider, err = thaiwater.New("WATER")
		case "THAIWATER_RAIN":
			provider, err = thaiwater.New("RAIN")
		case "THAIWATER_GATE":
			provider, err = thaiwater.New("GATE")
		case "OPENMETEO_ECMWF":
			provider, err = openmeteo.New("ecmwf_ifs025")
		case "OPENMETEO_GFS":
			provider, err = openmeteo.New("gfs_seamless")
		case "OPENMETEO_ICON":
			provider, err = openmeteo.New("icon_seamless")
		case "OPENMETEO_ENSEMBLE":
			provider, err = openmeteo.New("ensemble")
		case "GLOFAS":
			provider, err = openmeteo.New("glofas")
		case "HII_WRF":
			provider, err = hii.New()
		case "TMD_CAP":
			provider, err = tmd.New()
		case "FM91_NEWS":
			provider, err = fm91.New()
		default:
			return nil, errors.New("unregistered external provider")
		}
		if err != nil {
			return nil, err
		}
		registered[code] = provider
	}
	return registered, nil
}
