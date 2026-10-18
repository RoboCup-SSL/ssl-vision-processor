package config

import (
	"fmt"
	"strings"
)

// Where an effective setting's value comes from.
const (
	SourceCamera   = "camera"   // the camera's own config
	SourceDefaults = "defaults" // the document's shared defaults
	SourceDerived  = "derived"  // written by the host from the document
	SourceFallback = "fallback" // unset: vision_processor's own fallback
)

// EffectiveSetting is one setting as camera's vision_processor runs it.
type EffectiveSetting struct {
	Value  any    `json:"value"`
	Source string `json:"source"`
	// Known is false for a key the schema doesn't describe: it's written to
	// config.yml, but nothing the host knows of reads it.
	Known bool `json:"known"`
}

// EffectiveConfig is every setting of one camera, resolved.
type EffectiveConfig struct {
	CameraID int `json:"cameraId"`
	// Settings by dotted config.yml path, as in Schema.
	Settings map[string]EffectiveSetting `json:"settings"`
}

// EffectiveConfig resolves camera id's settings the way its vision_processor
// will: its generated config.yml, then vision_processor's fallbacks for
// anything that leaves out. ErrUnknownCamera if there's no such camera.
func (d Document) EffectiveConfig(id int) (EffectiveConfig, error) {
	c := d.camera(id)
	if c == nil {
		return EffectiveConfig{}, ErrUnknownCamera
	}

	merged := d.rendered(*c)
	out := EffectiveConfig{CameraID: id, Settings: map[string]EffectiveSetting{}}

	sourceOf := func(path string) string {
		if s, ok := settings[path]; ok && s.Apply == ApplyDerived {
			return SourceDerived
		}

		if _, ok := lookup(c.Config, path); ok {
			return SourceCamera
		}

		return SourceDefaults
	}

	for path, s := range settings {
		value, ok := lookup(merged, path)

		switch {
		case ok || s.Apply == ApplyDerived:
			out.Settings[path] = EffectiveSetting{Value: value, Source: sourceOf(path), Known: true}
		default:
			out.Settings[path] = EffectiveSetting{Value: s.Default, Source: SourceFallback, Known: true}
		}
	}

	// cameradriver.cpp's path fallback depends on the id in effect.
	if p := out.Settings["camera.path"]; p.Source == SourceFallback {
		p.Value = fmt.Sprintf("/dev/video%v", out.Settings["camera.id"].Value)
		out.Settings["camera.path"] = p
	}

	for key, value := range merged {
		block, isBlock := value.(map[string]any)
		if !isBlock {
			if _, known := settings[key]; !known {
				out.Settings[key] = EffectiveSetting{Value: value, Source: sourceOf(key)}
			}

			continue
		}

		for k, v := range block {
			path := key + "." + k
			if _, known := settings[path]; !known {
				out.Settings[path] = EffectiveSetting{Value: v, Source: sourceOf(path)}
			}
		}
	}

	return out, nil
}

// lookup finds a top-level ("cam_id") or block ("thresholds.blobs") path in
// a config.yml-layout map.
func lookup(m map[string]any, path string) (any, bool) {
	block, key, nested := strings.Cut(path, ".")
	if !nested {
		v, ok := m[path]

		return v, ok
	}

	values, _ := m[block].(map[string]any)
	v, ok := values[key]

	return v, ok
}
