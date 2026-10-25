package config

import (
	"slices"
	"testing"
)

func TestRestartKeys(t *testing.T) {
	doc := Document{
		Version:  Version,
		Defaults: Settings{"thresholds": map[string]any{"score": 5, "blobs": 2000}},
		Cameras:  []Camera{{CameraID: 0, Config: Settings{"camera": map[string]any{"exposure": 5.0}}}},
	}
	doc.normalize()
	base := doc.Rendered(doc.Cameras[0])

	for name, tc := range map[string]struct {
		edit func(*Document)
		want []string
	}{
		"unchanged": {func(*Document) {}, nil},
		"live only": {func(d *Document) {
			d.Defaults["thresholds"] = map[string]any{"score": 6, "blobs": 2000}
			d.Defaults["color"] = map[string]any{"min_reference_force": 0.2}
		}, nil},
		"startup setting": {func(d *Document) {
			d.Cameras[0].Config["camera"] = map[string]any{"exposure": 6.0}
		}, []string{"camera.exposure"}},
		"derived": {func(d *Document) {
			d.Cameras[0].CameraID = 1
			d.Layout = &Layout{CameraCount: 2}
		}, []string{"cam_id", "geometry.camera_amount"}},
		"unknown key": {func(d *Document) {
			d.Defaults["thresholds"] = map[string]any{"score": 5, "blobs": 2000, "made_up": 1}
		}, []string{"thresholds.made_up"}},
		"removed startup setting": {func(d *Document) {
			d.Defaults["thresholds"] = map[string]any{"score": 5}
		}, []string{"thresholds.blobs"}},
	} {
		t.Run(name, func(t *testing.T) {
			next := doc.clone()
			tc.edit(&next)

			if got := RestartKeys(base, next.Rendered(next.Cameras[0])); !slices.Equal(got, tc.want) {
				t.Errorf("RestartKeys = %v, want %v", got, tc.want)
			}
		})
	}
}
