package geometry

import (
	"errors"
	"fmt"
	"math"
	"os"
	"regexp"

	"gopkg.in/yaml.v3"
)

// defaultMinReferenceForce is used when config.yml has no min_reference_force
// yet. It's a Go-host-only safety rail (see ColorConfig doc), not a C++
// constant to mirror, so the value is a starting-point choice rather than a
// port of something else -- 5% keeps the reference color from being tuned
// away entirely without blocking deliberate low settings.
const defaultMinReferenceForce = 0.05

// colorReferenceDefaults mirrors src/Resources.cpp's applyTunables fallbacks
// (Resources.cpp:204-209) -- what vision_processor itself uses when a color
// is left commented out in config.yml, as every color ships by default (see
// config-camtest.yml's color: block). ReadColorConfig needs the same
// fallbacks so the panel shows what the instance is actually running, not a
// zeroed-out {0,0,0}.
var colorReferenceDefaults = map[string]RGB{
	"orange": {192, 128, 64},
	"field":  {128, 128, 128},
	"yellow": {255, 128, 0},
	"blue":   {0, 128, 255},
	"green":  {0, 255, 128},
	"pink":   {255, 0, 128},
}

// RGB is one reference color, 0-255 per channel.
type RGB struct {
	R int `json:"r" yaml:"r"`
	G int `json:"g" yaml:"g"`
	B int `json:"b" yaml:"b"`
}

// ColorConfig mirrors config.yml's color: block (see
// proto/proto/vision/ssl_vp_config.proto's SSL_VPConfigColor for the
// upstream schema this is deliberately kept separate from -- MinReferenceForce
// has no protocol equivalent, it's a GUI-only floor on ReferenceForce, kept
// here rather than the protocol so it doesn't need an upstream ssl-protocol-defs
// change to exist).
type ColorConfig struct {
	ReferenceForce    float64 `json:"referenceForce" yaml:"referenceForce"`
	HistoryForce      float64 `json:"historyForce" yaml:"historyForce"`
	MinReferenceForce float64 `json:"minReferenceForce" yaml:"minReferenceForce"`
	Orange            RGB     `json:"orange" yaml:"orange"`
	Field             RGB     `json:"field" yaml:"field"`
	Yellow            RGB     `json:"yellow" yaml:"yellow"`
	Blue              RGB     `json:"blue" yaml:"blue"`
	Green             RGB     `json:"green" yaml:"green"`
	Pink              RGB     `json:"pink" yaml:"pink"`
}

// colorFileConfig is the plain YAML shadow of the color: block. Scalars are
// pointers so a missing key is distinguishable from an explicit 0 -- ReadColorConfig
// applies colorReferenceDefaults/defaultMinReferenceForce itself rather than
// relying on Go zero values, which would silently disagree with what
// vision_processor actually falls back to.
type colorFileConfig struct {
	Color struct {
		ReferenceForce    *float64 `yaml:"reference_force"`
		HistoryForce      *float64 `yaml:"history_force"`
		MinReferenceForce *float64 `yaml:"min_reference_force"`
		Orange            []int    `yaml:"orange"`
		Field             []int    `yaml:"field"`
		Yellow            []int    `yaml:"yellow"`
		Blue              []int    `yaml:"blue"`
		Green             []int    `yaml:"green"`
		Pink              []int    `yaml:"pink"`
	} `yaml:"color"`
}

// Validate reports every out-of-range field at once, same convention as
// FieldConfig.Validate -- a caller using errors.As gets a single
// *ValidationError back (400), not an internal error.
func (c ColorConfig) Validate() error {
	var errs error

	checkForce := func(name string, value float64) {
		if value < 0 || value > 1 {
			errs = errors.Join(errs, fmt.Errorf("%s: must be between 0 and 1, got %v", name, value))
		}
	}

	checkForce("referenceForce", c.ReferenceForce)
	checkForce("historyForce", c.HistoryForce)
	checkForce("minReferenceForce", c.MinReferenceForce)

	if c.ReferenceForce < c.MinReferenceForce {
		errs = errors.Join(errs, fmt.Errorf("referenceForce: must be at least minReferenceForce (%v), got %v", c.MinReferenceForce, c.ReferenceForce))
	}

	if c.ReferenceForce+c.HistoryForce > 1 {
		errs = errors.Join(errs, fmt.Errorf("referenceForce + historyForce: must not exceed 1, got %v", c.ReferenceForce+c.HistoryForce))
	}

	checkColor := func(name string, c RGB) {
		for channel, v := range map[string]int{"red": c.R, "green": c.G, "blue": c.B} {
			if v < 0 || v > 255 {
				errs = errors.Join(errs, fmt.Errorf("%s.%s: must be between 0 and 255, got %d", name, channel, v))
			}
		}
	}

	checkColor("orange", c.Orange)
	checkColor("field", c.Field)
	checkColor("yellow", c.Yellow)
	checkColor("blue", c.Blue)
	checkColor("green", c.Green)
	checkColor("pink", c.Pink)

	if errs != nil {
		return &ValidationError{errs}
	}

	return nil
}

func rgbFromSlice(vals []int, def RGB) RGB {
	if len(vals) != 3 {
		return def
	}

	return RGB{R: vals[0], G: vals[1], B: vals[2]}
}

