package config

import (
	"errors"
	"fmt"
	"slices"

	"gopkg.in/yaml.v3"
)

// CameraDrivers are vision_processor's camera drivers (src/driver). Which
// ones a given build has depends on the SDKs it was compiled against.
var CameraDrivers = []string{"SPINNAKER", "MVIMPACT", "DC1394", "OPENCV"}

// cameraBlock is the YAML shape of a camera: block, as
// src/driver/cameradriver.cpp reads it.
type cameraBlock struct {
	Driver       *string   `yaml:"driver"`
	ID           *int      `yaml:"id"`
	Device       *string   `yaml:"device"`
	Path         *string   `yaml:"path"` // former name of device
	Width        *int      `yaml:"width"`
	Height       *int      `yaml:"height"`
	Exposure     *float64  `yaml:"exposure"`
	Gain         *float64  `yaml:"gain"`
	Gamma        *float64  `yaml:"gamma"`
	WhiteBalance yaml.Node `yaml:"white_balance"`
}

// deviceKey identifies the physical camera a camera block opens on host, or
// is empty when the block doesn't say which driver. Two cameras with the same
// key would fight over one device.
func deviceKey(host string, block any) string {
	driver, device, ok := captureDevice(block, "")
	if !ok {
		return ""
	}

	return fmt.Sprintf("%s|%s|%s", host, driver, device)
}

// CaptureDevice is the driver and device camera c's vision_processor opens,
// with vision_processor's fallbacks filled in: see captureDevice.
func (d Document) CaptureDevice(c Camera) (driver, device string) {
	driver, device, _ = captureDevice(d.Effective(c)["camera"], settings["camera.driver"].Default.(string))

	return driver, device
}

// captureDevice is the driver and device a camera block opens, as
// src/driver/cameradriver.cpp picks them: device (or its former name path),
// else /dev/video<id> for OPENCV, else "id <id>", the driver's id-th camera.
// A block naming no driver uses defaultDriver; ok is false when that's empty
// too.
func captureDevice(block any, defaultDriver string) (driver, device string, ok bool) {
	var c cameraBlock

	if block != nil {
		data, err := yaml.Marshal(block)
		if err != nil || yaml.Unmarshal(data, &c) != nil {
			return "", "", false
		}
	}

	driver = defaultDriver
	if c.Driver != nil {
		driver = *c.Driver
	}

	if driver == "" {
		return "", "", false
	}

	id := 0
	if c.ID != nil {
		id = *c.ID
	}

	named := c.Device
	if named == nil {
		named = c.Path
	}

	switch {
	case named != nil && *named != "":
		return driver, *named, true
	case driver == "OPENCV":
		return driver, fmt.Sprintf("/dev/video%d", id), true
	}

	return driver, fmt.Sprintf("id %d", id), true
}

// validateCamera checks a camera block for what vision_processor would
// misread rather than reject: an unknown driver is fatal at startup, and any
// white_balance string other than OUTDOOR silently means INDOOR.
func validateCamera(block any) error {
	if block == nil {
		return nil
	}

	data, err := yaml.Marshal(block)
	if err != nil {
		return err
	}

	var c cameraBlock
	if err := yaml.Unmarshal(data, &c); err != nil {
		return fmt.Errorf("camera: %w", err)
	}

	var errs error

	add := func(format string, args ...any) { errs = errors.Join(errs, fmt.Errorf(format, args...)) }

	if c.Driver != nil && !slices.Contains(CameraDrivers, *c.Driver) {
		add("camera.driver: want one of %v, got %q", CameraDrivers, *c.Driver)
	}

	if c.ID != nil && *c.ID < 0 {
		add("camera.id: must not be negative")
	}

	width, height := 0, 0
	if c.Width != nil {
		width = *c.Width
	}

	if c.Height != nil {
		height = *c.Height
	}

	if width < 0 || height < 0 {
		add("camera.width/height: must not be negative")
	} else if (width == 0) != (height == 0) {
		add("camera.width/height: set both, or neither for the camera's maximum")
	}

	for name, v := range map[string]*float64{"exposure": c.Exposure, "gain": c.Gain} {
		if v != nil && *v < 0 {
			add("camera.%s: must not be negative (0 is automatic)", name)
		}
	}

	if c.Gamma != nil && *c.Gamma <= 0 {
		add("camera.gamma: must be positive (1.0 is off)")
	}

	switch wb := c.WhiteBalance; wb.Kind {
	case 0: // unset: OUTDOOR
	case yaml.ScalarNode:
		if wb.Value != "OUTDOOR" && wb.Value != "INDOOR" {
			add("camera.white_balance: want OUTDOOR, INDOOR, or {red, blue}, got %q", wb.Value)
		}
	case yaml.MappingNode:
		var manual struct {
			Red  *float64 `yaml:"red"`
			Blue *float64 `yaml:"blue"`
		}

		if err := wb.Decode(&manual); err != nil {
			add("camera.white_balance: %v", err)
		} else if (manual.Red != nil && *manual.Red < 0) || (manual.Blue != nil && *manual.Blue < 0) {
			add("camera.white_balance: red and blue must not be negative")
		}
	default:
		add("camera.white_balance: want OUTDOOR, INDOOR, or {red, blue}")
	}

	return errs
}
