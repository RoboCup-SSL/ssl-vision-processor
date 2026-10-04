// Package config owns vision.yml: the single host-owned file holding the
// field template, shared and per-camera vision_processor settings, and
// explicitly locked camera calibrations. See Store for the live working copy
// and its relationship to the file on disk.
package config

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/geometry"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/vision"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"gopkg.in/yaml.v3"
)

// Version is the vision.yml format version this package reads and writes.
const Version = 1

const fileHeader = `# vision-processor-gui configuration. Edit freely: the GUI watches this file
# and asks before applying or overwriting changes made here. Comments are not
# preserved when the GUI saves.
`

// Document is one vision.yml.
type Document struct {
	Version            int                          `yaml:"version" json:"version"`
	Field              geometry.FieldConfig         `yaml:"field" json:"field"`
	OptionalFieldLines geometry.OptionalLinesConfig `yaml:"optional_field_lines" json:"optionalFieldLines"`
	// Models is SSL_GeometryModels in protojson (proto field names).
	Models map[string]any `yaml:"models,omitempty" json:"models,omitempty"`
	// Defaults uses vision_processor's own config.yml layout (camera:,
	// geometry:, color:, thresholds:, ...) and applies to every camera.
	Defaults map[string]any `yaml:"defaults,omitempty" json:"defaults,omitempty"`
	Cameras  []Camera       `yaml:"cameras" json:"cameras"`
	// Host is settings for this GUI host alone; no vision_processor reads it.
	Host *Host `yaml:"host,omitempty" json:"host,omitempty"`
}

// Host is settings that only mean something on the machine running the GUI
// host, such as its network interface names.
type Host struct {
	Interfaces *Interfaces `yaml:"interfaces,omitempty" json:"interfaces,omitempty"`
}

// Interfaces chooses which network interfaces the host's sockets use.
type Interfaces struct {
	// Auto (the default when unset) lets the host pick. Skip is then kept
	// only so switching back to manual restores it.
	Auto *bool `yaml:"auto,omitempty" json:"auto,omitempty"`
	// Skip names interfaces not to use when choosing manually.
	Skip []string `yaml:"skip,omitempty" json:"skip,omitempty"`
}

// InterfaceSelection is whether the host picks interfaces itself, and which
// ones to skip when it doesn't.
func (d Document) InterfaceSelection() (auto bool, skip []string) {
	if d.Host == nil || d.Host.Interfaces == nil {
		return true, nil
	}

	i := d.Host.Interfaces

	return i.Auto == nil || *i.Auto, i.Skip
}

// Camera is one vision_processor instance, keyed by camera_id (its position
// on the field), not by which machine runs it.
type Camera struct {
	CameraID int    `yaml:"camera_id" json:"cameraId"`
	Instance string `yaml:"instance,omitempty" json:"instance,omitempty"`
	// ConfigPath is a host-local config.yml to regenerate from this document
	// for a vision_processor on the same machine. Empty for remote instances.
	ConfigPath string `yaml:"config_path,omitempty" json:"configPath,omitempty"`
	// Config overrides Defaults, same config.yml layout, deep-merged.
	Config      map[string]any `yaml:"config,omitempty" json:"config,omitempty"`
	Seed        *Seed          `yaml:"seed,omitempty" json:"seed,omitempty"`
	Calibration *Calibration   `yaml:"calibration,omitempty" json:"calibration,omitempty"`
}

// Seed is the human-picked calibration starting point: the four field line
// corners in image pixels, and the image resolution they were picked at.
// Resolution [0, 0] means unknown (e.g. imported from a legacy config.yml).
type Seed struct {
	Resolution  [2]int   `yaml:"resolution,flow" json:"resolution"`
	LineCorners [][2]int `yaml:"line_corners,flow" json:"lineCorners"`
	// GoalSideMarker records which corner-picker marker (1-4) was chosen as
	// the goal-side corner. vision_processor never reads it.
	GoalSideMarker int `yaml:"goal_side_marker,omitempty" json:"goalSideMarker,omitempty"`
}

// Calibration is a locked SSL_GeometryCameraCalibration, published in place
// of whatever the camera's vision_processor last sent.
type Calibration struct {
	LockedAt time.Time `yaml:"locked_at" json:"lockedAt"`
	// FieldHash identifies the field dimensions the calibration was solved
	// against; a mismatch means the field changed since locking.
	FieldHash string `yaml:"field_hash" json:"fieldHash"`
	// Camera is SSL_GeometryCameraCalibration in protojson (proto field names).
	Camera map[string]any `yaml:"camera" json:"camera"`
}

// ValidationError marks a document rejected for its content -- the caller's
// mistake, not ours. Callers use errors.As to answer 400 instead of 500.
type ValidationError struct{ err error }

func (v *ValidationError) Error() string { return v.err.Error() }
func (v *ValidationError) Unwrap() error { return v.err }

