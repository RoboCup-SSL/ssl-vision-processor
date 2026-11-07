package config

import (
	"fmt"
	"os"

	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/defaults"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/geometry"
)

// Default is the document a host with no config starts from: the default
// rulebook field and one camera, 0, with no config_path, marked as a first run.
func Default() (Document, error) {
	data, name, err := defaults.DefaultField()
	if err != nil {
		return Document{}, err
	}

	field, optional, models, err := geometry.ParseFieldFile(data, name)
	if err != nil {
		return Document{}, err
	}

	doc := Document{
		Version:            Version,
		FirstRun:           true,
		Field:              field,
		OptionalFieldLines: optional,
		Cameras:            []Camera{{CameraID: 0}},
	}

	if models != nil {
		m, err := protoMap(models)
		if err != nil {
			return Document{}, fmt.Errorf("models: %w", err)
		}

		doc.Models = m
	}

	if host, err := os.Hostname(); err == nil {
		doc.Cameras[0].Instance = host
	}

	doc.normalize()

	return doc, doc.Validate()
}
