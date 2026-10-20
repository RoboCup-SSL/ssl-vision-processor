package alerts

import (
	"encoding/json"
	"flag"
	"os"
	"slices"
	"testing"

	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/config"
)

var update = flag.Bool("update", false, "rewrite the frontend's alert-codes.json from Codes()")

// codesFile is where the frontend's tests read the codes from, to check the
// GUI words every one (alerts.test.ts).
const codesFile = "../../frontend/src/lib/alerts/alert-codes.json"

func TestCodesFileIsCurrent(t *testing.T) {
	want, err := json.MarshalIndent(Codes(), "", "  ")
	if err != nil {
		t.Fatal(err)
	}

	want = append(want, '\n')

	if *update {
		if err := os.WriteFile(codesFile, want, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	got, err := os.ReadFile(codesFile)
	if err != nil {
		t.Fatalf("%v (regenerate with go test ./internal/alerts -run Codes -update)", err)
	}

	if string(got) != string(want) {
		t.Errorf("%s is out of date; regenerate with go test ./internal/alerts -run Codes -update", codesFile)
	}
}

// A camera warning missing from cameraWarnings would still show, but always
// as a caution.
func TestEveryCameraWarningHasASeverity(t *testing.T) {
	for _, code := range config.WarningCodes {
		if _, ok := cameraWarnings[code]; !ok {
			t.Errorf("camera warning %q has no severity in cameraWarnings", code)
		}
	}

	if len(cameraWarnings) != len(config.WarningCodes) {
		t.Errorf("cameraWarnings has %d codes, config.WarningCodes %d", len(cameraWarnings), len(config.WarningCodes))
	}

	if !slices.IsSorted(Codes()) {
		t.Error("Codes() isn't sorted")
	}
}
