package hii

import (
	"context"
	"encoding/binary"
	"errors"
	"math"
	"os"
	"strings"
	"testing"
)

const (
	testWidth       = 543
	testHeight      = 699
	testTargetRow   = 313
	testTargetCol   = 203
	testTargetValue = float32(12.5)
)

type fakeFetcher struct {
	bodies map[string][]byte
	urls   []string
}

func (fetcher *fakeFetcher) Fetch(_ context.Context, url string) ([]byte, error) {
	fetcher.urls = append(fetcher.urls, url)
	body, ok := fetcher.bodies[url]
	if !ok {
		return nil, errors.New("unexpected URL")
	}
	return append([]byte(nil), body...), nil
}

func TestFetchUsesExactMetadataAndTIFFURLsAndPreservesRawBodies(t *testing.T) {
	metadataURL := "https://live1.hii.or.th/product/latest/wrfroms/tiff/description_wrfroms.json"
	urls := []string{
		"https://live1.hii.or.th/product/latest/wrfroms/tiff/fcst_rain24hr_d03_day1.tif",
		"https://live1.hii.or.th/product/latest/wrfroms/tiff/fcst_rain24hr_d03_day2.tif",
		"https://live1.hii.or.th/product/latest/wrfroms/tiff/fcst_rain24hr_d03_day3.tif",
	}
	metadata := []byte(`[[{"date":"2026-10-04","fcst_time":"07:00","filename":"` + urls[0] + `"}],[{"date":"2026-10-05","fcst_time":"07:00","filename":"` + urls[1] + `"}],[{"date":"2026-10-06","fcst_time":"07:00","filename":"` + urls[2] + `"}]]`)
	fetcher := &fakeFetcher{bodies: map[string][]byte{
		metadataURL: metadata,
		urls[0]:     syntheticTIFF(testTargetValue, 5),
		urls[1]:     syntheticTIFF(testTargetValue, 5),
		urls[2]:     syntheticTIFF(testTargetValue, 5),
	}}
	provider, err := NewWithFetcher(fetcher)
	if err != nil {
		t.Fatal(err)
	}
	batch, err := provider.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	wantURLs := append([]string{metadataURL}, urls...)
	if len(fetcher.urls) != len(wantURLs) {
		t.Fatalf("fetch URLs = %#v, want %#v", fetcher.urls, wantURLs)
	}
	for index, want := range wantURLs {
		if fetcher.urls[index] != want {
			t.Fatalf("fetch URL %d = %q, want %q", index, fetcher.urls[index], want)
		}
	}
	if len(batch.Raw) != 4 || string(batch.Raw[0].Body) != string(metadata) || batch.Raw[0].URL != metadataURL {
		t.Fatalf("raw metadata = %+v", batch.Raw)
	}
	if len(batch.Records) != 3 {
		t.Fatalf("records = %d, want 3", len(batch.Records))
	}
	for index, record := range batch.Records {
		if record.Kind != "FORECAST" || record.Provenance != "MODEL" || record.Model != "HII_WRF_ROMS" || record.Metric != "precipitation" {
			t.Fatalf("record %d identity = %+v", index, record)
		}
		if record.Value != nil || !strings.Contains(record.Reason, "unverified") {
			t.Fatalf("record %d should remain unknown until semantics are verified: %+v", index, record)
		}
		if record.ValidFrom != nil || record.ValidTo != nil || record.IssuedAt != nil {
			t.Fatalf("record %d must not invent an interval or issue time: %+v", index, record)
		}
		if record.Latitude == nil || record.Longitude == nil || *record.Latitude == 14.1 || *record.Longitude == 101 {
			t.Fatalf("record %d must retain selected grid coordinates: %+v", index, record)
		}
	}
}

