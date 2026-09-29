package geometry

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testColorConfig() ColorConfig {
	return ColorConfig{
		ReferenceForce:    0.2,
		HistoryForce:      0.5,
		MinReferenceForce: 0.05,
		Orange:            RGB{200, 130, 70},
		Field:             RGB{130, 130, 130},
		Yellow:            RGB{250, 130, 10},
		Blue:              RGB{10, 130, 250},
		Green:             RGB{10, 250, 130},
		Pink:              RGB{250, 10, 130},
	}
}

func TestWriteColorConfigUncommentsAndUpdatesValuesInPlace(t *testing.T) {
	path := testConfigFile(t)

	if err := WriteColorConfig(path, testColorConfig()); err != nil {
		t.Fatalf("WriteColorConfig: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	for _, want := range []string{
		"reference_force: 0.2",
		"history_force: 0.5",
		"min_reference_force: 0.05",
		"orange: [200, 130, 70]",
		"field: [130, 130, 130]",
		"yellow: [250, 130, 10]",
		"blue: [10, 130, 250]",
		"green: [10, 250, 130]",
		"pink: [250, 10, 130]",
	} {
		if !strings.Contains(string(got), want) {
			t.Errorf("output missing %q:\n%s", want, got)
		}
	}

	if strings.Contains(string(got), "#orange") || strings.Contains(string(got), "#reference_force") {
		t.Errorf("commented-out key survived uncommenting:\n%s", got)
	}
}

func TestWriteColorConfigPreservesComments(t *testing.T) {
	path := testConfigFile(t)

	before, err := os.ReadFile("testdata/config.yml")
	if err != nil {
		t.Fatalf("ReadFile fixture: %v", err)
	}

	if err := WriteColorConfig(path, testColorConfig()); err != nil {
		t.Fatalf("WriteColorConfig: %v", err)
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	// Every *explanatory* comment survives untouched. The commented-out key
	// lines themselves (#orange: [...], #reference_force: ...) are expected
	// to disappear -- that's WriteColorConfig uncommenting them into their
	// live equivalent, not a lost comment.
	writtenKeys := []string{"reference_force:", "history_force:", "min_reference_force:", "orange:", "field:", "yellow:", "blue:", "green:", "pink:"}

	for _, line := range strings.Split(string(before), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || !strings.HasPrefix(trimmed, "#") {
			continue
		}

		isTargetKey := false
		for _, key := range writtenKeys {
			if strings.HasPrefix(strings.TrimPrefix(trimmed, "#"), key) {
				isTargetKey = true

				break
			}
		}

		if isTargetKey {
			continue
		}

		if !strings.Contains(string(after), line) {
			t.Errorf("comment line lost: %q", line)
		}
	}

	if !strings.Contains(string(after), "camera_amount: 2") {
		t.Error("unrelated 'geometry' section was disturbed")
	}
}

func TestWriteColorConfigUpdatesAnAlreadyLiveKeyInPlace(t *testing.T) {
	path := testConfigFile(t)

	if err := WriteColorConfig(path, testColorConfig()); err != nil {
		t.Fatalf("WriteColorConfig (1st): %v", err)
	}

	second := testColorConfig()
	second.Orange = RGB{1, 2, 3}

	if err := WriteColorConfig(path, second); err != nil {
		t.Fatalf("WriteColorConfig (2nd): %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	if strings.Count(string(got), "orange:") != 1 {
		t.Errorf("expected exactly one 'orange:' key, got:\n%s", got)
	}

	if !strings.Contains(string(got), "orange: [1, 2, 3]") {
		t.Errorf("orange was not updated in place:\n%s", got)
	}
}

func TestWriteColorConfigAppendsMissingKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yml")
	data := "color:\n  #reference_force: 0.1\n"
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := WriteColorConfig(path, testColorConfig()); err != nil {
		t.Fatalf("WriteColorConfig: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	if !strings.Contains(string(got), "orange: [200, 130, 70]") {
		t.Errorf("missing 'orange' key was not appended:\n%s", got)
	}
}

func TestWriteColorConfigErrorsWithoutAColorSection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(path, []byte("camera:\n  driver: OPENCV\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := WriteColorConfig(path, testColorConfig()); err == nil {
		t.Fatal("expected an error, got nil")
	}
}

func TestWriteColorConfigErrorsOnAMissingFile(t *testing.T) {
	err := WriteColorConfig(filepath.Join(t.TempDir(), "does-not-exist.yml"), testColorConfig())
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
}

func TestWriteColorConfigRejectsInvalidValues(t *testing.T) {
	base := testColorConfig()

	cases := map[string]func(c *ColorConfig){
		"referenceForce out of range": func(c *ColorConfig) { c.ReferenceForce = 1.5 },
		"forces sum over 1":           func(c *ColorConfig) { c.ReferenceForce, c.HistoryForce = 0.6, 0.6 },
		"reference below floor":       func(c *ColorConfig) { c.ReferenceForce, c.MinReferenceForce = 0.01, 0.05 },
		"channel out of range":        func(c *ColorConfig) { c.Orange.R = 300 },
	}

	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			path := testConfigFile(t)

			cfg := base
			mutate(&cfg)

			if err := WriteColorConfig(path, cfg); err == nil {
				t.Fatal("expected an error, got nil")
			}
		})
	}
}

func TestReadColorConfigRoundTripsWhatWriteColorConfigSaved(t *testing.T) {
	path := testConfigFile(t)

	want := testColorConfig()
	if err := WriteColorConfig(path, want); err != nil {
		t.Fatalf("WriteColorConfig: %v", err)
	}

	got, err := ReadColorConfig(path)
	if err != nil {
		t.Fatalf("ReadColorConfig: %v", err)
	}

	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestReadColorConfigReturnsVisionProcessorsDefaultsWhenNothingIsSetYet(t *testing.T) {
	path := testConfigFile(t)

	got, err := ReadColorConfig(path)
	if err != nil {
		t.Fatalf("ReadColorConfig: %v", err)
	}

	want := ColorConfig{
		ReferenceForce:    0.1,
		HistoryForce:      0.7,
		MinReferenceForce: defaultMinReferenceForce,
		Orange:            RGB{192, 128, 64},
		Field:             RGB{128, 128, 128},
		Yellow:            RGB{255, 128, 0},
		Blue:              RGB{0, 128, 255},
		Green:             RGB{0, 255, 128},
		Pink:              RGB{255, 0, 128},
	}

	if got != want {
		t.Errorf("got %+v, want %+v (vision_processor's own Resources.cpp defaults)", got, want)
	}
}

func TestReadColorConfigErrorsOnAMissingFile(t *testing.T) {
	_, err := ReadColorConfig(filepath.Join(t.TempDir(), "does-not-exist.yml"))
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
}
