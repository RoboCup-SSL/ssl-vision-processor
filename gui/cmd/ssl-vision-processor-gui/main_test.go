package main

import (
	"flag"
	"os"
	"path/filepath"
	"regexp"
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

func TestUfwEnabled(t *testing.T) {
	dir := t.TempDir()

	for content, want := range map[string]bool{
		"# comment\nENABLED=yes\nLOGLEVEL=low\n": true,
		"ENABLED=no\n":                           false,
		`ENABLED="yes"` + "\n":                   true,
		"":                                       false,
	} {
		path := filepath.Join(dir, "ufw.conf")
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}

		if got := ufwEnabled(path); got != want {
			t.Errorf("ufwEnabled(%q) = %v, want %v", content, got, want)
		}
	}

	if ufwEnabled(filepath.Join(dir, "missing")) {
		t.Error("no ufw installed reads as enabled")
	}
}

// The suggested ufw rule names the ports the host is set to, not the defaults.
func TestUDPPortsFollowTheConfig(t *testing.T) {
	h, _ := testHostWith(t, hostOptions{imgDir: t.TempDir()}, func(d *config.Document) {
		d.Defaults["network"] = map[string]any{"vision_port": 10020}
	})

	got := strings.Join(udpPorts(h.store), ",")
	if !strings.Contains(got, "10020") || strings.Contains(got, "10006") || !strings.Contains(got, "10003") {
		t.Errorf("udpPorts = %s, want the configured vision port 10020 and the default game controller 10003", got)
	}
}

// The package's man page lists every option, so it can't fall behind the flags.
func TestManPageListsEveryFlag(t *testing.T) {
	page, err := os.ReadFile("../../packaging/ssl-vision-processor-gui.1")
	if err != nil {
		t.Fatal(err)
	}

	flag.VisitAll(func(f *flag.Flag) {
		if strings.HasPrefix(f.Name, "test.") {
			return // the testing package's own
		}

		if !regexp.MustCompile(`(?m)^\.BI? \\-` + regexp.QuoteMeta(f.Name) + `( |$)`).Match(page) {
			t.Errorf("packaging/ssl-vision-processor-gui.1 doesn't describe -%s under OPTIONS", f.Name)
		}
	})
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
