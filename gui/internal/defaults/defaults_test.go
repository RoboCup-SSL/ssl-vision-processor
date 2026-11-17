package defaults

import (
	"bytes"
	"os"
	"testing"
)

// The embedded copies are made by the Makefile; a stale one means the binary
// disagrees with the files a person would read.
func TestEmbeddedFilesMatchTheirSources(t *testing.T) {
	for embedded, source := range map[string]string{
		"geometry-divA.yml": "../../../config/legacy/geometry-divA.yml",
		"geometry-divB.yml": "../../../config/legacy/geometry-divB.yml",
		RobotHeightsFile:    "../../../robot-heights.yml",
		"README.md":         "../../README.md",
	} {
		got, err := read(embedded)
		if err != nil {
			t.Errorf("%s: %v", embedded, err)

			continue
		}

		want, err := os.ReadFile(source)
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(got, want) {
			t.Errorf("%s differs from %s; run make", embedded, source)
		}
	}
}

func TestPresetsAreTheTwoDivisions(t *testing.T) {
	got, err := Presets()
	if err != nil {
		t.Fatal(err)
	}

	if len(got) != 2 || got[0].Name != "Division A" || got[1].Name != "Division B" {
		t.Errorf("presets = %+v, want Division A then Division B", got)
	}
}
