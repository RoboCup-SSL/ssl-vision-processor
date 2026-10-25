package config

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
)

// Setting types.
const (
	TypeInteger      = "integer"
	TypeNumber       = "number"
	TypeBoolean      = "boolean"
	TypeString       = "string"
	TypeRGB          = "rgb"          // [r, g, b], 0-255 each
	TypeWhiteBalance = "whiteBalance" // "OUTDOOR", "INDOOR", or {red, blue}
	TypePoints       = "points"       // [[x, y], ...]
)

// When a change to a setting takes effect.
const (
	ApplyLive    = "live"    // vision_processor reloads it from config.yml within a second
	ApplyRestart = "restart" // vision_processor reads it only at startup
	ApplyDerived = "derived" // the host writes it from the document; don't set it
	ApplyHost    = "host"    // only the GUI host reads it
)

// Setting is one key of vision_processor's config.yml layout, as the
// defaults block and each camera's config hold it.
type Setting struct {
	Type string `json:"type" enum:"integer,number,boolean,string,rgb,whiteBalance,points"`
	// Default is vision_processor's own fallback for an unset key, nil when
	// there is none (derived) or it depends on other keys (camera.path).
	Default any    `json:"default"`
	Apply   string `json:"apply" enum:"live,restart,derived,host"`
	Unit    string `json:"unit,omitempty"`
	// Bounds the host enforces, inclusive unless Exclusive.
	Minimum          *float64 `json:"minimum,omitempty"`
	Maximum          *float64 `json:"maximum,omitempty"`
	ExclusiveMinimum bool     `json:"exclusiveMinimum,omitempty"`
	Enum             []string `json:"enum,omitempty"`
	// Range is a sensible span for a control, narrower than the bounds. The
	// value itself may be anything within the bounds.
	Range *Range `json:"range,omitempty"`
}

// Range is a control's suggested span and step.
type Range struct {
	Min  float64 `json:"min"`
	Max  float64 `json:"max"`
	Step float64 `json:"step"`
}

// Driver is what a camera driver does with the camera: block
// (src/driver/*.cpp).
type Driver struct {
	ID string `json:"id"`
	// SelectsBy is how the block picks a device: "id", an index in the SDK's
	// camera list, or "path", a device or file path.
	SelectsBy string `json:"selectsBy" enum:"id,path"`
	Gamma     bool   `json:"gamma"`
	// WhiteBalanceProfiles is separate OUTDOOR and INDOOR auto profiles.
	WhiteBalanceProfiles bool `json:"whiteBalanceProfiles"`
}

// Schema describes everything a client needs to edit vision_processor
// settings without reading the C++: served as GET /api/schema.
type Schema struct {
	// Settings by dotted config.yml path, e.g. "thresholds.blobs".
	Settings map[string]Setting `json:"settings"`
	Drivers  []Driver           `json:"drivers"`
	// CameraCounts are the supported layout.cameraCount values.
	CameraCounts []int `json:"cameraCounts"`
}

func bound(v float64) *float64 { return &v }

func rangeOf(lo, hi, step float64) *Range { return &Range{Min: lo, Max: hi, Step: step} }

