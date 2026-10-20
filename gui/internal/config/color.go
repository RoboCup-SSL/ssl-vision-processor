package config

import (
	"errors"
	"fmt"

	"gopkg.in/yaml.v3"
)

// defaultMinReferenceForce is used when a color block has no
// min_reference_force. It's a Go-host-only safety rail (see ColorConfig), not
// a C++ constant to mirror -- 5% keeps the reference color from being tuned
// away entirely without blocking deliberate low settings.
const defaultMinReferenceForce = 0.05

// colorReferenceDefaults mirrors src/Resources.cpp's applyTunables fallbacks
// -- what vision_processor itself uses for a color left unset.
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
	R int
	G int
	B int
}

// ColorConfig is a camera's effective color: block, with vision_processor's
// own fallbacks filled in for anything unset. MinReferenceForce has no
// protocol equivalent: it's a GUI-only floor on ReferenceForce, stored as
// min_reference_force in the color block (yaml-cpp ignores unknown keys).
type ColorConfig struct {
	ReferenceForce    float64
	HistoryForce      float64
	MinReferenceForce float64
	Orange            RGB
	Field             RGB
	Yellow            RGB
	Blue              RGB
	Green             RGB
	Pink              RGB
}

// colorBlock is the YAML shape of a color: block. Scalars are pointers so a
// missing key falls back to vision_processor's default rather than Go's zero
// value, which would silently disagree with what the instance runs.
type colorBlock struct {
	ReferenceForce    *float64 `yaml:"reference_force"`
	HistoryForce      *float64 `yaml:"history_force"`
	MinReferenceForce *float64 `yaml:"min_reference_force"`
	Orange            []int    `yaml:"orange"`
	Field             []int    `yaml:"field"`
	Yellow            []int    `yaml:"yellow"`
	Blue              []int    `yaml:"blue"`
	Green             []int    `yaml:"green"`
	Pink              []int    `yaml:"pink"`
}

// colorFromBlock decodes a config.yml-layout color block (generic YAML values,
// possibly nil) into a ColorConfig with fallbacks applied.
func colorFromBlock(block any) (ColorConfig, error) {
	var parsed colorBlock

	if block != nil {
		data, err := yaml.Marshal(block)
		if err != nil {
			return ColorConfig{}, err
		}

		if err := yaml.Unmarshal(data, &parsed); err != nil {
			return ColorConfig{}, &ValidationError{fmt.Errorf("color: %w", err)}
		}
	}

	floatOr := func(v *float64, def float64) float64 {
		if v == nil {
			return def
		}

		return *v
	}

	rgb := func(name string, vals []int) (RGB, error) {
		switch len(vals) {
		case 0:
			return colorReferenceDefaults[name], nil
		case 3:
			return RGB{R: vals[0], G: vals[1], B: vals[2]}, nil
		default:
			return RGB{}, fmt.Errorf("color.%s: want [r, g, b], got %d values", name, len(vals))
		}
	}

	var errs error

	pick := func(name string, vals []int) RGB {
		c, err := rgb(name, vals)
		errs = errors.Join(errs, err)

		return c
	}

	cfg := ColorConfig{
		ReferenceForce:    floatOr(parsed.ReferenceForce, 0.1),
		HistoryForce:      floatOr(parsed.HistoryForce, 0.7),
		MinReferenceForce: floatOr(parsed.MinReferenceForce, defaultMinReferenceForce),
		Orange:            pick("orange", parsed.Orange),
		Field:             pick("field", parsed.Field),
		Yellow:            pick("yellow", parsed.Yellow),
		Blue:              pick("blue", parsed.Blue),
		Green:             pick("green", parsed.Green),
		Pink:              pick("pink", parsed.Pink),
	}

	if errs != nil {
		return ColorConfig{}, &ValidationError{errs}
	}

	return cfg, nil
}

// Validate reports every out-of-range value at once.
func (c ColorConfig) Validate() error {
	var errs error

	checkForce := func(name string, value float64) {
		if value < 0 || value > 1 {
			errs = errors.Join(errs, fmt.Errorf("color.%s: must be between 0 and 1, got %v", name, value))
		}
	}

	checkForce("reference_force", c.ReferenceForce)
	checkForce("history_force", c.HistoryForce)
	checkForce("min_reference_force", c.MinReferenceForce)

	if c.ReferenceForce < c.MinReferenceForce {
		errs = errors.Join(errs, fmt.Errorf("color.reference_force: must be at least min_reference_force (%v), got %v", c.MinReferenceForce, c.ReferenceForce))
	}

	if c.ReferenceForce+c.HistoryForce > 1 {
		errs = errors.Join(errs, fmt.Errorf("color.reference_force + history_force: must not exceed 1, got %v", c.ReferenceForce+c.HistoryForce))
	}

	checkColor := func(name string, c RGB) {
		for channel, v := range map[string]int{"red": c.R, "green": c.G, "blue": c.B} {
			if v < 0 || v > 255 {
				errs = errors.Join(errs, fmt.Errorf("color.%s.%s: must be between 0 and 255, got %d", name, channel, v))
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
