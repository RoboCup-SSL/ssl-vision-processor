/*
     Copyright 2024 Felix Weinmann

     Licensed under the Apache License, Version 2.0 (the "License");
     you may not use this file except in compliance with the License.
     You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

     Unless required by applicable law or agreed to in writing, software
     distributed under the License is distributed on an "AS IS" BASIS,
     WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
     See the License for the specific language governing permissions and
     limitations under the License.
 */
#include "cameradriver.h"
#include "config.h"

#include "driver/dummydriver.h"
#include "driver/opencvdriver.h"
#include <memory>
#ifdef SPINNAKER_AVAILABLE
#include "driver/spinnakerdriver.h"
#endif
#ifdef MVIMPACT_AVAILABLE
#include "driver/mvimpactdriver.h"
#endif
#ifdef DC1394_AVAILABLE
#include "driver/dc1394driver.h"
#endif


double realTimeOffset = 0.0;

double getRealTime() {
	return (double)std::chrono::duration_cast<std::chrono::microseconds>(std::chrono::high_resolution_clock::now().time_since_epoch()).count() / 1e6 + realTimeOffset;
}

double CameraDriver::getTime() {
	return getRealTime();
}

std::unique_ptr<CameraDriver> camera;

void init_camera() {
	camera = std::make_unique<DummyDriver>();
}

void update_camera_config(const SSL_VPConfigCamera* camera_config) {
	bool newInstance = false;

	SSL_VPConfigCameraDriver driver = (camera_config->has_driver() ? *camera_config : config.camera()).driver();
	uint32_t id = (camera_config->has_id() ? *camera_config : config.camera()).id();
	std::string device = (camera_config->has_path() ? *camera_config : config.camera()).path();
	if(driver != config.camera().driver() || id != config.camera().id() || device != config.camera().path()) {
		newInstance = true;

		switch(camera_config->driver()) {
            case DUMMY:
				camera = std::make_unique<DummyDriver>();
				break;
            case OPENCV:
				if(device.empty())
					device = "/dev/video" + std::to_string(id);

				camera = std::make_unique<OpenCVDriver>(device);
				break;
#ifdef SPINNAKER_AVAILABLE
            case SPINNAKER:
				camera = std::make_unique<SpinnakerDriver>(device, id);
				break;
#endif
#ifdef MVIMPACT_AVAILABLE
            case MVIMPACT:
				camera = std::make_unique<MVImpactDriver>(id);
				break;
#endif
#ifdef DC1394_AVAILABLE
            case DC1394:
				camera = std::make_unique<DC1394Driver>(id);
                break;
#endif
			default:
				WARN("Attempted to configure invalid/unavailable camera driver: " << camera_config->driver());
        }
    }

	uint32_t width = (camera_config->has_width() ? *camera_config : config.camera()).width();
	uint32_t height = (camera_config->has_height() ? *camera_config : config.camera()).height();
	if(newInstance || width != config.camera().width() || height != config.camera().height()) {
		camera->setResolution(width, height);
	}

	float exposure = (camera_config->has_exposure() ? *camera_config : config.camera()).exposure();
	if(newInstance || exposure != config.camera().exposure()) {
		camera->setExposure(exposure);
	}

	float gain = (camera_config->has_gain() ? *camera_config : config.camera()).gain();
	if(newInstance || gain != config.camera().gain()) {
		camera->setGain(gain);
	}

	float gamma = (camera_config->has_gamma() ? *camera_config : config.camera()).gamma();
	if(newInstance || gamma != config.camera().gamma()) {
		camera->setGamma(gamma);
	}

	SSL_VPConfigCameraWBType wbType = (camera_config->has_wb_type() ? *camera_config : config.camera()).wb_type();
	float wbRed = (camera_config->has_wb_red() ? *camera_config : config.camera()).wb_red();
	float wbBlue = (camera_config->has_wb_blue() ? *camera_config : config.camera()).wb_blue();
	if(newInstance || wbType != config.camera().wb_type() || wbRed != config.camera().wb_red() || wbBlue != config.camera().wb_blue()) {
		camera->setWhiteBalance(wbType, wbBlue, wbRed);
	}

	config.mutable_camera()->CopyFrom(*camera_config);
}
