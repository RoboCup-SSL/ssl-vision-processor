package config

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
)

// Section names which GUI tab a change belongs to.
const (
	SectionField    = "field"
	SectionGeometry = "geometry"
	SectionColor    = "color"
	SectionNetwork  = "network"
	SectionOther    = "other"
)

// Change is one leaf-level difference between two documents. Before or
// After is nil when the value was added or removed. Lists (e.g.
// line_corners) are compared as a whole, not element by element.
type Change struct {
	Path     string `json:"path"`
	Before   any    `json:"before"`
	After    any    `json:"after"`
	Section  string `json:"section"`
	CameraID *int   `json:"cameraId,omitempty"`
}

// Diff lists every difference from before to after, ordered by path.
// Cameras are matched by camera_id, so reordering the list is not a change.
func Diff(before, after Document) []Change {
	var changes []Change

	diffValue(nil, keyCameras(before.generic()), keyCameras(after.generic()), &changes)

	return changes
}

// keyCameras replaces the cameras list with a map keyed "cameras[<id>]"
// segments, so cameras diff by identity rather than list position.
func keyCameras(doc map[string]any) map[string]any {
	list, _ := doc["cameras"].([]any)
	delete(doc, "cameras")

	for _, item := range list {
		cam, ok := item.(map[string]any)
		if !ok {
			continue
		}

		id := cam["camera_id"]
		delete(cam, "camera_id")
		doc[fmt.Sprintf("cameras[%v]", id)] = cam
	}

	return doc
}

// diffValue recurses only while both sides are maps. A map present on just
// one side (a camera added, a calibration locked) is one change holding the
// whole block, not one change per leaf.
func diffValue(path []string, before, after any, changes *[]Change) {
	beforeMap, beforeIsMap := before.(map[string]any)
	afterMap, afterIsMap := after.(map[string]any)

	if !beforeIsMap || !afterIsMap {
		if !reflect.DeepEqual(before, after) {
			*changes = append(*changes, newChange(path, before, after))
		}

		return
	}

	keys := make([]string, 0, len(beforeMap)+len(afterMap))
	for k := range beforeMap {
		keys = append(keys, k)
	}

	for k := range afterMap {
		if _, ok := beforeMap[k]; !ok {
			keys = append(keys, k)
		}
	}

	slices.Sort(keys)

	for _, k := range keys {
		diffValue(append(slices.Clip(path), k), beforeMap[k], afterMap[k], changes)
	}
}

func newChange(path []string, before, after any) Change {
	section, cam := classify(path)

	return Change{
		Path:     strings.Join(path, "."),
		Before:   before,
		After:    after,
		Section:  section,
		CameraID: cam,
	}
}

// classify maps a change's path to the GUI tab it belongs to and, for
// per-camera settings, which camera.
func classify(path []string) (string, *int) {
	if len(path) == 0 {
		return SectionOther, nil
	}

	segment := func(i int) string {
		if i < len(path) {
			return path[i]
		}

		return ""
	}

	switch first := path[0]; {
	case first == "field" || first == "optional_field_lines" || first == "models":
		return SectionField, nil
	case first == "host":
		return SectionNetwork, nil
	case first == "defaults":
		switch segment(1) {
		case "color":
			return SectionColor, nil
		case "network":
			return SectionNetwork, nil
		}

		return SectionOther, nil
	case strings.HasPrefix(first, "cameras["):
		var id int
		if _, err := fmt.Sscanf(first, "cameras[%d]", &id); err != nil {
			return SectionOther, nil
		}

		switch segment(1) {
		case "config":
			switch segment(2) {
			case "color":
				return SectionColor, &id
			case "network":
				return SectionNetwork, &id
			case "camera", "geometry", "":
				return SectionGeometry, &id
			default:
				return SectionOther, &id
			}
		default:
			return SectionGeometry, &id
		}
	default:
		return SectionOther, nil
	}
}
