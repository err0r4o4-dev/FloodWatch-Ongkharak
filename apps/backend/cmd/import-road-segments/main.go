package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/roadnetworkimport"
	"github.com/jackc/pgx/v5/pgxpool"
)

type options struct {
	input           string
	bboxRaw         string
	sourceURL       string
	retrievedAtRaw  string
	manifestPath    string
	rawSnapshotPath string
	dryRun          bool
	apply           bool
}

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "import road segments failed: %v\n", err)
		os.Exit(1)
	}
}

func parseOptions(args []string) (options, error) {
	result := options{}
	flags := flag.NewFlagSet("import-road-segments", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var snapshotFile string
	flags.StringVar(&result.input, "input", "", "local OSM JSON snapshot")
	flags.StringVar(&snapshotFile, "snapshot-file", "", "rejected legacy alias; use --input")
	flags.StringVar(&result.bboxRaw, "bbox", "", "processing bbox west,south,east,north")
	flags.StringVar(&result.sourceURL, "source-url", "", "public OpenStreetMap source URL")
	flags.StringVar(&result.retrievedAtRaw, "retrieved-at", "", "retrieval timestamp in RFC3339")
	flags.StringVar(&result.manifestPath, "manifest", "", "relative data/ manifest path")
	flags.StringVar(&result.rawSnapshotPath, "raw-snapshot-path", "", "relative data/ raw snapshot artifact path")
	flags.BoolVar(&result.dryRun, "dry-run", true, "validate and report without database writes")
	flags.BoolVar(&result.apply, "apply", false, "apply the import to DATABASE_URL")
	if err := flags.Parse(args); err != nil {
		return options{}, err
	}
	if flags.NArg() != 0 {
		return options{}, errors.New("unexpected positional arguments")
	}
	if snapshotFile != "" {
		return options{}, errors.New("--snapshot-file is not accepted; use --input for the source snapshot")
	}
	if result.input == "" {
		return options{}, errors.New("--input is required")
	}
	if result.apply {
		result.dryRun = false
		if strings.TrimSpace(result.sourceURL) == "" {
			return options{}, errors.New("--source-url is required with --apply")
		}
		if result.rawSnapshotPath == "" {
			result.rawSnapshotPath = filepath.ToSlash(filepath.Join("data", "road-network", filepath.Base(result.input)))
		}
	} else if !result.dryRun {
		return options{}, errors.New("use --apply for database writes")
	}
	if _, err := roadnetworkimport.ParseBBox(result.bboxRaw); err != nil {
		return options{}, fmt.Errorf("invalid --bbox: %w", err)
	}
	if result.sourceURL != "" {
		if err := roadnetworkimport.ValidateSourceURL(result.sourceURL); err != nil {
			return options{}, fmt.Errorf("invalid --source-url: %w", err)
		}
	}
	if result.manifestPath != "" {
		if err := roadnetworkimport.ValidateArtifactPath(result.manifestPath); err != nil {
			return options{}, fmt.Errorf("invalid --manifest: %w", err)
		}
	}
	if result.rawSnapshotPath != "" {
		if err := roadnetworkimport.ValidateArtifactPath(result.rawSnapshotPath); err != nil {
			return options{}, fmt.Errorf("invalid --raw-snapshot-path: %w", err)
		}
	}
	if result.retrievedAtRaw != "" {
		if _, err := time.Parse(time.RFC3339, result.retrievedAtRaw); err != nil {
			return options{}, fmt.Errorf("invalid --retrieved-at: %w", err)
		}
	}
	return result, nil
}

func run(args []string, output io.Writer) error {
	options, err := parseOptions(args)
	if err != nil {
		return err
	}
	if options.manifestPath != "" {
		if err := validateManifestDestination(resolveArtifactPath(options.manifestPath), options.input, resolveArtifactPath(options.rawSnapshotPath)); err != nil {
			return err
		}
	}
	bbox, err := roadnetworkimport.ParseBBox(options.bboxRaw)
	if err != nil {
		return err
	}
	loaded, err := roadnetworkimport.ReadSnapshotFile(options.input)
	if err != nil {
		return err
	}
	plan, err := roadnetworkimport.Normalize(loaded.Snapshot, bbox)
	if err != nil {
		return fmt.Errorf("normalize OSM snapshot: %w", err)
	}
	retrievedAt := loaded.ModTime
	if retrievedAt.IsZero() {
		retrievedAt = time.Now().UTC()
	}
	if options.retrievedAtRaw != "" {
		retrievedAt, err = time.Parse(time.RFC3339, options.retrievedAtRaw)
		if err != nil {
			return fmt.Errorf("parse retrieval timestamp: %w", err)
		}
	}
	manifest := roadnetworkimport.NewManifest(plan, bbox, options.sourceURL, loaded.SHA256, options.rawSnapshotPath, retrievedAt)
	if !options.apply {
		if options.manifestPath != "" {
			if err := roadnetworkimport.WriteResolvedManifest(resolveArtifactPath(options.manifestPath), manifest); err != nil {
				return err
			}
		}
		return writeReport(output, importReport{DryRun: true, Manifest: manifest, Excluded: plan.Excluded})
	}
	if err := verifyRawArtifact(resolveArtifactPath(options.rawSnapshotPath), loaded.SHA256); err != nil {
		return err
	}
	if options.manifestPath != "" {
		if err := preflightManifestPath(resolveArtifactPath(options.manifestPath)); err != nil {
			return err
		}
	}
	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		return errors.New("DATABASE_URL is required with --apply")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return errors.New("configure PostgreSQL connection pool failed")
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		return errors.New("PostgreSQL is unavailable")
	}
	result, err := roadnetworkimport.Apply(ctx, pool, plan, manifest)
	if err != nil {
		return err
	}
	if options.manifestPath != "" {
		if err := roadnetworkimport.WriteResolvedManifest(resolveArtifactPath(options.manifestPath), result.Manifest); err != nil {
			return err
		}
	}
	return writeReport(output, importReport{Applied: true, Manifest: result.Manifest, ImportID: result.ImportID.String(), RoadsInserted: result.RoadsInserted, RoadsReused: result.RoadsReused, SegmentsAdded: result.SegmentsAdded, SegmentsReused: result.SegmentsReused, Excluded: result.Excluded})
}

