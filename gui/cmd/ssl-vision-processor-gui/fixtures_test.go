package main

// The test fixtures, shared with the packages that own them rather than
// copied here.
const (
	// visionFixture is a vision.yml: two cameras, camera 0 with a locked
	// calibration.
	visionFixture = "../../internal/config/testdata/vision.yml"
	// geometryFixture is a legacy geometry.yml.
	geometryFixture = "../../internal/geometry/testdata/geometry.yml"
	// configFixture is a legacy vision_processor config.yml.
	configFixture = "../../internal/geometry/testdata/config.yml"
)
