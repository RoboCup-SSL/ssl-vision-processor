package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/config"
)

func TestBootstrapConfigCreatesAParseableDefault(t *testing.T) {
	dir := t.TempDir()

	if err := bootstrapConfig(dir, "vision.yml", false); err != nil {
		t.Fatalf("bootstrapConfig: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "vision.yml"))
	if err != nil {
		t.Fatalf("no file created: %v", err)
	}

	doc, err := config.Parse(data)
	if err != nil {
		t.Fatalf("bootstrapped file doesn't parse: %v", err)
	}

	if path := doc.Cameras[0].ConfigPath; path != "" {
		t.Errorf("camera 0 config_path = %q, want none without -runCameras", path)
	}

	heights, err := os.ReadFile(filepath.Join(dir, "robot-heights.yml"))
	if err != nil || len(heights) == 0 {
		t.Errorf("no robot height table beside it: %v", err)
	}
}

func TestBootstrapConfigLeavesExistingFileAlone(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "vision.yml")

	if err := os.WriteFile(path, []byte("already here"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := bootstrapConfig(dir, "vision.yml", true); err != nil {
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

// With -runCameras, a webcam works on the first run: camera 0 has a file to
// run with.
func TestBootstrapConfigRunCamerasGivesCameraZeroAFile(t *testing.T) {
	dir := t.TempDir()

	if err := bootstrapConfig(dir, "vision.yml", true); err != nil {
		t.Fatalf("bootstrapConfig: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "vision.yml"))

	doc, err := config.Parse(data)
	if err != nil {
		t.Fatal(err)
	}

	if path := doc.Cameras[0].ConfigPath; path != "cam0.yml" {
		t.Errorf("camera 0 config_path = %q, want cam0.yml", path)
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