// IsValidation reports whether err is a content problem in either this
// package's or internal/geometry's sense.
func IsValidation(err error) bool {
	var ours *ValidationError
	var theirs *geometry.ValidationError

	return errors.As(err, &ours) || errors.As(err, &theirs)
}

// Parse decodes and validates a vision.yml. Unknown keys are rejected so a
// typo in a hand edit is reported rather than silently ignored.
func Parse(data []byte) (Document, error) {
	doc, err := decode(data)
	if err != nil {
		return Document{}, err
	}

	return doc, doc.Validate()
}

func decode(data []byte) (Document, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)

	var doc Document
	if err := decoder.Decode(&doc); err != nil {
		return Document{}, &ValidationError{fmt.Errorf("parse: %w", err)}
	}

	doc.normalize()

	return doc, nil
}

// Marshal encodes doc as vision.yml, header comment included.
func Marshal(doc Document) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString(fileHeader)

	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)

	if err := encoder.Encode(doc); err != nil {
		return nil, err
	}

	if err := encoder.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// Validate reports every problem at once, wrapped in a *ValidationError.
func (d Document) Validate() error {
	var errs error

	add := func(err error) { errs = errors.Join(errs, err) }

	if d.Version != Version {
		add(fmt.Errorf("version: want %d, got %d", Version, d.Version))
	}

	if err := d.Field.Validate(); err != nil {
		add(fmt.Errorf("field: %w", err))
	}

	if _, err := d.ModelsProto(); err != nil {
		add(err)
	}

	if n, err := d.Network(); err != nil {
		add(fmt.Errorf("defaults: %w", err))
	} else if err := n.Validate(); err != nil {
		add(fmt.Errorf("defaults: %w", err))
	}

	seen := map[int]bool{}

	for _, c := range d.Cameras {
		prefix := fmt.Sprintf("cameras[%d]", c.CameraID)

		if c.CameraID < 0 {
			add(fmt.Errorf("%s.camera_id: must not be negative", prefix))
		}

		if seen[c.CameraID] {
			add(fmt.Errorf("%s.camera_id: duplicate", prefix))
		}

		seen[c.CameraID] = true

		if c.Seed != nil {
			add(c.Seed.validate(prefix + ".seed"))
		}

		if c.Calibration != nil {
			p, err := c.Calibration.Proto()
			switch {
			case err != nil:
				add(fmt.Errorf("%s.calibration: %w", prefix, err))
			case int(p.GetCameraId()) != c.CameraID:
				add(fmt.Errorf("%s.calibration.camera.camera_id: want %d, got %d", prefix, c.CameraID, p.GetCameraId()))
			}
		}

		// The shared block is checked above; only a camera's own override can
		// add a new problem here.
		if c.Config["network"] != nil {
			if n, err := networkFromBlock(d.Effective(c)["network"]); err != nil {
				add(fmt.Errorf("%s: %w", prefix, err))
			} else if err := n.Validate(); err != nil {
				add(fmt.Errorf("%s: %w", prefix, err))
			}
		}

		if err := validateCamera(d.Effective(c)["camera"]); err != nil {
			add(fmt.Errorf("%s: %w", prefix, err))
		}

		color, err := colorFromBlock(d.Effective(c)["color"])
		if err != nil {
			add(fmt.Errorf("%s: %w", prefix, err))
		} else if err := color.Validate(); err != nil {
			add(fmt.Errorf("%s: %w", prefix, err))
		}
	}

	if errs != nil {
		return &ValidationError{errs}
	}

	return nil
}

func (s Seed) validate(prefix string) error {
	var errs error

	if s.Resolution[0] < 0 || s.Resolution[1] < 0 {
		errs = errors.Join(errs, fmt.Errorf("%s.resolution: must not be negative", prefix))
	}

	if (s.Resolution[0] == 0) != (s.Resolution[1] == 0) {
		errs = errors.Join(errs, fmt.Errorf("%s.resolution: set both width and height, or neither", prefix))
	}

	if n := len(s.LineCorners); n != 0 && n != 4 {
		errs = errors.Join(errs, fmt.Errorf("%s.line_corners: want 0 or 4 corners, got %d", prefix, n))
	}

	if s.GoalSideMarker < 0 || s.GoalSideMarker > 4 {
		errs = errors.Join(errs, fmt.Errorf("%s.goal_side_marker: want 0-4, got %d", prefix, s.GoalSideMarker))
	}

	return errs
}

// Effective is a camera's config.yml-layout settings: Defaults with the
// camera's own Config deep-merged over it.
func (d Document) Effective(c Camera) map[string]any {
	return deepMerge(d.Defaults, c.Config)
}