type importReport struct {
	DryRun         bool                       `json:"dryRun"`
	Applied        bool                       `json:"applied"`
	ImportID       string                     `json:"importID,omitempty"`
	Manifest       roadnetworkimport.Manifest `json:"manifest"`
	RoadsInserted  int                        `json:"roadsInserted,omitempty"`
	RoadsReused    int                        `json:"roadsReused,omitempty"`
	SegmentsAdded  int                        `json:"segmentsAdded,omitempty"`
	SegmentsReused int                        `json:"segmentsReused,omitempty"`
	Excluded       map[string]int             `json:"excludedCounts"`
}

func writeReport(output io.Writer, report importReport) error {
	encoder := json.NewEncoder(output)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}

func verifyRawArtifact(path, expectedSHA string) error {
	loaded, err := roadnetworkimport.ReadSnapshotFile(path)
	if err != nil {
		return fmt.Errorf("verify raw snapshot artifact: %w", err)
	}
	if loaded.SHA256 != expectedSHA {
		return errors.New("raw snapshot artifact does not match --input")
	}
	return nil
}

func resolveArtifactPath(path string) string {
	workingDirectory, err := os.Getwd()
	if err != nil {
		return filepath.Clean(path)
	}
	return resolveArtifactPathFrom(workingDirectory, path)
}

func resolveArtifactPathFrom(workingDirectory, path string) string {
	clean := filepath.Clean(path)
	if filepath.IsAbs(clean) {
		return clean
	}
	for current := workingDirectory; ; current = filepath.Dir(current) {
		if _, err := os.Stat(filepath.Join(current, "apps", "backend", "go.mod")); err == nil {
			return filepath.Join(current, clean)
		}
		parent := filepath.Dir(current)
		if parent == current {
			return filepath.Join(workingDirectory, clean)
		}
	}
}

func preflightManifestPath(path string) error {
	info, err := os.Stat(path)
	if err == nil {
		if info.IsDir() {
			return errors.New("manifest path is a directory")
		}
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
		if err != nil {
			return errors.New("manifest path is not writable")
		}
		return file.Close()
	}
	if !errors.Is(err, os.ErrNotExist) {
		return errors.New("manifest path cannot be inspected")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return errors.New("manifest directory is not writable")
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".road-import-manifest-*")
	if err != nil {
		return errors.New("manifest directory is not writable")
	}
	temporaryPath := temporary.Name()
	if err := temporary.Close(); err != nil {
		_ = os.Remove(temporaryPath)
		return errors.New("manifest directory is not writable")
	}
	if err := os.Remove(temporaryPath); err != nil {
		return errors.New("manifest directory cleanup failed")
	}
	return nil
}

func validateManifestDestination(manifestPath string, sourcePaths ...string) error {
	manifestAbsolute, err := filepath.Abs(manifestPath)
	if err != nil {
		return errors.New("manifest path cannot be resolved")
	}
	manifestInfo, manifestErr := os.Stat(manifestAbsolute)
	if manifestErr != nil && !errors.Is(manifestErr, os.ErrNotExist) {
		return errors.New("manifest path cannot be inspected")
	}
	for _, sourcePath := range sourcePaths {
		if sourcePath == "" {
			continue
		}
		sourceAbsolute, err := filepath.Abs(sourcePath)
		if err != nil {
			return errors.New("source artifact path cannot be resolved")
		}
		if manifestAbsolute == sourceAbsolute {
			return errors.New("manifest destination must not overwrite a source snapshot")
		}
		sourceInfo, sourceErr := os.Stat(sourceAbsolute)
		if manifestErr == nil && sourceErr == nil && os.SameFile(manifestInfo, sourceInfo) {
			return errors.New("manifest destination must not alias a source snapshot")
		}
	}
	return nil
}
