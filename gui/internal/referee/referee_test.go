package referee

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeTable(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "robot-heights.yml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	return path
}

func TestDescribeLooksTeamsUp(t *testing.T) {
	path := writeTable(t, "ER-Force: 148.0\nTIGERs Mannheim: 150.0\nibis: 90.0\n")
	teams := &Teams{}
	teams.Record("ER-Force", "Unknown FC", time.Unix(1000, 0))

	s := Describe(teams, &Heights{}, path)

	if s.Heard == nil || s.HeightsError != "" {
		t.Fatalf("state = %+v, want heard and a readable table", s)
	}

	if s.Yellow.Height == nil || *s.Yellow.Height != 148 {
		t.Errorf("yellow = %+v, want ER-Force at 148", s.Yellow)
	}

	if s.Blue.Height != nil {
		t.Errorf("blue = %+v, want no height for a team not in the table", s.Blue)
	}

	if s.MaxHeight != 150 || s.MeanHeight < 129.33 || s.MeanHeight > 129.34 {
		t.Errorf("mean=%v max=%v, want 129.33 and 150", s.MeanHeight, s.MaxHeight)
	}
}

func TestDescribeReportsAnUnreadableTable(t *testing.T) {
	s := Describe(&Teams{}, &Heights{}, filepath.Join(t.TempDir(), "missing.yml"))

	if s.HeightsError == "" || s.Heard != nil {
		t.Fatalf("state = %+v, want a table error and nothing heard", s)
	}
}

func TestHeightsRereadsAChangedFile(t *testing.T) {
	path := writeTable(t, "A: 100\n")
	h := &Heights{}

	if table, _ := h.load(path); table["A"] != 100 {
		t.Fatalf("table = %v, want A at 100", table)
	}

	// A different size is enough to notice the change, whatever the clock.
	if err := os.WriteFile(path, []byte("A: 120\nB: 130\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if table, _ := h.load(path); table["A"] != 120 || table["B"] != 130 {
		t.Fatalf("table = %v, want the new values", table)
	}
}

func TestMatchUsesTheChosenHeightsFile(t *testing.T) {
	path := writeTable(t, "ER-Force: 148\n")
	m := &Match{}
	m.Teams.Record("ER-Force", "", time.Unix(1000, 0))

	if s := m.State(); s.HeightsError == "" {
		t.Fatalf("state = %+v, want an error before a heights file is set", s)
	}

	m.SetHeightsFile(path)

	if s := m.State(); s.HeightsFile != path || s.Yellow.Height == nil || *s.Yellow.Height != 148 {
		t.Fatalf("state = %+v, want ER-Force at 148 from %s", s, path)
	}
}
