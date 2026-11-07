package main

import (
	"path/filepath"
	"testing"
)

func TestResolvePathsDefaultsToTheUsersDirectories(t *testing.T) {
	t.Setenv("HOME", "/home/someone")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_STATE_HOME", "")

	p, err := resolvePaths(pathFlags{configFile: "vision.yml"})
	if err != nil {
		t.Fatal(err)
	}

	want := paths{
		configDir:  "/home/someone/.config/rcssl/vision-processor-gui",
		configFile: "vision.yml",
		logFile:    "/home/someone/.local/state/rcssl/vision-processor-gui/logs/vision-processor-gui.log",
		imgDir:     "/home/someone/.local/state/rcssl/vision-processor-gui/img",
	}
	if p != want {
		t.Errorf("paths = %+v, want %+v", p, want)
	}

	if got := p.config(); got != "/home/someone/.config/rcssl/vision-processor-gui/vision.yml" {
		t.Errorf("config() = %q", got)
	}
}

func TestResolvePathsFollowsXDG(t *testing.T) {
	t.Setenv("HOME", "/home/someone")
	t.Setenv("XDG_CONFIG_HOME", "/xdg/config")
	t.Setenv("XDG_STATE_HOME", "/xdg/state")

	p, err := resolvePaths(pathFlags{configFile: "vision.yml"})
	if err != nil {
		t.Fatal(err)
	}

	if p.configDir != "/xdg/config/rcssl/vision-processor-gui" {
		t.Errorf("configDir = %q", p.configDir)
	}

	if p.imgDir != "/xdg/state/rcssl/vision-processor-gui/img" {
		t.Errorf("imgDir = %q", p.imgDir)
	}
}

// With no home there's nowhere safe to default to, and writing somewhere
// surprising is worse than asking.
func TestResolvePathsWithoutAHomeAsksForTheFlags(t *testing.T) {
	t.Setenv("HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_STATE_HOME", "")

	if _, err := resolvePaths(pathFlags{configFile: "vision.yml"}); err == nil {
		t.Error("no -configDir and no home: want an error")
	}

	if _, err := resolvePaths(pathFlags{configDir: "/c", configFile: "vision.yml"}); err == nil {
		t.Error("no -logFile and no home: want an error")
	}

	none, img := "", "/i"
	if _, err := resolvePaths(pathFlags{configDir: "/c", configFile: "vision.yml", logFile: &none, imgDir: &img}); err != nil {
		t.Errorf("every path given: %v", err)
	}
}

func TestResolvePathsTellsAnEmptyLogFileFromAnUnsetOne(t *testing.T) {
	t.Setenv("HOME", "/home/someone")

	empty := ""

	p, err := resolvePaths(pathFlags{configFile: "vision.yml", logFile: &empty})
	if err != nil {
		t.Fatal(err)
	}

	if p.logFile != "" {
		t.Errorf("logFile = %q, want disabled", p.logFile)
	}
}

func TestResolvePathsMakesRelativeFlagsAbsolute(t *testing.T) {
	logFile, img := "logs/x.log", "img"

	p, err := resolvePaths(pathFlags{configDir: ".", configFile: "vision.yml", logFile: &logFile, imgDir: &img})
	if err != nil {
		t.Fatal(err)
	}

	for name, path := range map[string]string{"configDir": p.configDir, "logFile": p.logFile, "imgDir": p.imgDir} {
		if !filepath.IsAbs(path) {
			t.Errorf("%s = %q, want absolute", name, path)
		}
	}
}

func TestResolvePathsRefusesAPathAsTheConfigFile(t *testing.T) {
	t.Setenv("HOME", "/home/someone")

	for _, name := range []string{"sub/vision.yml", "/etc/vision.yml", "../vision.yml", "vision.txt"} {
		if _, err := resolvePaths(pathFlags{configFile: name}); err == nil {
			t.Errorf("-configFile %q: want an error", name)
		}
	}
}

// A supervised vision_processor writes img/ in the config folder, so that's
// where the host has to look.
func TestResolvePathsFindsSupervisedSnapshotsInTheConfigFolder(t *testing.T) {
	t.Setenv("HOME", "/home/someone")

	p, err := resolvePaths(pathFlags{configDir: "/c", configFile: "vision.yml", supervising: true})
	if err != nil {
		t.Fatal(err)
	}

	if p.imgDir != "/c/img" {
		t.Errorf("imgDir = %q, want /c/img", p.imgDir)
	}
}
