package config

import (
	"fmt"
	"os"
	"slices"

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

// AssignLocal readies d for a host that runs every camera's vision_processor:
// a camera in every region, each run on this host from a config file of its
// own.
func (d *Document) AssignLocal(ownFile string) {
	d.FillRegions()
	d.AssignInstances()
	d.AssignConfigPaths(ownFile)
}

// FillRegions adds a camera for every region below the camera count that has
// none. Each opens its driver's camera_id-th device, not the driver's default
// first one, so that a host's cameras don't all open the same device.
func (d *Document) FillRegions() {
	added := false

	for id := range d.CameraCount() {
		if d.camera(id) == nil {
			d.Cameras = append(d.Cameras, Camera{CameraID: id, Config: Settings{"camera": map[string]any{"id": id}}})
			added = true
		}
	}

	if added {
		slices.SortFunc(d.Cameras, func(a, b Camera) int { return a.CameraID - b.CameraID })
	}
}

// AssignInstances names this host as the instance of every camera that names
// none.
func (d *Document) AssignInstances() {
	host, err := os.Hostname()
	if err != nil {
		return
	}

	for i := range d.Cameras {
		if d.Cameras[i].Instance == "" {
			d.Cameras[i].Instance = host
		}
	}
}