func TestDecodeTIFFSupportsObservedFloat32LZWGridAndNoData(t *testing.T) {
	image, err := decodeTIFF(syntheticTIFF(testTargetValue, 5))
	if err != nil {
		t.Fatal(err)
	}
	if image.width != testWidth || image.height != testHeight || image.bitsPerSample != 32 || image.compression != 5 {
		t.Fatalf("image header = %+v", image)
	}
	value, ok := image.valueAt(testTargetRow, testTargetCol)
	if !ok || value != float64(testTargetValue) {
		t.Fatalf("selected value = %v, present=%v", value, ok)
	}
	nodataImage, err := decodeTIFF(syntheticTIFF(-9999, 5))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := nodataImage.valueAt(testTargetRow, testTargetCol); ok {
		t.Fatal("NoData cell accepted as a numeric value")
	}
	if nodataImage.noData == nil || *nodataImage.noData != -9999 {
		t.Fatalf("NoData = %v", nodataImage.noData)
	}
}

func TestDecodeTIFFRejectsUnsupportedCompressionAndUnsafeStrip(t *testing.T) {
	if _, err := decodeTIFF(syntheticTIFF(testTargetValue, 1)); err == nil {
		t.Fatal("unsupported compression accepted")
	}
	unsafe := syntheticTIFF(testTargetValue, 5)
	setFirstStripOffset(unsafe, uint32(len(unsafe)+1))
	if _, err := decodeTIFF(unsafe); err == nil {
		t.Fatal("out-of-bounds strip accepted")
	}
}

func TestLiveHII(t *testing.T) {
	if os.Getenv("TEST_LIVE_HII") != "true" {
		t.Skip("set TEST_LIVE_HII=true to call the official HII provider")
	}
	provider, err := New()
	if err != nil {
		t.Fatal(err)
	}
	batch, err := provider.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(batch.Raw) != 4 || len(batch.Records) != 3 {
		t.Fatalf("live HII batch = raw %d records %d", len(batch.Raw), len(batch.Records))
	}
}

func syntheticTIFF(target float32, compression uint16) []byte {
	strips := make([][]byte, 233)
	for strip := range strips {
		raw := make([]byte, 3*testWidth*4)
		rowStart := strip * 3
		if testTargetRow >= rowStart && testTargetRow < rowStart+3 {
			offset := (testTargetRow-rowStart)*testWidth*4 + testTargetCol*4
			binary.LittleEndian.PutUint32(raw[offset:], math.Float32bits(target))
		}
		strips[strip] = literalLZW(raw)
	}

	const tagCount = 14
	ifdEnd := 8 + 2 + tagCount*12 + 4
	stripOffsetsOffset := ifdEnd
	stripCountsOffset := stripOffsetsOffset + len(strips)*4
	scaleOffset := stripCountsOffset + len(strips)*4
	tiepointOffset := scaleOffset + 24
	noDataOffset := tiepointOffset + 48
	dataOffset := noDataOffset + 6
	stripOffsets := make([]uint32, len(strips))
	stripCounts := make([]uint32, len(strips))
	data := make([]byte, dataOffset)
	for index, strip := range strips {
		stripOffsets[index] = uint32(len(data))
		stripCounts[index] = uint32(len(strip))
		data = append(data, strip...)
	}
	for index, offset := range stripOffsets {
		binary.LittleEndian.PutUint32(data[stripOffsetsOffset+index*4:], offset)
		binary.LittleEndian.PutUint32(data[stripCountsOffset+index*4:], stripCounts[index])
	}
	for index, value := range []float64{0.0271216107477, 0.0271216107468, 0} {
		binary.LittleEndian.PutUint64(data[scaleOffset+index*8:], math.Float64bits(value))
	}
	for index, value := range []float64{0, 0, 0, 95.483639207, 22.599005195, 0} {
		binary.LittleEndian.PutUint64(data[tiepointOffset+index*8:], math.Float64bits(value))
	}
	copy(data[noDataOffset:], []byte("-9999\x00"))
	entries := []struct {
		tag   uint16
		type_ uint16
		count uint32
		value []byte
	}{
		{256, 3, 1, shortValue(testWidth)},
		{257, 3, 1, shortValue(testHeight)},
		{258, 3, 1, shortValue(32)},
		{259, 3, 1, shortValue(int(compression))},
		{262, 3, 1, shortValue(1)},
		{273, 4, uint32(len(strips)), uint32Array(stripOffsetsOffset, len(strips))},
		{277, 3, 1, shortValue(1)},
		{278, 4, 1, uint32Value(3)},
		{279, 4, uint32(len(strips)), uint32Array(stripCountsOffset, len(strips))},
		{284, 3, 1, shortValue(1)},
		{33550, 12, 3, uint32Value(scaleOffset)},
		{339, 3, 1, shortValue(3)},
		{33922, 12, 6, uint32Value(tiepointOffset)},
		{42113, 2, 6, uint32Value(noDataOffset)},
	}
	binary.LittleEndian.PutUint16(data[0:], 0x4949)
	binary.LittleEndian.PutUint16(data[2:], 42)
	binary.LittleEndian.PutUint32(data[4:], 8)
	binary.LittleEndian.PutUint16(data[8:], tagCount)
	for index, entry := range entries {
		offset := 10 + index*12
		binary.LittleEndian.PutUint16(data[offset:], entry.tag)
		binary.LittleEndian.PutUint16(data[offset+2:], entry.type_)
		binary.LittleEndian.PutUint32(data[offset+4:], entry.count)
		if len(entry.value) <= 4 {
			copy(data[offset+8:offset+12], entry.value)
		} else {
			binary.LittleEndian.PutUint32(data[offset+8:], binary.LittleEndian.Uint32(entry.value))
		}
	}
	return data
}

