package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
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

func TestReferenceLegacyFilesImport(t *testing.T) {
	for _, pair := range [][2]string{
		{"geometry-divA.yml", "config.yml"},
		{"geometry-divB.yml", "config.yml"},
		{"geometry-divB.yml", "config-minimal.yml"},
	} {
		t.Run(pair[0]+"+"+pair[1], func(t *testing.T) {
			_, err := Import(
				filepath.Join(referenceDir, "legacy", pair[0]),
				filepath.Join(referenceDir, "legacy", pair[1]),
			)
			if err != nil {
				t.Fatalf("Import: %v", err)
			}
		})
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
