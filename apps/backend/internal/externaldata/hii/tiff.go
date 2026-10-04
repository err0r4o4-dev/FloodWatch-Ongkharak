package hii

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

const maxTIFFBytes = 5 << 20

const (
	hiiWidth  = 543
	hiiHeight = 699
)

type tiffImage struct {
	width         int
	height        int
	bitsPerSample uint16
	compression   uint16
	rowsPerStrip  uint32
	values        []float64
	noData        *float64
	scaleX        float64
	scaleY        float64
	tieLon        float64
	tieLat        float64
}

func decodeTIFF(body []byte) (tiffImage, error) {
	if len(body) == 0 || len(body) > maxTIFFBytes || len(body) < 8 {
		return tiffImage{}, errors.New("TIFF size is unsafe")
	}
	order, err := tiffOrder(body[:2])
	if err != nil || order.Uint16(body[2:4]) != 42 {
		return tiffImage{}, errors.New("unsupported TIFF header")
	}
	ifdOffset := uint64(order.Uint32(body[4:8]))
	if ifdOffset > uint64(len(body)-2) {
		return tiffImage{}, errors.New("TIFF IFD is out of bounds")
	}
	count := int(order.Uint16(body[ifdOffset:]))
	if count == 0 || count > 64 || uint64(2+count*12+4) > uint64(len(body))-ifdOffset {
		return tiffImage{}, errors.New("TIFF IFD is invalid")
	}
	fields := map[uint16]tiffField{}
	for index := 0; index < count; index++ {
		offset := int(ifdOffset) + 2 + index*12
		field, fieldErr := readField(body, order, offset)
		if fieldErr != nil {
			return tiffImage{}, fieldErr
		}
		fields[field.tag] = field
	}
	width, err := fieldScalar(fields, body, order, 256)
	if err != nil || width != hiiWidth {
		return tiffImage{}, errors.New("unsupported TIFF width")
	}
	height, err := fieldScalar(fields, body, order, 257)
	if err != nil || height != hiiHeight {
		return tiffImage{}, errors.New("unsupported TIFF height")
	}
	if uint64(width)*uint64(height) > uint64(len(body))*16 || uint64(width)*uint64(height) > 10_000_000 {
		return tiffImage{}, errors.New("TIFF raster dimensions are unsafe")
	}
	bits, err := fieldScalar(fields, body, order, 258)
	if err != nil || (bits != 32 && bits != 64) {
		return tiffImage{}, errors.New("unsupported TIFF sample size")
	}
	compression, err := fieldScalar(fields, body, order, 259)
	if err != nil || (compression != 1 && compression != 5) {
		return tiffImage{}, fmt.Errorf("unsupported TIFF compression %d", compression)
	}
	samples, err := fieldScalar(fields, body, order, 277)
	if err != nil || samples != 1 {
		return tiffImage{}, errors.New("unsupported TIFF samples per pixel")
	}
	planar, err := fieldScalar(fields, body, order, 284)
	if err != nil || planar != 1 {
		return tiffImage{}, errors.New("unsupported TIFF planar configuration")
	}
	sampleFormat, err := fieldScalar(fields, body, order, 339)
	if err != nil || sampleFormat != 3 {
		return tiffImage{}, errors.New("unsupported TIFF sample format")
	}
	rowsPerStrip, err := fieldScalar(fields, body, order, 278)
	if err != nil || rowsPerStrip == 0 {
		return tiffImage{}, errors.New("invalid TIFF rows per strip")
	}
	offsets, err := fieldIntegers(fields, body, order, 273)
	if err != nil {
		return tiffImage{}, errors.New("invalid TIFF strip offsets")
	}
	counts, err := fieldIntegers(fields, body, order, 279)
	if err != nil || len(offsets) != len(counts) || len(offsets) == 0 {
		return tiffImage{}, errors.New("invalid TIFF strip byte counts")
	}
	expectedStrips := (height + int(rowsPerStrip) - 1) / int(rowsPerStrip)
	if len(offsets) != expectedStrips {
		return tiffImage{}, errors.New("unsupported TIFF strip layout")
	}
	if _, tiled := fields[324]; tiled {
		return tiffImage{}, errors.New("tiled TIFF is unsupported")
	}
	image := tiffImage{width: width, height: height, bitsPerSample: uint16(bits), compression: uint16(compression), rowsPerStrip: uint32(rowsPerStrip), values: make([]float64, width*height)}
	if err := decodeStrips(&image, body, order, offsets, counts); err != nil {
		return tiffImage{}, err
	}
	if err := image.readGeoTIFF(fields, body, order); err != nil {
		return tiffImage{}, err
	}
	return image, nil
}

type tiffField struct {
	tag   uint16
	type_ uint16
	count uint32
	data  []byte
}

func tiffOrder(header []byte) (binary.ByteOrder, error) {
	switch string(header) {
	case "II":
		return binary.LittleEndian, nil
	case "MM":
		return binary.BigEndian, nil
	default:
		return nil, errors.New("unsupported TIFF byte order")
	}
}

