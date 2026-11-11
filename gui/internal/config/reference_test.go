package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/geometry"
	"gopkg.in/yaml.v3"
)

// The read-only reference files at the repo root's config/, which users copy.
const referenceDir = "../../../config"

func TestReferenceVisionYMLParses(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(referenceDir, "vision.yml"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	if _, err := Parse(data); err != nil {
		t.Fatalf("config/vision.yml doesn't parse: %v", err)
	}
}

func TestReferenceLegacyFilesParse(t *testing.T) {
	for _, name := range []string{"geometry-divA.yml", "geometry-divB.yml"} {
		data, err := os.ReadFile(filepath.Join(referenceDir, "legacy", name))
		if err != nil {
			t.Fatal(err)
		}

		if _, _, _, err := geometry.ParseFieldFile(data, name); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}

	for _, name := range []string{"config.yml", "config-minimal.yml"} {
		data, err := os.ReadFile(filepath.Join(referenceDir, "legacy", name))
		if err != nil {
			t.Fatal(err)
		}

		var raw map[string]any
		if err := yaml.Unmarshal(data, &raw); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestAtomicWriteRefusesAReadOnlyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reference.yml")
	if err := os.WriteFile(path, []byte("original\n"), 0o444); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := atomicWrite(path, []byte("replaced\n")); !errors.Is(err, errReadOnly) {
		t.Fatalf("atomicWrite = %v, want errReadOnly", err)
	}

	if data, _ := os.ReadFile(path); string(data) != "original\n" {
		t.Fatalf("file = %q, want it untouched", data)
	}
}