func literalLZW(raw []byte) []byte {
	codes := make([]uint16, 0, len(raw)/2)
	codes = append(codes, 256)
	dictionary := make(map[string]uint16, 4096)
	reset := func() {
		clear(dictionary)
		for value := 0; value < 256; value++ {
			dictionary[string([]byte{byte(value)})] = uint16(value)
		}
	}
	reset()
	codeSize, nextCode := 9, 258
	w := []byte(nil)
	for _, value := range raw {
		candidate := append(append([]byte(nil), w...), value)
		if _, ok := dictionary[string(candidate)]; ok {
			w = candidate
			continue
		}
		codes = append(codes, dictionary[string(w)])
		if nextCode < 4096 {
			dictionary[string(candidate)] = uint16(nextCode)
			nextCode++
			if nextCode == (1<<codeSize)-1 && codeSize < 12 {
				codeSize++
			}
		} else {
			codes = append(codes, 256)
			reset()
			codeSize, nextCode = 9, 258
		}
		w = []byte{value}
	}
	if len(w) > 0 {
		codes = append(codes, dictionary[string(w)])
	}
	codes = append(codes, 257)
	encoded := make([]byte, (len(codes)*12+7)/8)
	bit, codeIndex, codeSize, nextCode := 0, 0, 9, 258
	hasPrevious := false
	for codeIndex < len(codes) {
		code := codes[codeIndex]
		for shift := codeSize - 1; shift >= 0; shift-- {
			if code&(1<<shift) != 0 {
				encoded[bit/8] |= 1 << (7 - bit%8)
			}
			bit++
		}
		codeIndex++
		if code == 256 {
			codeSize, nextCode = 9, 258
			hasPrevious = false
			continue
		}
		if code == 257 {
			break
		}
		if hasPrevious && nextCode < 4096 {
			nextCode++
			if nextCode == (1<<codeSize)-1 && codeSize < 12 {
				codeSize++
			}
		}
		hasPrevious = true
	}
	return encoded[:(bit+7)/8]
}

func shortValue(value int) []byte {
	result := make([]byte, 4)
	binary.LittleEndian.PutUint16(result, uint16(value))
	return result
}

func uint32Value(value int) []byte {
	result := make([]byte, 4)
	binary.LittleEndian.PutUint32(result, uint32(value))
	return result
}

func uint32Array(offset, count int) []byte {
	result := make([]byte, 4)
	binary.LittleEndian.PutUint32(result, uint32(offset))
	_ = count
	return result
}

func setFirstStripOffset(body []byte, value uint32) {
	ifd := binary.LittleEndian.Uint32(body[4:])
	count := binary.LittleEndian.Uint16(body[ifd:])
	for index := uint16(0); index < count; index++ {
		offset := int(ifd) + 2 + int(index)*12
		if binary.LittleEndian.Uint16(body[offset:]) == 273 {
			arrayOffset := binary.LittleEndian.Uint32(body[offset+8:])
			binary.LittleEndian.PutUint32(body[arrayOffset:], value)
			return
		}
	}
}