func readField(body []byte, order binary.ByteOrder, offset int) (tiffField, error) {
	if offset < 0 || offset+12 > len(body) {
		return tiffField{}, errors.New("TIFF field is out of bounds")
	}
	field := tiffField{tag: order.Uint16(body[offset:]), type_: order.Uint16(body[offset+2:]), count: order.Uint32(body[offset+4:])}
	unit, ok := typeSize(field.type_)
	if !ok || field.count == 0 || uint64(unit)*uint64(field.count) > uint64(maxTIFFBytes) {
		return tiffField{}, errors.New("TIFF field size is invalid")
	}
	total := uint64(unit) * uint64(field.count)
	valueOffset := offset + 8
	if total > 4 {
		point := uint64(order.Uint32(body[offset+8:]))
		if point > uint64(len(body)) || total > uint64(len(body))-point {
			return tiffField{}, errors.New("TIFF field value is out of bounds")
		}
		valueOffset = int(point)
	}
	if uint64(valueOffset) > uint64(len(body)) || total > uint64(len(body))-uint64(valueOffset) {
		return tiffField{}, errors.New("TIFF inline value is out of bounds")
	}
	field.data = append([]byte(nil), body[valueOffset:valueOffset+int(total)]...)
	return field, nil
}

func typeSize(typeID uint16) (uint64, bool) {
	switch typeID {
	case 1, 2, 6, 7:
		return 1, true
	case 3, 8:
		return 2, true
	case 4, 9, 11:
		return 4, true
	case 5, 10, 12:
		return 8, true
	default:
		return 0, false
	}
}

func fieldScalar(fields map[uint16]tiffField, body []byte, order binary.ByteOrder, tag uint16) (int, error) {
	field, ok := fields[tag]
	if !ok || field.count != 1 {
		return 0, errors.New("required TIFF scalar missing")
	}
	switch field.type_ {
	case 3:
		return int(order.Uint16(field.data)), nil
	case 4:
		value := order.Uint32(field.data)
		if uint64(value) > uint64(^uint(0)>>1) {
			return 0, errors.New("TIFF scalar is too large")
		}
		return int(value), nil
	default:
		_ = body
		return 0, errors.New("unsupported TIFF scalar type")
	}
}

func fieldIntegers(fields map[uint16]tiffField, body []byte, order binary.ByteOrder, tag uint16) ([]uint32, error) {
	field, ok := fields[tag]
	if !ok || (field.type_ != 3 && field.type_ != 4) || field.count > 10000 {
		return nil, errors.New("required TIFF integer array missing")
	}
	values := make([]uint32, field.count)
	for index := range values {
		if field.type_ == 3 {
			values[index] = uint32(order.Uint16(field.data[index*2:]))
		} else {
			values[index] = order.Uint32(field.data[index*4:])
		}
	}
	_ = body
	return values, nil
}

func decodeStrips(image *tiffImage, body []byte, order binary.ByteOrder, offsets, counts []uint32) error {
	bytesPerSample := int(image.bitsPerSample / 8)
	for index, offset := range offsets {
		if uint64(offset) > uint64(len(body)) || uint64(counts[index]) > uint64(len(body))-uint64(offset) {
			return errors.New("TIFF strip is out of bounds")
		}
		rows := int(image.rowsPerStrip)
		if start := index * rows; start+rows > image.height {
			rows = image.height - start
		}
		expected := rows * image.width * bytesPerSample
		compressed := body[offset : uint64(offset)+uint64(counts[index])]
		decoded, err := decodeStrip(compressed, image.compression, expected)
		if err != nil {
			return err
		}
		if len(decoded) != expected {
			return errors.New("TIFF strip has an unexpected decoded size")
		}
		for sample := 0; sample < rows*image.width; sample++ {
			position := (index*int(image.rowsPerStrip)*image.width + sample) * bytesPerSample
			if image.bitsPerSample == 32 {
				image.values[position/bytesPerSample] = float64(math.Float32frombits(order.Uint32(decoded[sample*4:])))
			} else {
				image.values[position/bytesPerSample] = math.Float64frombits(order.Uint64(decoded[sample*8:]))
			}
			value := image.values[position/bytesPerSample]
			if math.IsNaN(value) || math.IsInf(value, 0) {
				return errors.New("TIFF contains a non-finite value")
			}
		}
	}
	return nil
}

func decodeStrip(body []byte, compression uint16, expected int) ([]byte, error) {
	if compression == 1 {
		if len(body) > expected {
			return nil, errors.New("uncompressed TIFF strip is too large")
		}
		return append([]byte(nil), body...), nil
	}
	return decodeLZW(body, expected)
}

