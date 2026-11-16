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
#pragma once

#include <memory>
#include "opencl.h"
#include "vision/ssl_vp_config.pb.h"

extern double realTimeOffset;
double getRealTime();


/** Abstract camera interface for the implementation of arbitrary camera interfaces. */
class CameraDriver {
public:
	virtual ~CameraDriver() = default;

	virtual std::shared_ptr<RawImage> readImage() = 0;

	virtual const PixelFormat format() = 0;

	virtual double expectedFrametime() = 0;

	// Bound to the driver for reproducibility during testing with files.
	virtual double getTime();

	virtual uint32_t getWidth() = 0;
	virtual uint32_t getHeight() = 0;
	virtual void setResolution(uint32_t width, uint32_t height) = 0;

	virtual float getExposure() = 0;
	virtual void setExposure(float exposure) = 0;

	virtual float getGain() = 0;
	virtual void setGain(float gain) = 0;

	virtual float getGamma() = 0;
	virtual void setGamma(float gamma) = 0;

	virtual SSL_VPConfigCameraWBType getWhiteBalanceType() = 0;
	virtual float getWhiteBalanceBlue() = 0;
	virtual float getWhiteBalanceRed() = 0;
	virtual void setWhiteBalance(SSL_VPConfigCameraWBType type, float blue, float red) = 0;
};


extern std::unique_ptr<CameraDriver> camera;

void init_camera();
void update_camera_config(const SSL_VPConfigCamera* camera_config);
