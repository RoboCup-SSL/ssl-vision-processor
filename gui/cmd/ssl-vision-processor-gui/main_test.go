package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/config"
)

func TestBootstrapConfigImportsLegacyFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vision.yml")

	if err := bootstrapConfig(path, "testdata/geometry.yml", "testdata/config.yml", "missing-preset.yml"); err != nil {
		t.Fatalf("bootstrapConfig: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	doc, err := config.Parse(data)
	if err != nil {
		t.Fatalf("bootstrapped file doesn't parse: %v", err)
	}

	if len(doc.Cameras) != 1 || doc.Cameras[0].ConfigPath != "testdata/config.yml" || doc.Cameras[0].Seed == nil {
		t.Fatalf("cameras = %+v, want the legacy config imported as camera with its corners", doc.Cameras)
	}
}

func TestBootstrapConfigFallsBackToThePreset(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "vision.yml")

	if err := bootstrapConfig(path, filepath.Join(dir, "none.yml"), filepath.Join(dir, "none.yml"), "testdata/geometry.yml"); err != nil {
		t.Fatalf("bootstrapConfig: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("no file created: %v", err)
	}
}

func TestBootstrapConfigLeavesExistingFileAlone(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vision.yml")

	if err := os.WriteFile(path, []byte("already here"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := bootstrapConfig(path, "testdata/geometry.yml", "", "testdata/geometry.yml"); err != nil {
		t.Fatalf("bootstrapConfig: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	if string(got) != "already here" {
		t.Errorf("existing file was overwritten: %q", got)
	}
}

func TestFormattedAddressUsesAnExplicitHostAsIs(t *testing.T) {
	got := formattedAddress("192.168.1.50:8085")
	want := "http://192.168.1.50:8085"

	if got != want {
		t.Errorf("formattedAddress = %q, want %q", got, want)
	}
}

// A bind-all host has no single deterministic answer in a test environment
// (LAN IP if one's reachable, localhost otherwise) -- what's worth asserting
// is that it always resolves to a well-formed URL on the right port.
func TestFormattedAddressResolvesABindAllHostToAURL(t *testing.T) {
	got := formattedAddress(":8085")

	if !strings.HasPrefix(got, "http://") || !strings.HasSuffix(got, ":8085") {
		t.Errorf("formattedAddress(%q) = %q, want an http://<host>:8085 URL", ":8085", got)
	}
}

func TestFormattedAddressFallsBackOnUnparseableInput(t *testing.T) {
	got := formattedAddress("not-a-host-port")
	want := "http://not-a-host-port"

	if got != want {
		t.Errorf("formattedAddress = %q, want %q", got, want)
	}
}