func decodeLZW(body []byte, expected int) ([]byte, error) {
	const clearCode = 256
	const endCode = 257
	dictionary := make([][]byte, 4096)
	output := make([]byte, 0, expected)
	codeSize, nextCode := 9, 258
	previous := []byte(nil)
	reader := lzwBitReader{body: body}
	for {
		code, ok := reader.read(codeSize)
		if !ok {
			return nil, errors.New("truncated TIFF LZW stream")
		}
		switch code {
		case clearCode:
			dictionary = make([][]byte, 4096)
			codeSize, nextCode = 9, 258
			previous = nil
			continue
		case endCode:
			if len(output) != expected {
				return nil, errors.New("TIFF LZW output length mismatch")
			}
			return output, nil
		}
		var entry []byte
		if code < 256 {
			entry = []byte{byte(code)}
		} else if int(code) < nextCode && dictionary[code] != nil {
			entry = dictionary[code]
		} else if int(code) == nextCode && previous != nil {
			entry = append(append([]byte(nil), previous...), previous[0])
		} else {
			return nil, errors.New("invalid TIFF LZW code")
		}
		if len(output)+len(entry) > expected {
			return nil, errors.New("TIFF LZW output exceeds expected size")
		}
		output = append(output, entry...)
		if previous != nil {
			dictionary[nextCode] = append(append([]byte(nil), previous...), entry[0])
			nextCode++
			if nextCode == (1<<codeSize)-1 && codeSize < 12 {
				codeSize++
			}
		}
		previous = entry
	}
}

type lzwBitReader struct {
	body []byte
	bit  int
}

func (reader *lzwBitReader) read(width int) (uint16, bool) {
	if reader.bit+width > len(reader.body)*8 {
		return 0, false
	}
	var value uint16
	for index := 0; index < width; index++ {
		value <<= 1
		if reader.body[reader.bit/8]&(1<<uint(7-reader.bit%8)) != 0 {
			value |= 1
		}
		reader.bit++
	}
	return value, true
}

func (image *tiffImage) readGeoTIFF(fields map[uint16]tiffField, body []byte, order binary.ByteOrder) error {
	scale, err := fieldDoubles(fields, body, order, 33550, 3)
	if err != nil || scale[0] <= 0 || scale[1] <= 0 {
		return errors.New("invalid TIFF pixel scale")
	}
	tiepoint, err := fieldDoubles(fields, body, order, 33922, 6)
	if err != nil || tiepoint[0] != 0 || tiepoint[1] != 0 || tiepoint[2] != 0 {
		return errors.New("invalid TIFF tiepoint")
	}
	image.scaleX, image.scaleY = scale[0], scale[1]
	image.tieLon, image.tieLat = tiepoint[3], tiepoint[4]
	if field, ok := fields[42113]; ok && field.type_ == 2 {
		text := strings.TrimRight(string(field.data), "\x00")
		value, parseErr := strconv.ParseFloat(strings.TrimSpace(text), 64)
		if parseErr != nil || math.IsNaN(value) || math.IsInf(value, 0) {
			return errors.New("invalid TIFF NoData value")
		}
		image.noData = &value
	}
	return nil
}

func fieldDoubles(fields map[uint16]tiffField, body []byte, order binary.ByteOrder, tag uint16, count uint32) ([]float64, error) {
	field, ok := fields[tag]
	if !ok || field.type_ != 12 || field.count != count {
		return nil, errors.New("required TIFF georeference missing")
	}
	values := make([]float64, count)
	for index := range values {
		bits := order.Uint64(field.data[index*8:])
		var raw [8]byte
		order.PutUint64(raw[:], bits)
		values[index] = math.Float64frombits(order.Uint64(field.data[index*8:]))
		if math.IsNaN(values[index]) || math.IsInf(values[index], 0) {
			return nil, errors.New("non-finite TIFF georeference")
		}
	}
	_ = body
	return values, nil
}

func (image tiffImage) valueAt(row, col int) (float64, bool) {
	if row < 0 || row >= image.height || col < 0 || col >= image.width {
		return 0, false
	}
	value := image.values[row*image.width+col]
	if math.IsNaN(value) || image.noData != nil && value == *image.noData {
		return 0, false
	}
	return value, true
}

func (image tiffImage) coordinate(row, col int) (float64, float64) {
	return image.tieLat - float64(row)*image.scaleY, image.tieLon + float64(col)*image.scaleX
}

func (image tiffImage) referenceCell(latitude, longitude float64) (int, int, error) {
	row := int(math.Round((image.tieLat - latitude) / image.scaleY))
	col := int(math.Round((longitude - image.tieLon) / image.scaleX))
	if row < 0 || row >= image.height || col < 0 || col >= image.width {
		return 0, 0, errors.New("HII reference point is outside the TIFF grid")
	}
	actualLat, actualLon := image.coordinate(row, col)
	if math.Abs(actualLat-latitude) > image.scaleY || math.Abs(actualLon-longitude) > image.scaleX {
		return 0, 0, errors.New("HII reference point is not near a TIFF grid cell")
	}
	return row, col, nil
}
