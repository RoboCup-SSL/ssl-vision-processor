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
#define SPINNAKER
#ifdef SPINNAKER

#include "cameradriver.h"
#include "Spinnaker.h"

class SpinnakerDriver : public CameraDriver {
public:
	explicit SpinnakerDriver(const CameraConfig& config);
	~SpinnakerDriver() override;

	std::shared_ptr<RawImage> readImage() override;

	const PixelFormat format() override;

	double expectedFrametime() override;

	std::shared_ptr<RawImage> borrow(const Spinnaker::ImagePtr& pImage);
	void restore(const RawImage& image);

	uint32_t getWidth() override;
	uint32_t getHeight() override;
	void setResolution(uint32_t width, uint32_t height) override;

	float getExposure() override;
	void setExposure(float exposure) override;

	float getGain() override;
	void setGain(float gain) override;

	float getGamma() override;
	void setGamma(float gamma) override;

	WhiteBalanceType getWhiteBalanceType() override;
	float getWhiteBalanceBlue() override;
	float getWhiteBalanceRed() override;
	void setWhiteBalance(WhiteBalanceType type, float blue, float red) override;

private:
	Spinnaker::SystemPtr pSystem;
	Spinnaker::CameraPtr pCam;

	std::map<std::shared_ptr<RawImage>, std::unique_ptr<CLMap<uint8_t>>> buffers; // Use own image buffers for page size alignment (OpenCL pinned memory and zero copy)
};

#endif