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
#ifdef SPINNAKER_AVAILABLE

#include <set>
#include <string>

#include "cameradriver.h"
#include "spinnakerfeatures.h"
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
	void startAcquisition();
	void stopAcquisition();
	void allocateBuffers();
	bool buffersIdle() const;
	void configurePacketSize();
	void selectPixelFormat();
	void refreshFrametime();

	// Feature access by GenICam node name. Camera generations name some features differently
	// (e.g. GammaEnable vs GammaEnabled), so the first available name is used. Features the camera
	// lacks or does not allow in its current mode are reported once and then skipped.
	Spinnaker::GenApi::INode* findNode(const SpinnakerFeature& feature);
	bool setBool(const SpinnakerFeature& feature, bool value);
	bool setFloat(const SpinnakerFeature& feature, double value);
	bool setEnum(const SpinnakerFeature& feature, const char* entry);
	std::string getEnum(const SpinnakerFeature& feature);
	void reportOnce(const std::string& message);

	std::set<std::string> reported;

	const PixelFormat* pixelFormat = &PixelFormat::RGGB8;
	// Last requested auto profile, reported back on cameras without BalanceWhiteAutoProfile
	WhiteBalanceType whiteBalanceType = WhiteBalanceType_AutoOutdoor;

	Spinnaker::SystemPtr pSystem;
	Spinnaker::CameraPtr pCam;

	// Acquisition starts on the first readImage(), so the settings applied after construction
	// (resolution in particular) can still be written; the camera locks them while streaming.
	bool acquiring = false;

	// Cached so the frame loop doesn't do a register read (a network round trip on GigE) every frame
	double frametime = 0.0;
	double frametimeUpdated = 0.0;

	std::map<std::shared_ptr<RawImage>, std::unique_ptr<CLMap<uint8_t>>> buffers; // Use own image buffers for page size alignment (OpenCL pinned memory and zero copy)
	// Pointer array passed to SetUserBuffers. Spinnaker reads it during BeginAcquisition, so it must outlive the call.
	std::vector<void*> bufferPtrs;
};

#endif