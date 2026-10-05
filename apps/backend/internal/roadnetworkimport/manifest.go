package roadnetworkimport

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

func (box BBox) MarshalJSON() ([]byte, error) {
	return json.Marshal([4]float64{box.West, box.South, box.East, box.North})
}

func (box *BBox) UnmarshalJSON(data []byte) error {
	var values [4]float64
	if err := json.Unmarshal(data, &values); err != nil {
		return err
	}
	box.West, box.South, box.East, box.North = values[0], values[1], values[2], values[3]
	return box.Validate()
}

func NewManifest(plan Plan, bbox BBox, sourceURL, sha256Hex, rawSnapshotPath string, retrievedAt time.Time) Manifest {
	return Manifest{Provider: providerName, License: providerLicense, SourceURL: sourceURL, RetrievedAt: retrievedAt.UTC(), ContentSHA256: sha256Hex, BBox: bbox, RoadCount: len(plan.Roads), SegmentCount: len(plan.Segments), RawSnapshotPath: filepath.ToSlash(rawSnapshotPath), ExcludedCounts: cloneCounts(plan.Excluded)}
}

func ImportID(contentSHA256 string) uuid.UUID {
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte(providerCode+"\x00"+contentSHA256))
}

func WriteManifest(path string, manifest Manifest) error {
	if err := ValidateArtifactPath(path); err != nil {
		return err
	}
	return writeManifest(path, manifest)
}

func WriteResolvedManifest(path string, manifest Manifest) error {
	if !filepath.IsAbs(path) {
		return errors.New("resolved manifest path must be absolute")
	}
	return writeManifest(path, manifest)
}

func writeManifest(path string, manifest Manifest) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("encode import manifest: %w", err)
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create manifest directory: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write import manifest: %w", err)
	}
	return nil
}

func ValidateArtifactPath(path string) error {
	clean := filepath.Clean(strings.TrimSpace(path))
	if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return errors.New("artifact path must be a relative path within the repository")
	}
	if !strings.HasPrefix(filepath.ToSlash(clean), "data/") {
		return errors.New("artifact path must be under data/")
	}
	return nil
}

func ValidateManifest(manifest Manifest) error {
	if manifest.Provider != providerName || manifest.License != providerLicense || manifest.RoadCount < 0 || manifest.SegmentCount < 0 || manifest.RetrievedAt.IsZero() || manifest.BBox.Validate() != nil {
		return errors.New("invalid OSM import manifest")
	}
	if err := ValidateSourceURL(manifest.SourceURL); err != nil {
		return err
	}
	if len(manifest.ContentSHA256) != sha256.Size*2 {
		return errors.New("manifest sha256 must be 64 hexadecimal characters")
	}
	if _, err := hex.DecodeString(manifest.ContentSHA256); err != nil {
		return errors.New("manifest sha256 must be hexadecimal")
	}
	if manifest.RawSnapshotPath == "" {
		return errors.New("manifest raw snapshot path is required")
	}
	return ValidateArtifactPath(manifest.RawSnapshotPath)
}

func cloneCounts(values map[string]int) map[string]int {
	clone := make(map[string]int, len(values))
	for key, value := range values {
		clone[key] = value
	}
	return clone
}
