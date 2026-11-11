package config

import "math"

// camera.exposure is microseconds in vision.yml, whatever the driver.
// config.yml's unit depends on the driver (src/driver/*.cpp): SPINNAKER and
// MVIMPACT read milliseconds, and OPENCV sends the value × 1000 to V4L2's
// exposure_time_absolute, which counts 100 µs steps. Render and Import
// convert; nothing else sees config.yml's unit.

// microsPerFileUnit is how many µs one config.yml exposure unit is.
func microsPerFileUnit(driver string) float64 {
	if driver == "OPENCV" {
		return 100_000
	}

	return 1000
}

// blockDriver is a camera: block's driver, vision_processor's SPINNAKER when
// unset.
func blockDriver(block map[string]any) string {
	if d, ok := block["driver"].(string); ok && d != "" {
		return d
	}

	return "SPINNAKER"
}

// convertExposure multiplies a camera: block's exposure by factor in place,
// if it has a number there. 0 (automatic) stays 0.
func convertExposure(block map[string]any, factor float64) {
	v, ok := number(block["exposure"])
	if !ok {
		return
	}

	block["exposure"] = normalize(math.Round(v*factor*1e6) / 1e6)
}

// exposureToFile converts a rendered config's exposure from µs to config.yml's
// unit for its driver.
func exposureToFile(rendered map[string]any) {
	if cam, ok := rendered["camera"].(map[string]any); ok {
		convertExposure(cam, 1/microsPerFileUnit(blockDriver(cam)))
	}
}

// migrateV1 converts a version 1 document, whose exposures are config.yml's
// units, to µs. A camera that inherits its exposure from defaults but runs a
// different driver gets the value converted for its own driver, so nothing
// it runs with changes.
func (d *Document) migrateV1() {
	defaultsCam, _ := d.Defaults["camera"].(map[string]any)
	defaultsDriver := blockDriver(defaultsCam)
	inherited, hasInherited := number(defaultsCam["exposure"])

	for i := range d.Cameras {
		c := &d.Cameras[i]
		own, _ := c.Config["camera"].(map[string]any)

		driver := defaultsDriver
		if ownDriver, ok := own["driver"].(string); ok && ownDriver != "" {
			driver = ownDriver
		}

		if _, ok := own["exposure"]; ok {
			convertExposure(own, microsPerFileUnit(driver))
		} else if hasInherited && driver != defaultsDriver {
			if own == nil {
				own = map[string]any{}

				if c.Config == nil {
					c.Config = Settings{}
				}

				c.Config["camera"] = own
			}

			own["exposure"] = inherited
			convertExposure(own, microsPerFileUnit(driver))
		}
	}

	if defaultsCam != nil {
		convertExposure(defaultsCam, microsPerFileUnit(defaultsDriver))
	}

	d.Version = 2
}

func number(v any) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case float64:
		return n, true
	}

	return 0, false
}