// settings mirrors src/Resources.cpp and src/driver/cameradriver.cpp: every
// key vision_processor reads, its fallback, and whether applyTunables
// reloads it. Keep it in step with them.
var settings = map[string]Setting{
	"cam_id":           {Type: TypeInteger, Apply: ApplyDerived},
	"bot_heights_file": {Type: TypeString, Default: DefaultBotHeightsFile, Apply: ApplyRestart},

	"camera.driver":        {Type: TypeString, Default: "SPINNAKER", Apply: ApplyRestart, Enum: CameraDrivers},
	"camera.id":            {Type: TypeInteger, Default: 0, Apply: ApplyRestart, Minimum: bound(0)},
	"camera.path":          {Type: TypeString, Apply: ApplyRestart}, // /dev/video{id}
	"camera.width":         {Type: TypeInteger, Default: 0, Apply: ApplyRestart, Unit: "px", Minimum: bound(0)},
	"camera.height":        {Type: TypeInteger, Default: 0, Apply: ApplyRestart, Unit: "px", Minimum: bound(0)},
	"camera.exposure":      {Type: TypeNumber, Default: 0, Apply: ApplyRestart, Unit: "µs", Minimum: bound(0)},
	"camera.gain":          {Type: TypeNumber, Default: 0, Apply: ApplyRestart, Minimum: bound(0)},
	"camera.gamma":         {Type: TypeNumber, Default: 1, Apply: ApplyRestart, Minimum: bound(0), ExclusiveMinimum: true},
	"camera.white_balance": {Type: TypeWhiteBalance, Default: "OUTDOOR", Apply: ApplyRestart, Enum: []string{"OUTDOOR", "INDOOR"}},

	"network.vision_ip":   {Type: TypeString, Default: defaultNetwork.VisionIP, Apply: ApplyRestart},
	"network.vision_port": {Type: TypeInteger, Default: defaultNetwork.VisionPort, Apply: ApplyRestart, Minimum: bound(1), Maximum: bound(65535)},
	"network.gc_ip":       {Type: TypeString, Default: defaultNetwork.GCIP, Apply: ApplyRestart},
	"network.gc_port":     {Type: TypeInteger, Default: defaultNetwork.GCPort, Apply: ApplyRestart, Minimum: bound(1), Maximum: bound(65535)},

	"stream.active":         {Type: TypeBoolean, Default: true, Apply: ApplyRestart},
	"stream.ip_base_prefix": {Type: TypeString, Default: defaultStream.prefix, Apply: ApplyRestart},
	"stream.ip_base_end":    {Type: TypeInteger, Default: defaultStream.end, Apply: ApplyRestart, Minimum: bound(0), Maximum: bound(255)},
	"stream.port":           {Type: TypeInteger, Default: defaultStream.port, Apply: ApplyRestart, Minimum: bound(1), Maximum: bound(65535)},
	"stream.raw_feed":       {Type: TypeBoolean, Default: false, Apply: ApplyRestart},

	"geometry.camera_amount":           {Type: TypeInteger, Apply: ApplyDerived},
	"geometry.line_corners":            {Type: TypePoints, Apply: ApplyDerived},
	"geometry.camera_height":           {Type: TypeNumber, Default: 0, Apply: ApplyRestart, Minimum: bound(0)},
	"geometry.refinement":              {Type: TypeBoolean, Default: true, Apply: ApplyRestart},
	"geometry.field_line_threshold":    {Type: TypeInteger, Default: 5, Apply: ApplyRestart, Minimum: bound(0), Maximum: bound(255), Range: rangeOf(0, 50, 1)},
	"geometry.min_line_segment_length": {Type: TypeNumber, Default: 10, Apply: ApplyRestart, Unit: "px", Minimum: bound(0), Range: rangeOf(0, 50, 1)},
	"geometry.max_line_segment_offset": {Type: TypeNumber, Default: 10, Apply: ApplyRestart, Unit: "px", Minimum: bound(0), Range: rangeOf(0, 50, 1)},
	"geometry.max_line_segment_angle":  {Type: TypeNumber, Default: 3, Apply: ApplyRestart, Unit: "°", Minimum: bound(0), Maximum: bound(90), Range: rangeOf(0, 15, 0.5)},

	"thresholds.circularity":           {Type: TypeNumber, Default: 15, Apply: ApplyLive, Minimum: bound(0), Range: rangeOf(0, 60, 0.5)},
	"thresholds.score":                 {Type: TypeNumber, Default: 5, Apply: ApplyLive, Minimum: bound(0), Range: rangeOf(0, 20, 0.5)},
	"thresholds.min_confidence":        {Type: TypeNumber, Default: 0.2, Apply: ApplyLive, Minimum: bound(0), Maximum: bound(1), Range: rangeOf(0, 1, 0.01)},
	"thresholds.resampling_factor":     {Type: TypeNumber, Default: 1, Apply: ApplyLive, Unit: "×", Minimum: bound(0), ExclusiveMinimum: true, Range: rangeOf(0.25, 2, 0.05)},
	"thresholds.blobs":                 {Type: TypeInteger, Default: 2000, Apply: ApplyRestart, Minimum: bound(1), Range: rangeOf(100, 10000, 100)},
	"thresholds.min_cam_edge_distance": {Type: TypeNumber, Default: 170, Apply: ApplyLive, Unit: "mm", Minimum: bound(0), Range: rangeOf(0, 500, 5)},
	"thresholds.clipping_tolerance":    {Type: TypeNumber, Default: 10, Apply: ApplyLive, Unit: "mm", Minimum: bound(0), Range: rangeOf(0, 50, 1)},
	"thresholds.geometry_tolerance":    {Type: TypeNumber, Default: 10, Apply: ApplyRestart, Unit: "mm", Minimum: bound(0), Range: rangeOf(0, 100, 1)},

	"tracking.min_tracking_radius":  {Type: TypeNumber, Default: 20, Apply: ApplyLive, Unit: "mm", Minimum: bound(0), Range: rangeOf(0, 200, 5)},
	"tracking.max_bot_acceleration": {Type: TypeNumber, Default: 6.5, Apply: ApplyLive, Unit: "m/s²", Minimum: bound(0), Range: rangeOf(0, 20, 0.5)},

	"color.reference_force":     {Type: TypeNumber, Default: 0.1, Apply: ApplyLive, Minimum: bound(0), Maximum: bound(1)},
	"color.history_force":       {Type: TypeNumber, Default: 0.7, Apply: ApplyLive, Minimum: bound(0), Maximum: bound(1)},
	"color.min_reference_force": {Type: TypeNumber, Default: defaultMinReferenceForce, Apply: ApplyHost, Minimum: bound(0), Maximum: bound(1)},
	"color.orange":              {Type: TypeRGB, Default: colorReferenceDefaults["orange"].list(), Apply: ApplyLive},
	"color.field":               {Type: TypeRGB, Default: colorReferenceDefaults["field"].list(), Apply: ApplyLive},
	"color.yellow":              {Type: TypeRGB, Default: colorReferenceDefaults["yellow"].list(), Apply: ApplyLive},
	"color.blue":                {Type: TypeRGB, Default: colorReferenceDefaults["blue"].list(), Apply: ApplyLive},
	"color.green":               {Type: TypeRGB, Default: colorReferenceDefaults["green"].list(), Apply: ApplyLive},
	"color.pink":                {Type: TypeRGB, Default: colorReferenceDefaults["pink"].list(), Apply: ApplyLive},

	"debug.ground_truth":             {Type: TypeString, Default: "gt.yml", Apply: ApplyRestart},
	"debug.wait_for_geometry":        {Type: TypeBoolean, Default: false, Apply: ApplyRestart},
	"debug.debug_images":             {Type: TypeBoolean, Default: false, Apply: ApplyLive},
	"debug.debug_stream_interval_ms": {Type: TypeInteger, Default: 0, Apply: ApplyLive, Unit: "ms", Minimum: bound(0)},
}