// ReadColorConfig reads back config.yml's color: block, filling in whatever
// vision_processor's own hardcoded defaults are (colorReferenceDefaults,
// defaultMinReferenceForce) for anything left commented out -- same
// "missing means the normal fallback, not an error" contract as
// ReadLineCorners, but here the fallback is a real color rather than empty.
func ReadColorConfig(path string) (ColorConfig, error) {
	instanceConfigMu.Lock()
	defer instanceConfigMu.Unlock()

	data, err := os.ReadFile(path)
	if err != nil {
		return ColorConfig{}, fmt.Errorf("read %s: %w", path, err)
	}

	var cfg colorFileConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return ColorConfig{}, fmt.Errorf("parse %s: %w", path, err)
	}

	floatOr := func(v *float64, def float64) float64 {
		if v == nil {
			return def
		}

		return *v
	}

	return ColorConfig{
		ReferenceForce:    floatOr(cfg.Color.ReferenceForce, 0.1),
		HistoryForce:      floatOr(cfg.Color.HistoryForce, 0.7),
		MinReferenceForce: floatOr(cfg.Color.MinReferenceForce, defaultMinReferenceForce),
		Orange:            rgbFromSlice(cfg.Color.Orange, colorReferenceDefaults["orange"]),
		Field:             rgbFromSlice(cfg.Color.Field, colorReferenceDefaults["field"]),
		Yellow:            rgbFromSlice(cfg.Color.Yellow, colorReferenceDefaults["yellow"]),
		Blue:              rgbFromSlice(cfg.Color.Blue, colorReferenceDefaults["blue"]),
		Green:             rgbFromSlice(cfg.Color.Green, colorReferenceDefaults["green"]),
		Pink:              rgbFromSlice(cfg.Color.Pink, colorReferenceDefaults["pink"]),
	}, nil
}

// WriteColorConfig writes cfg into config.yml's color: block, in place.
// Every key in that block ships commented out by default (see
// config-camtest.yml), so this uncomments a key the first time it's given a
// value rather than appending a live duplicate under the dead commented
// line -- see spliceColorKey.
func WriteColorConfig(path string, cfg ColorConfig) error {
	if err := cfg.Validate(); err != nil {
		return err
	}

	instanceConfigMu.Lock()
	defer instanceConfigMu.Unlock()

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	updated, err := spliceColorConfig(string(data), cfg)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}

	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}

	return nil
}

// spliceColorConfig is the pure text transform behind WriteColorConfig, split
// out so it's testable without touching disk -- same split as
// spliceLineCorners/spliceGoalSideMarker.
func spliceColorConfig(doc string, cfg ColorConfig) (string, error) {
	lines, trailingNewline := splitDoc(doc)

	start, end, err := sectionBounds(lines, "color")
	if err != nil {
		return "", err
	}

	formatForce := func(v float64) string {
		// Round to avoid writing float noise (e.g. 0.30000000000000004) picked
		// up from triangle-drag arithmetic on the frontend.
		return fmt.Sprintf("%g", math.Round(v*1e6)/1e6)
	}
	formatRGB := func(c RGB) string {
		return fmt.Sprintf("[%d, %d, %d]", c.R, c.G, c.B)
	}

	keys := []struct {
		name  string
		value string
	}{
		{"reference_force", formatForce(cfg.ReferenceForce)},
		{"history_force", formatForce(cfg.HistoryForce)},
		{"min_reference_force", formatForce(cfg.MinReferenceForce)},
		{"orange", formatRGB(cfg.Orange)},
		{"field", formatRGB(cfg.Field)},
		{"yellow", formatRGB(cfg.Yellow)},
		{"blue", formatRGB(cfg.Blue)},
		{"green", formatRGB(cfg.Green)},
		{"pink", formatRGB(cfg.Pink)},
	}

	for _, k := range keys {
		lines, end = spliceColorKey(lines, start, end, k.name, k.value)
	}

	return joinLines(lines, trailingNewline), nil
}

// spliceColorKey sets key's value within the color: section (lines[start:end]),
// uncommenting the line in place if it's currently "#key: ..." (config.yml's
// shipped, unset state), updating it in place if it's already live, or
// appending a fresh "key: value" line at the end of the section if the key
// isn't present at all (e.g. a hand-trimmed config.yml). Returns the
// (possibly-grown) lines slice and the section's new end index, since
// appending shifts it for the next key in the same WriteColorConfig call.
func spliceColorKey(lines []string, start, end int, key, value string) ([]string, int) {
	pattern := regexp.MustCompile(`^(\s*)#?` + regexp.QuoteMeta(key) + `:.*$`)

	for i := start + 1; i < end; i++ {
		if m := pattern.FindStringSubmatch(lines[i]); m != nil {
			lines[i] = fmt.Sprintf("%s%s: %s", m[1], key, value)

			return lines, end
		}
	}

	indent := firstChildIndent(lines[start+1 : end])
	newLine := fmt.Sprintf("%s%s: %s", indent, key, value)

	out := make([]string, 0, len(lines)+1)
	out = append(out, lines[:end]...)
	out = append(out, newLine)
	out = append(out, lines[end:]...)

	return out, end + 1
}
