package geometry

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/vision"
	"google.golang.org/protobuf/encoding/protojson"
	"gopkg.in/yaml.v3"
)

type optionalLines struct {
	Goal2Goal    *bool `yaml:"goal2goal"`
	Halfway      *bool `yaml:"halfway"`
	CenterCircle *bool `yaml:"centercircle"`
	Penalty      *bool `yaml:"penalty"`
}

// Load reads path and returns the wrapper packet, the optional-line toggles
// (not a proto field, so callers wanting to inspect or re-save them need it
// separately), and whatever top-level YAML keys aren't "optional_field_lines"
// or "field" (e.g. "models") so a later save can put them back unchanged.
func Load(path string) (*vision.SSL_WrapperPacket, optionalLines, map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, optionalLines{}, nil, fmt.Errorf("read %s: %w", path, err)
	}

	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, optionalLines{}, nil, fmt.Errorf("parse %s: %w", path, err)
	}

	lines, err := popOptionalLines(raw)
	if err != nil {
		return nil, optionalLines{}, nil, fmt.Errorf("%s: %w", path, err)
	}

	geometry, err := decodeGeometry(raw)
	if err != nil {
		return nil, optionalLines{}, nil, fmt.Errorf("%s: %w", path, err)
	}

	delete(raw, "field")

	wrapper := &vision.SSL_WrapperPacket{
		Geometry: geometry,
		Source:   vision.SSL_Source_SSL_SOURCE_VISION_PROCESSOR.Enum(),
	}

	generateFieldMarkings(wrapper.GetGeometry().GetField(), lines)

	return wrapper, lines, raw, nil
}

func popOptionalLines(raw map[string]any) (optionalLines, error) {
	block, ok := raw["optional_field_lines"]
	if !ok {
		return optionalLines{}, errors.New("optional_field_lines: required block is missing")
	}

	delete(raw, "optional_field_lines")

	return decodeOptionalLines(block)
}

func decodeOptionalLines(block any) (optionalLines, error) {
	data, err := yaml.Marshal(block)
	if err != nil {
		return optionalLines{}, fmt.Errorf("optional_field_lines: %w", err)
	}

	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)

	var lines optionalLines
	if err := decoder.Decode(&lines); err != nil {
		return optionalLines{}, fmt.Errorf("optional_field_lines: %w", err)
	}

	return lines, lines.validate()
}

func (o optionalLines) validate() error {
	var errs error

	for key, value := range map[string]*bool{
		"goal2goal":    o.Goal2Goal,
		"halfway":      o.Halfway,
		"centercircle": o.CenterCircle,
		"penalty":      o.Penalty,
	} {
		if value == nil {
			errs = errors.Join(errs, fmt.Errorf("optional_field_lines.%s: required", key))
		}
	}

	return errs
}

func decodeGeometry(raw map[string]any) (*vision.SSL_GeometryData, error) {
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("geometry: %w", err)
	}

	geometry := &vision.SSL_GeometryData{}
	if err := protojson.Unmarshal(data, geometry); err != nil {
		return nil, fmt.Errorf("geometry: %w", err)
	}

	return geometry, nil
}
