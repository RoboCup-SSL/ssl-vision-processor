package config

import (
	"strings"
	"testing"
)

func TestValidName(t *testing.T) {
	for _, name := range []string{"vision.yml", "vision.yaml", "field-b.vision.yml", "my config.yml"} {
		if err := ValidName(name); err != nil {
			t.Errorf("ValidName(%q) = %v, want nil", name, err)
		}
	}

	for _, name := range []string{
		"", ".", "..", "../x.yml", "/etc/x.yml", "a/b.yml", "./x.yml", `a\b.yml`, "x\x00.yml",
		".hidden.yml", ".yml", "x.txt", "x", "x.yml/",
	} {
		if err := ValidName(name); err == nil {
			t.Errorf("ValidName(%q) = nil, want an error", name)
		}
	}
}

func TestAssignConfigPaths(t *testing.T) {
	doc := Document{
		Defaults: Settings{"bot_heights_file": "cam2.yml"},
		Cameras: []Camera{
			{CameraID: 0, ConfigPath: "cam1.yml"}, // moved from slot 1
			{CameraID: 1},
			{CameraID: 2},
			{CameraID: 3},
			{CameraID: 4, ConfigPath: "mine.yml"},
		},
	}

	doc.AssignConfigPaths("cam3.yml")

	got := []string{}
	for _, c := range doc.Cameras {
		got = append(got, c.ConfigPath)
	}

	// Each skips a name another camera, the height table or the config file has.
	want := []string{"cam1.yml", "cam1-2.yml", "cam2-2.yml", "cam3-2.yml", "mine.yml"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("config_paths = %v, want %v", got, want)
	}
}

func TestDuplicateConfigPathIsRejected(t *testing.T) {
	doc := Document{Cameras: []Camera{{CameraID: 0, ConfigPath: "cam0.yml"}, {CameraID: 1, ConfigPath: "cam0.yml"}}}

	err := doc.validateFileNames()
	if err == nil || !strings.Contains(err.Error(), `cameras[1].config_path: "cam0.yml" is also cameras[0]'s`) {
		t.Errorf("validateFileNames = %v, want the duplicate named", err)
	}
}

func TestDeviceKeyFillsInFallbacks(t *testing.T) {
	key := func(block map[string]any) string { return deviceKey("host", block) }

	same := [][2]map[string]any{
		// OPENCV opens /dev/video<id> when no device is named.
		{{"driver": "OPENCV", "id": 1}, {"driver": "OPENCV", "device": "/dev/video1"}},
		{{"driver": "OPENCV"}, {"driver": "OPENCV", "path": "/dev/video0"}},
		// Other drivers' id defaults to 0.
		{{"driver": "SPINNAKER"}, {"driver": "SPINNAKER", "id": 0}},
	}

	for _, pair := range same {
		if a, b := key(pair[0]), key(pair[1]); a == "" || a != b {
			t.Errorf("%v and %v: keys %q and %q, want equal", pair[0], pair[1], a, b)
		}
	}

	if a, b := key(map[string]any{"driver": "SPINNAKER", "id": 1}), key(map[string]any{"driver": "SPINNAKER", "device": "1"}); a == b {
		t.Errorf("a Spinnaker index and serial share key %q", a)
	}

	// No driver: the block doesn't say which device, as for a camera
	// configured on its own machine.
	if k := key(map[string]any{"id": 1}); k != "" {
		t.Errorf("driverless key = %q, want empty", k)
	}
}
