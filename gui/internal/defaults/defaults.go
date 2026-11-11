// Package defaults holds the read-only files the host needs when nothing else
// is on disk, embedded so the binary runs from anywhere: the rulebook fields,
// the league's robot height table, and the README that -readme prints. The
// Makefile copies them here from their sources (config/legacy/,
// robot-heights.yml, gui/README.md), since go:embed can't reach outside this
// package's directory; the test checks the copies match.
package defaults

import (
	"embed"
	"fmt"
)

//go:embed all:data
var files embed.FS

// RobotHeightsFile is what the robot height table is called on disk.
const RobotHeightsFile = "robot-heights.yml"

// Preset is one rulebook field, as a geometry-*.yml file.
type Preset struct {
	Name string
	Data []byte
}

// presets are the rulebook fields, in the order the GUI offers them.
var presets = []struct{ name, file string }{
	{"Division A", "geometry-divA.yml"},
	{"Division B", "geometry-divB.yml"},
}

// defaultField is the field a new config starts from.
const defaultField = "geometry-divB.yml"

// Presets returns every rulebook field.
func Presets() ([]Preset, error) {
	out := make([]Preset, 0, len(presets))

	for _, p := range presets {
		data, err := read(p.file)
		if err != nil {
			return nil, err
		}

		out = append(out, Preset{Name: p.name, Data: data})
	}

	return out, nil
}

// DefaultField returns the geometry-*.yml a new config starts from, and its
// name for error messages.
func DefaultField() ([]byte, string, error) {
	data, err := read(defaultField)

	return data, defaultField, err
}

// RobotHeights returns the league's robot height table.
func RobotHeights() ([]byte, error) {
	return read(RobotHeightsFile)
}

// Readme returns the user guide, gui/README.md.
func Readme() ([]byte, error) {
	return read("README.md")
}

func read(name string) ([]byte, error) {
	data, err := files.ReadFile("data/" + name)
	if err != nil {
		return nil, fmt.Errorf("embedded %s missing, build with make: %w", name, err)
	}

	return data, nil
}