// ModelsProto decodes Models, or returns nil if there are none.
func (d Document) ModelsProto() (*vision.SSL_GeometryModels, error) {
	if len(d.Models) == 0 {
		return nil, nil
	}

	data, err := json.Marshal(d.Models)
	if err != nil {
		return nil, err
	}

	models := &vision.SSL_GeometryModels{}
	if err := protojson.Unmarshal(data, models); err != nil {
		return nil, fmt.Errorf("models: %w", err)
	}

	return models, nil
}

// Proto decodes the locked calibration and checks it's complete.
func (c Calibration) Proto() (*vision.SSL_GeometryCameraCalibration, error) {
	data, err := json.Marshal(c.Camera)
	if err != nil {
		return nil, err
	}

	calib := &vision.SSL_GeometryCameraCalibration{}
	if err := protojson.Unmarshal(data, calib); err != nil {
		return nil, fmt.Errorf("camera: %w", err)
	}

	if err := proto.CheckInitialized(calib); err != nil {
		return nil, fmt.Errorf("camera: %w", err)
	}

	return calib, nil
}

// protoMap is msg as protojson with proto field names, decoded to plain
// values -- the form Models and Calibration.Camera are stored in.
func protoMap(msg proto.Message) (map[string]any, error) {
	data, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(msg)
	if err != nil {
		return nil, err
	}

	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}

	normalized, _ := normalize(m).(map[string]any)

	return normalized, nil
}

// fieldHash fingerprints the field dimensions a calibration was solved
// against. Short, since it only needs to notice a change, not resist attack.
func fieldHash(field geometry.FieldConfig) string {
	data, err := yaml.Marshal(field)
	if err != nil {
		return ""
	}

	sum := sha256.Sum256(data)

	return "sha256:" + hex.EncodeToString(sum[:8])
}

// camera returns a pointer to the camera with id, or nil.
func (d *Document) camera(id int) *Camera {
	for i := range d.Cameras {
		if d.Cameras[i].CameraID == id {
			return &d.Cameras[i]
		}
	}

	return nil
}

// clone deep-copies d through its YAML encoding.
func (d Document) clone() Document {
	data, err := yaml.Marshal(d)
	if err != nil {
		panic(fmt.Sprintf("config: marshal for clone: %v", err))
	}

	var out Document
	if err := yaml.Unmarshal(data, &out); err != nil {
		panic(fmt.Sprintf("config: unmarshal for clone: %v", err))
	}

	out.normalize()

	return out
}

// generic is d as plain YAML values -- what Diff compares.
func (d Document) generic() map[string]any {
	data, err := yaml.Marshal(d)
	if err != nil {
		panic(fmt.Sprintf("config: marshal for diff: %v", err))
	}

	var out map[string]any
	if err := yaml.Unmarshal(data, &out); err != nil {
		panic(fmt.Sprintf("config: unmarshal for diff: %v", err))
	}

	normalized, _ := normalize(out).(map[string]any)

	return normalized
}

// normalize canonicalizes every free-form map in d, so a value is equal to
// itself whether it arrived as YAML (ints) or JSON from the browser (all
// float64).
func (d *Document) normalize() {
	d.Models = normalizeMap(d.Models)
	d.Defaults = normalizeMap(d.Defaults)

	for i := range d.Cameras {
		c := &d.Cameras[i]
		c.Config = normalizeMap(c.Config)

		if c.Calibration != nil {
			c.Calibration.Camera = normalizeMap(c.Calibration.Camera)
		}
	}
}

func normalizeMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}

	out, _ := normalize(m).(map[string]any)

	return out
}

// normalize turns integral floats into ints and every number type into int
// or float64, recursively.
func normalize(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			out[k] = normalize(val)
		}

		return out
	case []any:
		out := make([]any, len(t))
		for i, val := range t {
			out[i] = normalize(val)
		}

		return out
	case float64:
		if t == math.Trunc(t) && math.Abs(t) < 1<<53 {
			return int(t)
		}

		return t
	case float32:
		return normalize(float64(t))
	case int64:
		return int(t)
	case uint64:
		return int(t)
	default:
		return v
	}
}

// deepMerge returns base with over merged on top: nested maps merge, anything
// else in over replaces. Neither input is modified.
func deepMerge(base, over map[string]any) map[string]any {
	out := make(map[string]any, len(base)+len(over))

	for k, v := range base {
		out[k] = deepCopy(v)
	}

	for k, v := range over {
		baseMap, baseIsMap := out[k].(map[string]any)
		overMap, overIsMap := v.(map[string]any)

		if baseIsMap && overIsMap {
			out[k] = deepMerge(baseMap, overMap)
		} else {
			out[k] = deepCopy(v)
		}
	}

	return out
}

func deepCopy(v any) any {
	switch t := v.(type) {
	case map[string]any:
		return deepMerge(t, nil)
	case []any:
		out := make([]any, len(t))
		for i, val := range t {
			out[i] = deepCopy(val)
		}

		return out
	default:
		return v
	}
}
