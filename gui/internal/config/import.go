package config

import (
	"fmt"
	"os"

	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/geometry"
	"gopkg.in/yaml.v3"
)

// perCameraSections are the config.yml sections an import places on the
// camera itself; every other top-level key becomes a shared default.
var perCameraSections = []string{"camera", "geometry", "color"}

// Import builds a document from the pre-vision.yml layout: a geometry file
// (geometry-*.yml) and, optionally, one vision_processor config.yml. With no
// config file the document gets a single camera 0. The imported camera's
// ConfigPath is the config file itself, so the next render regenerates it in
// place.
func Import(geometryPath, configPath string) (Document, error) {
	field, optional, models, err := geometry.LoadFieldFile(geometryPath)
	if err != nil {
		return Document{}, err
	}

	doc := Document{
		Version:            Version,
		Field:              field,
		OptionalFieldLines: optional,
	}

	if models != nil {
		m, err := protoMap(models)
		if err != nil {
			return Document{}, fmt.Errorf("models: %w", err)
		}

		doc.Models = m
	}

	camera := Camera{CameraID: 0}

	if configPath != "" {
		var count int

		camera, doc.Defaults, count, err = importConfig(configPath)
		if err != nil {
			return Document{}, err
		}

		// Keep the legacy split, grown if needed so cam_id still fits.
		for count <= camera.CameraID {
			count = max(1, count*2)
		}

		if count > 1 {
			doc.Layout = &Layout{CameraCount: count}
		}
	}

	if host, err := os.Hostname(); err == nil {
		camera.Instance = host
	}

	doc.Cameras = []Camera{camera}
	doc.normalize()

	return doc, doc.Validate()
}

// importConfig also returns the file's geometry.camera_amount, 0 if unset.
func importConfig(path string) (Camera, map[string]any, int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Camera{}, nil, 0, fmt.Errorf("read %s: %w", path, err)
	}

	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return Camera{}, nil, 0, fmt.Errorf("parse %s: %w", path, err)
	}

	raw, _ = normalize(raw).(map[string]any)
	if raw == nil {
		raw = map[string]any{}
	}

	// A section header with every key under it commented out (how config.yml
	// ships) decodes as null; there's nothing to carry over.
	for k, v := range raw {
		if v == nil {
			delete(raw, k)
		}
	}

	camera := Camera{ConfigPath: path, Config: map[string]any{}}

	if id, ok := raw["cam_id"].(int); ok {
		camera.CameraID = id
	}

	delete(raw, "cam_id")

	count := 0

	if geom, ok := raw["geometry"].(map[string]any); ok {
		seed, err := importSeed(geom)
		if err != nil {
			return Camera{}, nil, 0, fmt.Errorf("%s: %w", path, err)
		}

		camera.Seed = seed

		delete(geom, "line_corners")
		delete(geom, "goal_side_marker")
		count, _ = geom["camera_amount"].(int)
		delete(geom, "camera_amount") // layout.camera_count now
	}

	for _, key := range perCameraSections {
		if block, ok := raw[key].(map[string]any); ok && len(block) > 0 {
			camera.Config[key] = block
		}

		delete(raw, key)
	}

	if len(camera.Config) == 0 {
		camera.Config = nil
	}

	if len(raw) == 0 {
		raw = nil
	}

	return camera, raw, count, nil
}

func importSeed(geom map[string]any) (*Seed, error) {
	list, _ := geom["line_corners"].([]any)
	marker, _ := geom["goal_side_marker"].(int)

	if len(list) == 0 {
		return nil, nil
	}

	seed := &Seed{GoalSideMarker: marker}

	for i, item := range list {
		pair, ok := item.([]any)
		if !ok || len(pair) != 2 {
			return nil, fmt.Errorf("geometry.line_corners[%d]: want [x, y]", i)
		}

		x, xok := pair[0].(int)
		y, yok := pair[1].(int)

		if !xok || !yok {
			return nil, fmt.Errorf("geometry.line_corners[%d]: want integer pixels", i)
		}

		seed.LineCorners = append(seed.LineCorners, [2]int{x, y})
	}

	return seed, nil
}
