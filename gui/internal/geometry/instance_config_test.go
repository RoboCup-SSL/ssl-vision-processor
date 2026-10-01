package geometry

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// testConfigFile copies testdata/config.yml into t.TempDir() so tests can
// freely write to it without touching the tracked fixture -- same discipline
// as testGeometry in geometry_test.go.
func testConfigFile(t *testing.T) string {
	t.Helper()

	src, err := os.ReadFile("testdata/config.yml")
	if err != nil {
		t.Fatalf("ReadFile fixture: %v", err)
	}

	path := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(path, src, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	return path
}

func TestWriteLineCornersUpdatesValuesInPlace(t *testing.T) {
	path := testConfigFile(t)

	corners := []Corner{{X: 1, Y: 2}, {X: 3, Y: 4}, {X: 5, Y: 6}, {X: 7, Y: 8}}
	if err := WriteLineCorners(path, corners, 1); err != nil {
		t.Fatalf("WriteLineCorners: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	want := "line_corners:\n  - [1, 2]\n  - [3, 4]\n  - [5, 6]\n  - [7, 8]\n"
	if !strings.Contains(string(got), want) {
		t.Errorf("output does not contain the new corners as a block sequence of flow tuples:\n%s", got)
	}

	if strings.Contains(string(got), "398, 422") {
		t.Error("old corner values are still present")
	}
}

func TestWriteLineCornersPreservesComments(t *testing.T) {
	path := testConfigFile(t)

	before, err := os.ReadFile("testdata/config.yml")
	if err != nil {
		t.Fatalf("ReadFile fixture: %v", err)
	}

	if err := WriteLineCorners(path, []Corner{{X: 1, Y: 2}, {X: 3, Y: 4}, {X: 5, Y: 6}, {X: 7, Y: 8}}, 1); err != nil {
		t.Fatalf("WriteLineCorners: %v", err)
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	// Every comment line and every other section is untouched -- only the
	// line_corners value itself should have changed.
	for _, line := range strings.Split(string(before), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || !strings.HasPrefix(trimmed, "#") {
			continue
		}

		if !strings.Contains(string(after), line) {
			t.Errorf("comment line lost: %q", line)
		}
	}

	if !strings.Contains(string(after), "camera_amount: 2") {
		t.Error("unrelated 'camera_amount' key was disturbed")
	}

	if !strings.Contains(string(after), "color:") {
		t.Error("unrelated 'color' section was disturbed")
	}
}

func TestWriteLineCornersAppendsTheKeyWhenAbsent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yml")
	data := "geometry:\n  camera_amount: 1\n"
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := WriteLineCorners(path, []Corner{{X: 1, Y: 2}, {X: 3, Y: 4}, {X: 5, Y: 6}, {X: 7, Y: 8}}, 1); err != nil {
		t.Fatalf("WriteLineCorners: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	if !strings.Contains(string(got), "line_corners:") {
		t.Errorf("line_corners key was not appended:\n%s", got)
	}

	if !strings.Contains(string(got), "camera_amount: 1") {
		t.Error("existing key was disturbed")
	}
}

func TestWriteLineCornersErrorsWithoutAGeometrySection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(path, []byte("camera:\n  driver: OPENCV\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	err := WriteLineCorners(path, []Corner{{X: 1, Y: 2}}, 1)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
}

func TestWriteLineCornersErrorsOnAMissingFile(t *testing.T) {
	err := WriteLineCorners(filepath.Join(t.TempDir(), "does-not-exist.yml"), []Corner{{X: 1, Y: 2}}, 1)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
}

func TestWriteLineCornersErrorsWithNoCorners(t *testing.T) {
	path := testConfigFile(t)

	if err := WriteLineCorners(path, nil, 1); err == nil {
		t.Fatal("expected an error, got nil")
	}
}

func TestWriteLineCornersErrorsWhenGoalSideMarkerIsOutOfRange(t *testing.T) {
	path := testConfigFile(t)

	corners := []Corner{{X: 1, Y: 2}, {X: 3, Y: 4}, {X: 5, Y: 6}, {X: 7, Y: 8}}

	for _, marker := range []int{0, 5, -1} {
		if err := WriteLineCorners(path, corners, marker); err == nil {
			t.Errorf("goalSideMarker=%d: expected an error, got nil", marker)
		}
	}
}

func TestWriteLineCornersRecordsTheGoalSideMarker(t *testing.T) {
	path := testConfigFile(t)

	corners := []Corner{{X: 1, Y: 2}, {X: 3, Y: 4}, {X: 5, Y: 6}, {X: 7, Y: 8}}
	if err := WriteLineCorners(path, corners, 3); err != nil {
		t.Fatalf("WriteLineCorners: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	if !strings.Contains(string(got), "goal_side_marker: 3") {
		t.Errorf("goal_side_marker was not recorded:\n%s", got)
	}
}

func TestReadLineCornersRoundTripsWhatWriteLineCornersSaved(t *testing.T) {
	path := testConfigFile(t)

	corners := []Corner{{X: 1, Y: 2}, {X: 3, Y: 4}, {X: 5, Y: 6}, {X: 7, Y: 8}}
	if err := WriteLineCorners(path, corners, 3); err != nil {
		t.Fatalf("WriteLineCorners: %v", err)
	}

	gotCorners, gotMarker, err := ReadLineCorners(path)
	if err != nil {
		t.Fatalf("ReadLineCorners: %v", err)
	}

	if len(gotCorners) != len(corners) {
		t.Fatalf("got %d corners, want %d", len(gotCorners), len(corners))
	}

	for i, c := range corners {
		if gotCorners[i] != c {
			t.Errorf("corner %d = %+v, want %+v", i, gotCorners[i], c)
		}
	}

	if gotMarker != 3 {
		t.Errorf("goalSideMarker = %d, want 3", gotMarker)
	}
}

func TestReadLineCornersReturnsEmptyWhenNothingSavedYet(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(path, []byte("geometry:\n  camera_amount: 1\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	corners, marker, err := ReadLineCorners(path)
	if err != nil {
		t.Fatalf("ReadLineCorners: %v", err)
	}

	if len(corners) != 0 {
		t.Errorf("corners = %+v, want none", corners)
	}

	if marker != 0 {
		t.Errorf("goalSideMarker = %d, want 0", marker)
	}
}

func TestReadLineCornersErrorsOnAMissingFile(t *testing.T) {
	_, _, err := ReadLineCorners(filepath.Join(t.TempDir(), "does-not-exist.yml"))
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
}

func TestWriteLineCornersUpdatesAnExistingGoalSideMarkerInPlace(t *testing.T) {
	path := testConfigFile(t)

	corners := []Corner{{X: 1, Y: 2}, {X: 3, Y: 4}, {X: 5, Y: 6}, {X: 7, Y: 8}}
	if err := WriteLineCorners(path, corners, 2); err != nil {
		t.Fatalf("WriteLineCorners: %v", err)
	}

	if err := WriteLineCorners(path, corners, 4); err != nil {
		t.Fatalf("WriteLineCorners: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	if strings.Count(string(got), "goal_side_marker:") != 1 {
		t.Errorf("expected exactly one goal_side_marker key, got:\n%s", got)
	}

	if !strings.Contains(string(got), "goal_side_marker: 4") {
		t.Errorf("goal_side_marker was not updated to the new value:\n%s", got)
	}
}

// Two browser tabs (or a double-click racing the Save button's disabled
// state) can call this concurrently on the same file. instanceConfigMu must
// serialize them -- run under -race, and check the result is one complete
// write, not a torn mix of two.
func TestWriteLineCornersIsSafeForConcurrentCallers(t *testing.T) {
	path := testConfigFile(t)

	var wg sync.WaitGroup

	for i := 1; i <= 8; i++ {
		wg.Add(1)

		go func(marker int) {
			defer wg.Done()

			corners := []Corner{
				{X: marker, Y: marker},
				{X: marker + 1, Y: marker + 1},
				{X: marker + 2, Y: marker + 2},
				{X: marker + 3, Y: marker + 3},
			}
			if err := WriteLineCorners(path, corners, 1); err != nil {
				t.Errorf("WriteLineCorners: %v", err)
			}
		}(i)
	}

	wg.Wait()

	corners, marker, err := ReadLineCorners(path)
	if err != nil {
		t.Fatalf("ReadLineCorners: %v", err)
	}

	if len(corners) != 4 || marker != 1 {
		t.Errorf("got corners=%v marker=%d, want exactly one complete write's worth of data", corners, marker)
	}
}
