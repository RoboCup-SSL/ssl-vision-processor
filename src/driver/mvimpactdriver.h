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
#ifdef MVIMPACT_AVAILABLE

#include "cameradriver.h"
#include <mvIMPACT_CPP/mvIMPACT_acquire.h>
#include <apps/Common/exampleHelper.h> //Necessary header for compilation of <mvIMPACT_acquire_helper.h> without -fpermissive
#include <mvIMPACT_CPP/mvIMPACT_acquire_helper.h>

class MVImpactDriver : public CameraDriver {
public:
	explicit MVImpactDriver(const CameraConfig& config);
	~MVImpactDriver() override;

	std::shared_ptr<RawImage> readImage() override;

	const PixelFormat format() override;

	double expectedFrametime() override;

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
	mvIMPACT::acquire::Device* device;
	SettingsBlueFOX settings;
	std::unique_ptr<mvIMPACT::acquire::helper::RequestProvider> provider;
};

#endif
