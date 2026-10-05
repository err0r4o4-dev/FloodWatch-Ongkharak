package roadnetworkimport

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/url"
	"os"
	"strings"
)

type rawSnapshot struct {
	Elements []json.RawMessage `json:"elements"`
}

type rawElement struct {
	Type      string            `json:"type"`
	ID        int64             `json:"id"`
	Lat       *float64          `json:"lat"`
	Lon       *float64          `json:"lon"`
	Nodes     []int64           `json:"nodes"`
	Tags      map[string]string `json:"tags"`
	Timestamp string            `json:"timestamp"`
}

func ReadSnapshotFile(path string) (LoadedSnapshot, error) {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return LoadedSnapshot{}, fmt.Errorf("stat OSM snapshot: %w", err)
	}
	if fileInfo.IsDir() {
		return LoadedSnapshot{}, errors.New("OSM snapshot input is a directory")
	}
	if fileInfo.Size() > maxSnapshotBytes {
		return LoadedSnapshot{}, fmt.Errorf("OSM snapshot exceeds %d bytes", maxSnapshotBytes)
	}
	file, err := os.Open(path)
	if err != nil {
		return LoadedSnapshot{}, fmt.Errorf("open OSM snapshot: %w", err)
	}
	defer file.Close()
	content, err := io.ReadAll(io.LimitReader(file, maxSnapshotBytes+1))
	if err != nil {
		return LoadedSnapshot{}, fmt.Errorf("read OSM snapshot: %w", err)
	}
	if len(content) > maxSnapshotBytes {
		return LoadedSnapshot{}, fmt.Errorf("OSM snapshot exceeds %d bytes", maxSnapshotBytes)
	}
	snapshot, err := ParseSnapshot(content)
	if err != nil {
		return LoadedSnapshot{}, err
	}
	digest := sha256.Sum256(content)
	return LoadedSnapshot{Snapshot: snapshot, Content: content, SHA256: hex.EncodeToString(digest[:]), ModTime: fileInfo.ModTime().UTC()}, nil
}

func ParseSnapshot(content []byte) (Snapshot, error) {
	if len(content) > maxSnapshotBytes {
		return Snapshot{}, fmt.Errorf("OSM snapshot exceeds %d bytes", maxSnapshotBytes)
	}
	var raw rawSnapshot
	decoder := json.NewDecoder(strings.NewReader(string(content)))
	if err := decoder.Decode(&raw); err != nil {
		return Snapshot{}, fmt.Errorf("decode OSM snapshot: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return Snapshot{}, errors.New("OSM snapshot contains multiple JSON values")
		}
		return Snapshot{}, fmt.Errorf("read OSM snapshot tail: %w", err)
	}

	snapshot := Snapshot{Nodes: make(map[int64]Point), Ways: make([]Way, 0), SourceTimestamp: nil}
	seenWays := make(map[int64]struct{})
	for index, encoded := range raw.Elements {
		var element rawElement
		if err := json.Unmarshal(encoded, &element); err != nil {
			return Snapshot{}, fmt.Errorf("decode OSM element %d: %w", index, err)
		}
		if element.ID <= 0 {
			return Snapshot{}, fmt.Errorf("OSM element %d has invalid id", index)
		}
		switch element.Type {
		case "node":
			if element.Lat == nil || element.Lon == nil || !validCoordinate(*element.Lon, *element.Lat) {
				return Snapshot{}, fmt.Errorf("OSM node %d has invalid coordinates", element.ID)
			}
			point := Point{NodeID: element.ID, Lon: *element.Lon, Lat: *element.Lat}
			if previous, found := snapshot.Nodes[element.ID]; found && !sameCoordinate(previous, point) {
				return Snapshot{}, fmt.Errorf("OSM node %d has conflicting coordinates", element.ID)
			}
			snapshot.Nodes[element.ID] = point
		case "way":
			if _, found := seenWays[element.ID]; found {
				return Snapshot{}, fmt.Errorf("OSM way %d is duplicated", element.ID)
			}
			seenWays[element.ID] = struct{}{}
			snapshot.Ways = append(snapshot.Ways, Way{ID: element.ID, NodeIDs: append([]int64(nil), element.Nodes...), Tags: cloneTags(element.Tags)})
		}
	}
	return snapshot, nil
}

func ValidateSourceURL(raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme != "https" || parsed.User != nil || parsed.Path == "" {
		return errors.New("source URL must be an HTTPS public OpenStreetMap URL")
	}
	if parsed.Port() != "" {
		return errors.New("source URL must not specify a port")
	}
	switch strings.ToLower(parsed.Hostname()) {
	case "api.openstreetmap.org":
		return nil
	default:
		return errors.New("source URL host is not an allowed OpenStreetMap public domain")
	}
}

func validCoordinate(lon, lat float64) bool {
	return math.IsNaN(lon) == false && math.IsInf(lon, 0) == false && math.IsNaN(lat) == false && math.IsInf(lat, 0) == false && lon >= -180 && lon <= 180 && lat >= -90 && lat <= 90
}

func cloneTags(tags map[string]string) map[string]string {
	if tags == nil {
		return nil
	}
	clone := make(map[string]string, len(tags))
	for key, value := range tags {
		clone[key] = value
	}
	return clone
}
