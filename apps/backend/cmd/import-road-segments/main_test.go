package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManifestCannotOverwriteSourceSnapshots(t *testing.T) {
	for _, apply := range []bool{false, true} {
		for _, collision := range []string{"input", "raw", "hard-link"} {
			t.Run(collision+map[bool]string{false: "-dry-run", true: "-apply"}[apply], func(t *testing.T) {
				directory := t.TempDir()
				t.Chdir(directory)
				if err := os.Mkdir("data", 0o755); err != nil {
					t.Fatal(err)
				}
				source := []byte(`{"elements":[]}`)
				for _, path := range []string{"data/input.json", "data/raw.json"} {
					if err := os.WriteFile(path, source, 0o644); err != nil {
						t.Fatal(err)
					}
				}
				manifest := "data/" + collision + ".json"
				if collision == "hard-link" {
					if err := os.Link("data/input.json", manifest); err != nil {
						t.Fatal(err)
					}
				}
				args := []string{"--input", "data/input.json", "--bbox", "100.98,14.02,101.08,14.14", "--raw-snapshot-path", "data/raw.json", "--manifest", manifest}
				if apply {
					args = append(args, "--apply", "--source-url", "https://api.openstreetmap.org/api/0.6/map.json")
				}
				if err := run(args, io.Discard); err == nil || !strings.Contains(err.Error(), "source snapshot") {
					t.Fatalf("collision error = %v, want source snapshot rejection", err)
				}
				for _, path := range []string{"data/input.json", "data/raw.json"} {
					retained, err := os.ReadFile(path)
					if err != nil || string(retained) != string(source) {
						t.Fatalf("source %s changed: %q, %v", path, retained, err)
					}
				}
			})
		}
	}
}

func TestParseOptionsDefaultsToDryRun(t *testing.T) {
	options, err := parseOptions([]string{"--input", "snapshot.json", "--bbox", "100.98,14.02,101.08,14.14"})
	if err != nil {
		t.Fatal(err)
	}
	if !options.dryRun || options.apply {
		t.Fatalf("options = %#v, want safe dry-run defaults", options)
	}
}

func TestParseOptionsRequiresSourceURLForApply(t *testing.T) {
	_, err := parseOptions([]string{"--input", "snapshot.json", "--bbox", "100.98,14.02,101.08,14.14", "--apply"})
	if err == nil {
		t.Fatal("parseOptions accepted apply without source URL")
	}
}

func TestParseOptionsRejectsSnapshotFileAlias(t *testing.T) {
	_, err := parseOptions([]string{"--snapshot-file", "snapshot.json", "--bbox", "100.98,14.02,101.08,14.14"})
	if err == nil {
		t.Fatal("parseOptions accepted the snapshot-file alias")
	}
}

func TestResolveArtifactPathUsesAbsoluteFallback(t *testing.T) {
	path := resolveArtifactPathFrom(t.TempDir(), "data/road-network/manifest.json")
	if !filepath.IsAbs(path) || !strings.HasSuffix(filepath.ToSlash(path), "data/road-network/manifest.json") {
		t.Fatalf("resolved path = %q, want absolute data artifact path", path)
	}
}

func TestPreflightManifestPathRejectsUnwritableTarget(t *testing.T) {
	directory := t.TempDir()
	target := filepath.Join(directory, "manifest-directory")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := preflightManifestPath(target); err == nil {
		t.Fatal("preflight accepted a directory as a manifest target")
	}
}

func TestParseOptionsRejectsAbsoluteRawSnapshotPath(t *testing.T) {
	_, err := parseOptions([]string{"--input", "snapshot.json", "--bbox", "100.98,14.02,101.08,14.14", "--raw-snapshot-path", "C:\\source.json"})
	if err == nil {
		t.Fatal("parseOptions accepted an absolute raw snapshot path")
	}
}