var drivers = []Driver{
	{ID: "SPINNAKER", SelectsBy: "id", Gamma: true, WhiteBalanceProfiles: true},
	{ID: "MVIMPACT", SelectsBy: "id"},
	{ID: "OPENCV", SelectsBy: "path", Gamma: true},
}

// CurrentSchema is the schema GET /api/schema serves.
func CurrentSchema() Schema {
	return Schema{Settings: settings, Drivers: drivers, CameraCounts: CameraCounts}
}

func (c RGB) list() []int { return []int{c.R, c.G, c.B} }

// genericBlocks are validated straight from the schema. camera, network and
// color have their own validators, which also check how keys relate (both
// width and height, reference_force at least min_reference_force).
var genericBlocks = []string{"thresholds", "tracking", "geometry", "stream", "debug"}

// validateSettings checks the type and bounds of every scalar setting in
// genericBlocks of an effective config.
func validateSettings(effective map[string]any) error {
	var errs error

	for path, s := range settings {
		block, key, _ := strings.Cut(path, ".")
		if !slices.Contains(genericBlocks, block) || s.Apply == ApplyDerived {
			continue
		}

		values, _ := effective[block].(map[string]any)

		value, ok := values[key]
		if !ok || value == nil {
			continue
		}

		if err := s.check(value); err != nil {
			errs = errors.Join(errs, fmt.Errorf("%s: %w", path, err))
		}
	}

	return errs
}

// check reports why value isn't a valid s, or nil. Values have been through
// normalize, so numbers are int or float64.
func (s Setting) check(value any) error {
	switch s.Type {
	case TypeBoolean:
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("want true or false, got %v", value)
		}
	case TypeString:
		if _, ok := value.(string); !ok {
			return fmt.Errorf("want a string, got %v", value)
		}
	case TypeInteger, TypeNumber:
		var n float64

		switch v := value.(type) {
		case int:
			n = float64(v)
		case float64:
			if s.Type == TypeInteger {
				return fmt.Errorf("want a whole number, got %g", v)
			}

			n = v
		default:
			return fmt.Errorf("want a number, got %v", value)
		}

		if s.Minimum != nil && (n < *s.Minimum || s.ExclusiveMinimum && n == *s.Minimum) {
			if s.ExclusiveMinimum {
				return fmt.Errorf("must be above %g, got %g", *s.Minimum, n)
			}

			return fmt.Errorf("must be at least %g, got %g", *s.Minimum, n)
		}

		if s.Maximum != nil && n > *s.Maximum {
			return fmt.Errorf("must be at most %g, got %g", *s.Maximum, n)
		}

		if math.IsNaN(n) || math.IsInf(n, 0) {
			return fmt.Errorf("want a finite number, got %g", n)
		}
	}

	return nil
}
